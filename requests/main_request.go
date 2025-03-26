package requests

import (
	"fmt"
	"project/config"
	"project/datatypes"
	"project/elevator_control"
	"project/elevio"
	"project/network/bcast"
	"project/network/peers"
	request_handler "project/requests/request_handler"
	"time"
)

func RequestControlLoop(
	localID string,
	reqChan chan<- [config.N_FLOORS][config.N_BUTTONS]bool,
	completedReqChan <-chan datatypes.ButtonEvent,
) {
	fmt.Println("=== RequestControlLoop startet, ny versjon ===")

	// Listen for button events
	buttenEventChan := make(chan elevio.ButtonEvent)
	go elevio.PollButtons(buttenEventChan)

	// Network
	sendMessageChan := make(chan datatypes.NetworkMsg)
	receiveMessageChan := make(chan datatypes.NetworkMsg)
	peerUpdateChan := make(chan peers.PeerUpdate)

	go peers.Receiver(config.PEER_PORT, peerUpdateChan)
	go peers.Transmitter(config.PEER_PORT, localID, nil)
	go bcast.Receiver(config.MSG_PORT, receiveMessageChan)
	go bcast.Transmitter(config.MSG_PORT, sendMessageChan)

	// Timers
	broadcastTicker := time.NewTicker(config.STATUS_UPDATE_INTERVAL_MS * time.Millisecond)
	assignRequestTicker := time.NewTicker(config.REQUEST_ASSIGNMENT_INTERVAL_MS * time.Millisecond)

	peerList := []string{}
	isNetworkConnected := false

	hallRequests := [config.N_FLOORS][config.N_HALL_BUTTONS]datatypes.RequestType{}
	allCabRequests := make(map[string][config.N_FLOORS]datatypes.RequestType)
	updatedInfoElevs := make(map[string]datatypes.ElevatorInfo)

	// Local elevator info
	allCabRequests[localID] = [config.N_FLOORS]datatypes.RequestType{}
	updatedInfoElevs[localID] = elevator_control.GetInfoElev()

	for {
		select {

		// --- Button Press Handling --- //
		case btn := <-buttenEventChan:
			fmt.Printf("DEBUG: Mottatt knappetrykk: Floor=%d, Button=%d\n", btn.Floor, btn.Button)
			var request datatypes.RequestType

			// Distinguish between cab vs hall calls
			if btn.Button == elevio.ButtonType(config.BT_CAB) {
				request = allCabRequests[localID][btn.Floor]

				switch request.State {
				case config.Completed:
					// Pressed again after completed => new request
					request.State = config.Assigned
					request.AwareList = AddIfMissing(request.AwareList, localID)
					elevio.SetButtonLamp(btn.Button, btn.Floor, true)

				case config.Unassigned:
					// Normal new cab call
					request.State = config.Assigned
					request.AwareList = []string{localID}
					elevio.SetButtonLamp(btn.Button, btn.Floor, true)

				case config.Assigned:
					// Already assigned => do nothing or custom logic
				}

				// Save back to local array
				localCabReqs := allCabRequests[localID]
				localCabReqs[btn.Floor] = request
				allCabRequests[localID] = localCabReqs
			} else {
				if !isNetworkConnected {
					fmt.Println("Network not connected; ignoring hall request")
					break
				}
				request = hallRequests[btn.Floor][btn.Button]
			}

			fmt.Printf("DEBUG: Før endring: Floor=%d, Button=%d, State=%v\n",
				btn.Floor, btn.Button, request.State)

			switch request.State {
			case config.Completed:
				// Pressing again after completed => new request
				request.State = config.Unassigned
				request.AwareList = []string{localID}

			case config.Unassigned:
				// ← This is where the fix goes
				if len(request.AwareList) == 0 {
					request.AwareList = []string{localID}
				} else {
					request.AwareList = AddIfMissing(request.AwareList, localID)
				}
			}

			if config.ButtonType(btn.Button) == config.BT_CAB {
				if btn.Floor >= 0 && btn.Floor < config.N_FLOORS {
					// Update cab request
					localCabReqs := allCabRequests[localID]
					localCabReqs[btn.Floor] = request
					allCabRequests[localID] = localCabReqs
				} else {
					fmt.Printf("ERROR: Invalid CAB button event: Floor=%d\n", btn.Floor)
				}
			} else {
				if btn.Floor >= 0 && btn.Floor < config.N_FLOORS && btn.Button >= 0 && btn.Button < config.N_HALL_BUTTONS {
					hallRequests[btn.Floor][btn.Button] = request
				} else {
					fmt.Printf("ERROR: Invalid HALL button event: Floor=%d, Button=%d\n", btn.Floor, btn.Button)
				}
			}

			// --- Calls Completed --- //
		case btn := <-completedReqChan:
			var request datatypes.RequestType
			if btn.Button == config.BT_CAB {
				request = allCabRequests[localID][btn.Floor]
			} else {
				request = hallRequests[btn.Floor][btn.Button]
			}
			if request.State == config.Assigned {
				request.State = config.Completed
				request.AwareList = []string{localID}
				request.Count++
				elevio.SetButtonLamp(elevio.ButtonType(btn.Button), btn.Floor, false)
			}
			// Store updated request back:
			if btn.Button == config.BT_CAB {
				localCabReqs := allCabRequests[localID]
				localCabReqs[btn.Floor] = request
				allCabRequests[localID] = localCabReqs
			} else {
				hallRequests[btn.Floor][btn.Button] = request
			}

		// --- Periodic Broadcast --- //
		case <-broadcastTicker.C:
			info := elevator_control.GetInfoElev()
			updatedInfoElevs[localID] = info

			newMsg := datatypes.NetworkMsg{
				SenderID:           localID,
				Available:          info.Available,
				Behavior:           info.Behaviour,
				Floor:              info.CurrentFloor,
				Direction:          elevator_control.DirConv(info.Direction),
				SenderHallRequests: hallRequests,
				AllCabRequests:     allCabRequests,
			}

			fmt.Println("Sending state update | ID:", localID,
				"| Floor:", newMsg.Floor,
				"| Direction:", newMsg.Direction,
				"| State:", newMsg.Behavior)

			if isNetworkConnected {
				sendMessageChan <- newMsg
			}
		case <-assignRequestTicker.C:
			// 1) Demote requests with multiple awareList entries
			for f := 0; f < config.N_FLOORS; f++ {
				for b := 0; b < config.N_HALL_BUTTONS; b++ {
					req := hallRequests[f][b]
					if req.State == config.Assigned && !IsSoleAssignee(req, localID, peerList) {
						fmt.Printf("DEMOTED: Floor %d Button %d | AwareList=%v\n", f, b, req.AwareList)
						req.State = config.Unassigned
						hallRequests[f][b] = req
					}
				}
			}

			// 2) Call request assigner
			allAssignedOrders := request_handler.RequestAssigner(
				hallRequests, allCabRequests, updatedInfoElevs, peerList, localID)
			assignedHallOrders := allAssignedOrders[localID]

			var unifiedOrders [config.N_FLOORS][config.N_BUTTONS]bool

			// 3) Apply only orders that this elevator is allowed to take
			for f := 0; f < config.N_FLOORS; f++ {
				for b := 0; b < config.N_HALL_BUTTONS; b++ {
					if assignedHallOrders[f][b] {
						// Only set if NOT already assigned to another elevator
						if len(hallRequests[f][b].AwareList) <= 1 || hallRequests[f][b].AwareList[0] == localID {
							hallRequests[f][b].State = config.Assigned
							hallRequests[f][b].AwareList = []string{localID}
							unifiedOrders[f][b] = true
							elevio.SetButtonLamp(elevio.ButtonType(b), f, true)
							fmt.Printf("[ASSIGNED] Floor %d Button %d to %s\n", f, b, localID)
							sendMessageChan <- datatypes.NetworkMsg{
								SenderID: localID,
								DebugLog: fmt.Sprintf("[ORDER ASSIGNED] Floor %d Button %d -> %s", f, b, localID),
							}
						}
					}

				}
			}

			// 4) Merge local cab calls
			localCabReqs := allCabRequests[localID]
			for f := 0; f < config.N_FLOORS; f++ {
				if localCabReqs[f].State == config.Assigned {
					unifiedOrders[f][config.BT_CAB] = true
					elevio.SetButtonLamp(elevio.ButtonType(config.BT_CAB), f, true)
				}
			}

			// 5) Send orders to FSM
			fmt.Println("Sending updated orders to FSM:", unifiedOrders)
			select {
			case reqChan <- unifiedOrders:
			default:
			}

			// --- Peer Updates --- //
		case peer := <-peerUpdateChan:
			peerList = peer.Peers

			if peer.New == localID {
				isNetworkConnected = true
			}
			if IsContainedIn([]string{localID}, peer.Lost) {
				isNetworkConnected = false
			}

		// --- Receiving Network Messages --- //
		case msg := <-receiveMessageChan:
			if msg.SenderID == localID {
				break
			}
			if !isNetworkConnected {
				break
			}
			if msg.SenderID != localID && msg.DebugLog != "" {
				fmt.Println("DEBUGLOG from", msg.SenderID+":", msg.DebugLog)
			}

			updatedInfoElevs[msg.SenderID] = datatypes.ElevatorInfo{
				Behaviour:    msg.Behavior,
				Direction:    config.Direction(msg.Direction),
				Available:    msg.Available,
				CurrentFloor: msg.Floor,
			}

			// Merge Hall Requests
			for f := 0; f < config.N_FLOORS; f++ {
				for b := 0; b < config.N_HALL_BUTTONS; b++ {
					if !canAcceptRequest(hallRequests[f][b], msg.SenderHallRequests[f][b]) {
						continue
					}
					accepted := msg.SenderHallRequests[f][b]
					accepted.AwareList = AddIfMissing(accepted.AwareList, localID)

					if accepted.State == config.Unassigned && IsContainedIn(accepted.AwareList, peerList) {
						accepted.State = config.Assigned
						accepted.AwareList = []string{localID}
						elevio.SetButtonLamp(elevio.ButtonType(b), f, true)
					}

					switch accepted.State {
					case config.Assigned:
						elevio.SetButtonLamp(elevio.ButtonType(b), f, true)
					case config.Completed:
						elevio.SetButtonLamp(elevio.ButtonType(b), f, false)
					}

					hallRequests[f][b] = accepted

				}
			}
		}
	}
}

func isAssignedToLocal(req datatypes.RequestType, localID string) bool {
	return req.State != config.Assigned || contains(req.AwareList, localID)
}

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}
