package requests

import (
	"encoding/json"
	"fmt"
	"os"
	"project/config"
	"project/datatypes"
	"project/elevator_control"
	"project/elevio"
	"project/network/bcast"
	"project/network/peers"
	request_handler "project/requests/request_handler"
	"time"
)

const (
	PEER_PORT                      = 30060
	MSG_PORT                       = 30061
	STATUS_UPDATE_INTERVAL_MS      = 200
	REQUEST_ASSIGNMENT_INTERVAL_MS = 1000
)

func RequestControlLoop(
	localID string,
	reqChan chan<- [config.N_FLOORS][config.N_BUTTONS]bool,
	completedReqChan chan elevio.ButtonEvent,
) {
	fmt.Println("=== RequestControlLoop startet, ny versjon ===")

	// Listen for button events
	buttenEventChan := make(chan elevio.ButtonEvent)
	go elevio.PollButtons(buttenEventChan)

	// Network
	sendMessageChan := make(chan datatypes.NetworkMsg)
	receiveMessageChan := make(chan datatypes.NetworkMsg)
	peerUpdateChan := make(chan peers.PeerUpdate)

	go peers.Receiver(PEER_PORT, peerUpdateChan)
	go peers.Transmitter(PEER_PORT, localID, nil)
	go bcast.Receiver(MSG_PORT, receiveMessageChan)
	go bcast.Transmitter(MSG_PORT, sendMessageChan)

	// Timers
	broadcastTicker := time.NewTicker(STATUS_UPDATE_INTERVAL_MS * time.Millisecond)
	assignRequestTicker := time.NewTicker(REQUEST_ASSIGNMENT_INTERVAL_MS * time.Millisecond)

	peerList := []string{}
	isNetworkConnected := false

	hallRequests := [config.N_FLOORS][config.N_HALL_BUTTONS]datatypes.RequestType{}
	updatedInfoElevs := make(map[string]datatypes.ElevatorInfo)
	allCabRequests := make(map[string][config.N_FLOORS]datatypes.RequestType)

	// Local elevator info
	updatedInfoElevs[localID] = elevator_control.GetInfoElev()
	if loaded, err := LoadCabCalls(localID); err == nil {
		allCabRequests[localID] = loaded
		fmt.Println("Restored cab calls for", localID)
	} else {
		allCabRequests[localID] = [config.N_FLOORS]datatypes.RequestType{}
	}

	for {
		select {

		// --- Button Press Handling --- //
		case btn := <-buttenEventChan:
			fmt.Printf("DEBUG: Mottatt knappetrykk: Floor=%d, Button=%d\n", btn.Floor, btn.Button)
			var request datatypes.RequestType

			// Distinguish between cab vs hall calls
			if btn.Button == elevio.ButtonType(elevio.BT_Cab) {
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
				SaveCabCalls(localID, allCabRequests)

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

			if elevio.ButtonType(btn.Button) == elevio.BT_Cab {
				if btn.Floor >= 0 && btn.Floor < config.N_FLOORS {
					// Update cab request
					localCabReqs := allCabRequests[localID]
					localCabReqs[btn.Floor] = request
					allCabRequests[localID] = localCabReqs
					SaveCabCalls(localID, allCabRequests)
				} else {
					fmt.Printf("ERROR: Invalid CAB button event: Floor=%d\n", btn.Floor)
				}
			} else {
				if btn.Floor >= 0 && btn.Floor < config.N_FLOORS && btn.Button >= 0 && btn.Button < config.N_HALL_BUTTONS {
					hallRequests[btn.Floor][btn.Button] = request

					// --- IMMEDIATE SERVE IF AT FLOOR AND IDLE/DOOROPEN --- //
					info := updatedInfoElevs[localID]
					if btn.Floor == info.CurrentFloor &&
						(info.Behaviour == config.Idle || info.Behaviour == config.DoorOpen) {

						request.State = config.Assigned
						request.AwareList = []string{localID}
						hallRequests[btn.Floor][btn.Button] = request
					}

				} else {
					fmt.Printf("ERROR: Invalid HALL button event: Floor=%d, Button=%d\n", btn.Floor, btn.Button)
				}
			}

			// --- Calls Completed --- //
		case btn := <-completedReqChan:
			var request datatypes.RequestType
			if btn.Button == elevio.BT_Cab {
				request = allCabRequests[localID][btn.Floor]
			} else {
				request = hallRequests[btn.Floor][btn.Button]
			}

			if request.State == config.Assigned {
				request.State = config.Completed
				request.AwareList = []string{}
				request.Count++
				elevio.SetButtonLamp(elevio.ButtonType(btn.Button), btn.Floor, false)
			}

			if btn.Button == elevio.BT_Cab {
				localCabReqs := allCabRequests[localID]
				localCabReqs[btn.Floor] = request
				allCabRequests[localID] = localCabReqs
				SaveCabCalls(localID, allCabRequests)
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
			for f := 0; f < config.N_FLOORS; f++ {
				for b := 0; b < config.N_HALL_BUTTONS; b++ {
					req := hallRequests[f][b]

					// Remove unavailable elevators from AwareList
					filtered := []string{}
					for _, id := range req.AwareList {
						if contains(peerList, id) {
							filtered = append(filtered, id)
						}
					}
					req.AwareList = filtered

					// Demote if assigned but not solely to this elevator
					if req.State == config.Assigned {
						active := 0
						for _, id := range req.AwareList {
							if contains(peerList, id) {
								active++
							}
						}
						if active > 1 {
							fmt.Printf("[DEMOTED] Too many active assignees: Floor %d Button %d | AwareList=%v\n", f, b, req.AwareList)
							req.State = config.Unassigned
						}
					}

					hallRequests[f][b] = req
				}
			}

			if isNetworkConnected {
				sendMessageChan <- datatypes.NetworkMsg{
					SenderID:           localID,
					SenderHallRequests: hallRequests,
				}
			}

			// Build jsonStates for the HRA
			type HRAElevState struct {
				Floor       int                   `json:"floor"`
				Direction   int                   `json:"direction"`
				Behaviour   string                `json:"behaviour"`
				CabRequests [config.N_FLOORS]bool `json:"cabRequests"`
			}

			jsonStates := make(map[string]HRAElevState)

			for id, info := range updatedInfoElevs {
				var cabReqs [config.N_FLOORS]bool
				for f := 0; f < config.N_FLOORS; f++ {
					if allCabRequests[id][f].State == config.Assigned {
						cabReqs[f] = true
					}
				}

				jsonStates[id] = HRAElevState{
					Floor:       info.CurrentFloor,
					Direction:   int(info.Direction),
					Behaviour:   fmt.Sprint(info.Behaviour),
					CabRequests: cabReqs,
				}
			}

			// 2) Call request assigner
			allAssignedOrders := request_handler.RequestAssigner(
				hallRequests, allCabRequests, updatedInfoElevs, peerList, localID)
			var assignedHallOrders [config.N_FLOORS][config.N_HALL_BUTTONS]bool
			if len(peerList) == 1 && peerList[0] == localID {
				// Alone on the network – take all unassigned/assigned-to-me requests
				for f := 0; f < config.N_FLOORS; f++ {
					for b := 0; b < config.N_HALL_BUTTONS; b++ {
						req := hallRequests[f][b]
						if req.State != config.Completed && len(req.AwareList) > 0 {
							assignedHallOrders[f][b] = true
						}
					}
				}
			} else {
				// Normal distributed assignment
				fullAssignment := allAssignedOrders[localID]
				for f := 0; f < config.N_FLOORS; f++ {
					for b := 0; b < config.N_HALL_BUTTONS; b++ {
						assignedHallOrders[f][b] = fullAssignment[f][b]
					}
				}
			}

			var unifiedOrders [config.N_FLOORS][config.N_BUTTONS]bool

			// 3) Apply only orders that this elevator is allowed to take
			for f := 0; f < config.N_FLOORS; f++ {
				for b := 0; b < config.N_HALL_BUTTONS; b++ {
					if assignedHallOrders[f][b] {
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
					unifiedOrders[f][elevio.BT_Cab] = true
					elevio.SetButtonLamp(elevio.ButtonType(elevio.BT_Cab), f, true)
				}
			}

			fmt.Println("RA: assignedHallOrders:", assignedHallOrders)
			fmt.Println("RA: Sending unifiedOrders to FSM:", unifiedOrders)

			// 5) Send orders to FSM
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

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

func SaveCabCalls(localID string, allCabRequests map[string][config.N_FLOORS]datatypes.RequestType) error {
	data, err := json.Marshal(allCabRequests[localID])
	if err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf("cab_calls_%s.json", localID), data, 0644)
}

func LoadCabCalls(localID string) ([config.N_FLOORS]datatypes.RequestType, error) {
	var calls [config.N_FLOORS]datatypes.RequestType
	data, err := os.ReadFile(fmt.Sprintf("cab_calls_%s.json", localID))
	if err != nil {
		return calls, err
	}
	err = json.Unmarshal(data, &calls)
	return calls, err
}
