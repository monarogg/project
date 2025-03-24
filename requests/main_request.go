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

const (
	PEER_PORT                      = 30060
	MSG_PORT                       = 30061
	STATUS_UPDATE_INTERVAL_MS      = 200
	REQUEST_ASSIGNMENT_INTERVAL_MS = 1000
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

	go peers.Receiver(PEER_PORT, peerUpdateChan)
	go peers.Transmitter(PEER_PORT, localID, nil)
	go bcast.Receiver(MSG_PORT, receiveMessageChan)
	go bcast.Transmitter(MSG_PORT, sendMessageChan)

	// Timers
	broadcastTicker := time.NewTicker(STATUS_UPDATE_INTERVAL_MS * time.Millisecond)
	assignRequestTicker := time.NewTicker(REQUEST_ASSIGNMENT_INTERVAL_MS * time.Millisecond)

	peerList := []string{}
	isNetworkConnected := false

	hallRequests := [datatypes.N_FLOORS][datatypes.N_HALL_BUTTONS]datatypes.RequestType{}
	allCabRequests := make(map[string][datatypes.N_FLOORS]datatypes.RequestType)
	updatedInfoElevs := make(map[string]datatypes.ElevatorInfo)

	// Local elevator info
	allCabRequests[localID] = [datatypes.N_FLOORS]datatypes.RequestType{}
	updatedInfoElevs[localID] = elevator_control.GetInfoElev()

	for {
		select {

		// --- Button Press Handling --- //
		case btn := <-buttenEventChan:
			fmt.Printf("DEBUG: Mottatt knappetrykk: Floor=%d, Button=%d\n", btn.Floor, btn.Button)
			var request datatypes.RequestType

			// Distinguish between cab vs hall calls
			if btn.Button == elevio.ButtonType(datatypes.BT_CAB) {
				request = allCabRequests[localID][btn.Floor]

				switch request.State {
				case datatypes.Completed:
					// Pressed again after completed => new request
					request.State = datatypes.Assigned
					request.AwareList = []string{localID}
					elevio.SetButtonLamp(btn.Button, btn.Floor, true)

				case datatypes.Unassigned:
					// Normal new cab call
					request.State = datatypes.Assigned
					request.AwareList = []string{localID}
					elevio.SetButtonLamp(btn.Button, btn.Floor, true)

				case datatypes.Assigned:
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
			case datatypes.Completed:
				// Pressing again after completed => new request
				request.State = datatypes.Unassigned
				request.AwareList = []string{localID}

			case datatypes.Unassigned:
				// Just ensure localID is aware. Actual assignment is done later
				request.AwareList = addIfMissing(request.AwareList, localID)

			case datatypes.Assigned:
				// Already assigned => do nothing or custom logic
			}

			fmt.Printf("DEBUG: Etter endring: Floor=%d, Button=%d, State=%v\n",
				btn.Floor, btn.Button, request.State)

			// Store updated request
			if btn.Button == elevio.ButtonType(datatypes.BT_CAB) {
				localCabReqs := allCabRequests[localID]
				localCabReqs[btn.Floor] = request
				allCabRequests[localID] = localCabReqs
			} else {
				hallRequests[btn.Floor][btn.Button] = request
			}

			// --- Calls Completed --- //
		case btn := <-completedReqChan:
			var request datatypes.RequestType
			if btn.Button == datatypes.BT_CAB {
				request = allCabRequests[localID][btn.Floor]
			} else {
				request = hallRequests[btn.Floor][btn.Button]
			}
			if request.State == datatypes.Assigned {
				request.State = datatypes.Completed
				request.AwareList = []string{localID}
				request.Count++
				elevio.SetButtonLamp(elevio.ButtonType(btn.Button), btn.Floor, false)
			}
			// Store updated request back:
			if btn.Button == datatypes.BT_CAB {
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

			// --- Periodic Assignment --- //
		case <-assignRequestTicker.C:
			// 1) Get hall orders from external assigner
			assignedHallOrders := request_handler.RequestAssigner(
				hallRequests, allCabRequests, updatedInfoElevs, peerList, localID)

			// 2) Build unified orders array for 0 & 1 (hall) and 2 (cab)
			var unifiedOrders [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool

			// 3) Merge hall calls
			for f := 0; f < datatypes.N_FLOORS; f++ {
				for b := 0; b < datatypes.N_HALL_BUTTONS; b++ {
					// If external says true OR local is assigned
					if assignedHallOrders[f][b] || (hallRequests[f][b].State == datatypes.Assigned) {
						unifiedOrders[f][b] = true
						// Optionally set local state to assigned
						hallRequests[f][b].State = datatypes.Assigned
						hallRequests[f][b].AwareList = []string{localID}
						// Turn on hall lamp
						elevio.SetButtonLamp(elevio.ButtonType(b), f, true)
					}
				}
			}

			// 4) Merge local cab calls (button index 2)
			localCabReqs := allCabRequests[localID]
			for f := 0; f < datatypes.N_FLOORS; f++ {
				if localCabReqs[f].State == datatypes.Assigned {
					unifiedOrders[f][datatypes.BT_CAB] = true
					// Turn on cab lamp
					elevio.SetButtonLamp(elevio.ButtonType(datatypes.BT_CAB), f, true)
				}
			}

			// 5) Send orders to the FSM
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
			if isContainedIn([]string{localID}, peer.Lost) {
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

			updatedInfoElevs[msg.SenderID] = datatypes.ElevatorInfo{
				Behaviour:    msg.Behavior,
				Direction:    datatypes.Direction(msg.Direction),
				Available:    msg.Available,
				CurrentFloor: msg.Floor,
			}

			// Merge Hall Requests
			for f := 0; f < datatypes.N_FLOORS; f++ {
				for b := 0; b < datatypes.N_HALL_BUTTONS; b++ {
					if !canAcceptRequest(hallRequests[f][b], msg.SenderHallRequests[f][b]) {
						continue
					}
					accepted := msg.SenderHallRequests[f][b]
					accepted.AwareList = addIfMissing(accepted.AwareList, localID)
					switch accepted.State {
					case datatypes.Assigned:
						elevio.SetButtonLamp(elevio.ButtonType(b), f, true)
					case datatypes.Unassigned:
						elevio.SetButtonLamp(elevio.ButtonType(b), f, false)
						// Only auto-promote if not already Completed:
						if accepted.State != datatypes.Completed && isContainedIn(peerList, accepted.AwareList) {
							accepted.State = datatypes.Assigned
							accepted.AwareList = []string{localID}
							elevio.SetButtonLamp(elevio.ButtonType(b), f, true)
						}
					case datatypes.Completed:
						elevio.SetButtonLamp(elevio.ButtonType(b), f, false)
					}
					hallRequests[f][b] = accepted
				}
			}

		}
	}
}
