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

func DistributedRequestLoop(
	localID string,
	reqChan chan<- [config.NUM_FLOORS][config.NUM_BUTTONS]bool,
	completedReqChan chan datatypes.ButtonEvent,
) {
	fmt.Println("=== DistributedRequestLoop startet, ny versjon ===")

	// Listen for button events
	buttonEventChan := make(chan elevio.ButtonEvent)
	go elevio.PollButtons(buttonEventChan)

	// Network
	senNetworkMsgChan := make(chan datatypes.NetworkMsg)
	receiveNetworkMsgChan := make(chan datatypes.NetworkMsg)
	peerUpdateChan := make(chan peers.PeerUpdate)

	go peers.Receiver(config.PEER_PORT, peerUpdateChan)
	go peers.Transmitter(config.PEER_PORT, localID, nil)
	go bcast.Receiver(config.MSG_PORT, receiveNetworkMsgChan)
	go bcast.Transmitter(config.MSG_PORT, senNetworkMsgChan)

	// Timers
	broadcastTicker := time.NewTicker(config.STATUS_UPDATE_INTERVAL)
	assignRequestTicker := time.NewTicker(config.REQUEST_ASSIGNMENT_INTERVAL)

	peerList := []string{}
	isNetworkConnected := false
	latestPeerList := []string{}

	var stablePeerCount = 0
	var stableTicks = 0
	var reducedTicks = 0
	var baselineEstablished = false

	hallRequests := [datatypes.NUM_FLOORS][datatypes.NUM_HALL_BUTTONS]datatypes.RequestType{}
	updatedInfoElevs := make(map[string]datatypes.ElevatorInfo)
	allCabRequests := make(map[string][datatypes.NUM_FLOORS]datatypes.RequestType)

	// Local elevator info
	updatedInfoElevs[localID] = elevator_control.GetInfoElev()
	if loaded, err := LoadCabCalls(localID); err == nil {
		allCabRequests[localID] = loaded
		fmt.Println("Restored cab calls for", localID)
	} else {
		allCabRequests[localID] = [datatypes.NUM_FLOORS]datatypes.RequestType{}
	}

	for {
		select {

		// --- Button Press Handling --- //
		case btn := <-buttonEventChan:
			fmt.Printf("DEBUG: Mottatt knappetrykk: Floor=%d, Button=%d\n", btn.Floor, btn.Button)
			var request datatypes.RequestType

			// Distinguish between cab vs hall calls
			if btn.Button == elevio.ButtonType(datatypes.BT_CAB) {
				request = allCabRequests[localID][btn.Floor]

				switch request.State {
				case datatypes.Completed:
					// Pressed again after completed => new request
					request.State = datatypes.Assigned
					request.AwareList = AddIfMissing(request.AwareList, localID)
					elevio.SetButtonLamp(btn.Button, btn.Floor, true)

				case datatypes.Unassigned:
					// Normal new cab call
					request.State = datatypes.Assigned
					request.AwareList = []string{localID}
					elevio.SetButtonLamp(btn.Button, btn.Floor, true)

				case datatypes.Assigned:
					// Already assigned
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
			case datatypes.Completed:
				// Pressing again after completed
				request.State = datatypes.Unassigned
				request.AwareList = []string{localID}

			case datatypes.Unassigned:
				if len(request.AwareList) == 0 {
					request.AwareList = []string{localID}
				} else {
					request.AwareList = AddIfMissing(request.AwareList, localID)
				}
			}

			if datatypes.ButtonType(btn.Button) == datatypes.BT_CAB {
				if btn.Floor >= 0 && btn.Floor < datatypes.NUM_FLOORS {
					// Update cab request
					localCabReqs := allCabRequests[localID]
					localCabReqs[btn.Floor] = request
					allCabRequests[localID] = localCabReqs
					SaveCabCalls(localID, allCabRequests)
				} else {
					fmt.Printf("ERROR: Invalid CAB button event: Floor=%d\n", btn.Floor)
				}
			} else {
				if btn.Floor >= 0 && btn.Floor < datatypes.NUM_FLOORS && btn.Button >= 0 && btn.Button < datatypes.NUM_HALL_BUTTONS {
					hallRequests[btn.Floor][btn.Button] = request

					// --- IMMEDIATE SERVE IF AT FLOOR AND IDLE/DOOROPEN --- //
					info := updatedInfoElevs[localID]
					if btn.Floor == info.CurrentFloor &&
						(info.Behaviour == datatypes.Idle || info.Behaviour == datatypes.DoorOpen) {

						request.State = datatypes.Assigned
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
			if btn.Button == datatypes.BT_CAB {
				request = allCabRequests[localID][btn.Floor]
			} else {
				request = hallRequests[btn.Floor][btn.Button]
			}

			if request.State == datatypes.Assigned {
				request.State = datatypes.Completed
				request.AwareList = []string{}
				request.Count++
				elevio.SetButtonLamp(elevio.ButtonType(btn.Button), btn.Floor, false)
			}

			if btn.Button == datatypes.BT_CAB {
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
				senNetworkMsgChan <- newMsg
			}

		case <-assignRequestTicker.C:
			peerList = latestPeerList
			for f := 0; f < datatypes.NUM_FLOORS; f++ {
				for b := 0; b < datatypes.NUM_HALL_BUTTONS; b++ {
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
					if req.State == datatypes.Assigned {
						active := 0
						for _, id := range req.AwareList {
							if contains(peerList, id) {
								active++
							}
						}
						if active > 1 {
							fmt.Printf("[DEMOTED] Too many active assignees: Floor %d Button %d | AwareList=%v\n", f, b, req.AwareList)
							req.State = datatypes.Unassigned
						}
					}

					hallRequests[f][b] = req
				}
			}

			if isNetworkConnected {
				senNetworkMsgChan <- datatypes.NetworkMsg{
					SenderID:           localID,
					SenderHallRequests: hallRequests,
				}
			}

			// Build jsonStates for the HRA
			type HRAElevState struct {
				Floor       int                     `json:"floor"`
				Direction   int                     `json:"direction"`
				Behaviour   string                  `json:"behaviour"`
				CabRequests [config.NUM_FLOORS]bool `json:"cabRequests"`
			}

			jsonStates := make(map[string]HRAElevState)

			for id, info := range updatedInfoElevs {
				var cabReqs [config.NUM_FLOORS]bool
				for f := 0; f < config.NUM_FLOORS; f++ {
					if allCabRequests[id][f].State == datatypes.Assigned {
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
			allAssignedOrders := request_handler.HRAmain(
				hallRequests, allCabRequests, updatedInfoElevs, peerList, localID)
			var assignedHallOrders [datatypes.NUM_FLOORS][datatypes.NUM_HALL_BUTTONS]bool
			currentCount := len(peerList)

			fmt.Printf("[TICK] peerCount=%d stableBaseline=%d stableTicks=%d reducedTicks=%d\n", currentCount, stablePeerCount, stableTicks, reducedTicks)

			if !baselineEstablished {
				if currentCount == stablePeerCount {
					stableTicks++
				} else {
					stablePeerCount = currentCount
					stableTicks = 1
				}
				if stableTicks >= 30 {
					baselineEstablished = true
					fmt.Printf("[BASELINE SET] Peer baseline established at %d\n", stablePeerCount)
				}
			} else {
				if currentCount < stablePeerCount {
					reducedTicks++
				} else {
					reducedTicks = 0
				}
			}

			if reducedTicks >= 15 {
				fmt.Println("Peer count dropped from baseline for 10 ticks – taking over hall orders")
				for f := 0; f < datatypes.NUM_FLOORS; f++ {
					for b := 0; b < datatypes.NUM_HALL_BUTTONS; b++ {
						req := hallRequests[f][b]
						if req.State != datatypes.Completed && len(req.AwareList) > 0 {
							assignedHallOrders[f][b] = true
						}
					}
				}
			} else {
				// Normal distributed assignment
				fullAssignment := allAssignedOrders[localID]
				for f := 0; f < datatypes.NUM_FLOORS; f++ {
					for b := 0; b < datatypes.NUM_HALL_BUTTONS; b++ {
						assignedHallOrders[f][b] = fullAssignment[f][b]
					}
				}
			}

			var unifiedOrders [datatypes.NUM_FLOORS][datatypes.NUM_BUTTONS]bool

			// 3) Apply only orders that this elevator is allowed to take
			for f := 0; f < datatypes.NUM_FLOORS; f++ {
				for b := 0; b < datatypes.NUM_HALL_BUTTONS; b++ {
					if assignedHallOrders[f][b] {
						if len(hallRequests[f][b].AwareList) <= 1 || hallRequests[f][b].AwareList[0] == localID {
							hallRequests[f][b].State = datatypes.Assigned
							hallRequests[f][b].AwareList = []string{localID}
							unifiedOrders[f][b] = true
							elevio.SetButtonLamp(elevio.ButtonType(b), f, true)
							fmt.Printf("[ASSIGNED] Floor %d Button %d to %s\n", f, b, localID)
							senNetworkMsgChan <- datatypes.NetworkMsg{
								SenderID: localID,
								DebugLog: fmt.Sprintf("[ORDER ASSIGNED] Floor %d Button %d -> %s", f, b, localID),
							}
						}
					}
				}
			}

			// 4) Merge local cab calls
			localCabReqs := allCabRequests[localID]
			for f := 0; f < datatypes.NUM_FLOORS; f++ {
				if localCabReqs[f].State == datatypes.Assigned {
					unifiedOrders[f][datatypes.BT_CAB] = true
					elevio.SetButtonLamp(elevio.ButtonType(datatypes.BT_CAB), f, true)
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
			latestPeerList = peer.Peers

			if peer.New == localID {
				isNetworkConnected = true
			}
			if IsContainedIn([]string{localID}, peer.Lost) {
				isNetworkConnected = false
			}

		// --- Receiving Network Messages --- //
		case msg := <-receiveNetworkMsgChan:
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
				Direction:    datatypes.Direction(msg.Direction),
				Available:    msg.Available,
				CurrentFloor: msg.Floor,
			}

			// Merge Hall Requests
			for f := 0; f < datatypes.NUM_FLOORS; f++ {
				for b := 0; b < datatypes.NUM_HALL_BUTTONS; b++ {
					if !canAcceptRequest(hallRequests[f][b], msg.SenderHallRequests[f][b]) {
						continue
					}
					accepted := msg.SenderHallRequests[f][b]
					accepted.AwareList = AddIfMissing(accepted.AwareList, localID)

					switch accepted.State {
					case datatypes.Assigned:
						elevio.SetButtonLamp(elevio.ButtonType(b), f, true)
					case datatypes.Completed:
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

func SaveCabCalls(localID string, allCabRequests map[string][datatypes.NUM_FLOORS]datatypes.RequestType) error {
	data, err := json.Marshal(allCabRequests[localID])
	if err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf("cab_calls_%s.json", localID), data, 0644)
}

func LoadCabCalls(localID string) ([datatypes.NUM_FLOORS]datatypes.RequestType, error) {
	var calls [datatypes.NUM_FLOORS]datatypes.RequestType
	data, err := os.ReadFile(fmt.Sprintf("cab_calls_%s.json", localID))
	if err != nil {
		return calls, err
	}
	err = json.Unmarshal(data, &calls)
	return calls, err
}
