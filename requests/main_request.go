package requests

import (
	"fmt"
	"project/datatypes"
	"project/elevator_control"
	"project/elevio"
	"project/network/bcast"
	"project/network/peers"
	requesthandler "project/requests/request_handler"
	"time"
)

// Ports for peer detection and broadcasting requests
const (
	PEER_PORT               = 17555
	MSG_PORT                = 17556
	ASSIGN_REQUESTS_TIME_MS = 1000
	SEND_TIME_MS            = 200
)

// RunRequestControl coordinates distributed requests among elevators.

func RunRequestControl(
	localID string,
	requestsCh chan<- [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool,
	completedRequestCh <-chan datatypes.ButtonEvent,
) {
	// Channels for local button presses, outgoing/incoming network messages, and peer updates
	buttonEventCh := make(chan elevio.ButtonEvent)
	go elevio.PollButtons(buttonEventCh)

	messageTx := make(chan datatypes.NetworkMsg)
	messageRx := make(chan datatypes.NetworkMsg)
	peerUpdateCh := make(chan peers.PeerUpdate)

	// Start peer detection
	go peers.Transmitter(PEER_PORT, localID, nil)
	go peers.Receiver(PEER_PORT, peerUpdateCh)

	// Start broadcast for request messages
	go bcast.Transmitter(MSG_PORT, messageTx)
	go bcast.Receiver(MSG_PORT, messageRx)

	// Timers to send updates and reassign requests
	assignRequestTicker := time.NewTicker(ASSIGN_REQUESTS_TIME_MS * time.Millisecond)
	sendTicker := time.NewTicker(SEND_TIME_MS * time.Millisecond)

	peerList := []string{}
	connectedToNetwork := false

	var hallRequests [datatypes.N_FLOORS][datatypes.N_HALL_BUTTONS]datatypes.RequestType
	allCabRequests := make(map[string][datatypes.N_FLOORS]datatypes.RequestType)
	allCabRequests[localID] = [datatypes.N_FLOORS]datatypes.RequestType{}

	// Store elevator info for all elevators
	latestInfoElevators := make(map[string]datatypes.ElevatorInfo)

	// Initialize your elevator info
	myElevInfo := elevator_control.GetInfoElev()
	latestInfoElevators[localID] = myElevInfo

	for {
		select {

		case btn := <-buttonEventCh:
			var req datatypes.RequestType
			if btn.Button == elevio.ButtonType(datatypes.BT_CAB) {
				req = allCabRequests[localID][btn.Floor]
			} else {
				if !connectedToNetwork {
					continue
				}
				req = hallRequests[btn.Floor][btn.Button]
			}

			switch req.State {
			case datatypes.Completed:
				req.State = datatypes.Unassignes
				req.Count++
				req.AwareList = []string{localID}

			case datatypes.Unassignes:
				req.Count++

				if isSubset(peerList, req.AwareList) {
					req.State = datatypes.Assigned
					req.AwareList = []string{localID}
				}
			}

			if btn.Button == elevio.ButtonType(datatypes.BT_CAB) {
				tempCab := allCabRequests[localID]
				tempCab[btn.Floor] = req
				allCabRequests[localID] = tempCab
			} else {
				hallRequests[btn.Floor][btn.Button] = req
			}

		case btn := <-completedRequestCh:
			var req datatypes.RequestType
			if btn.Button == datatypes.BT_CAB {
				req = allCabRequests[localID][btn.Floor]
			} else {
				req = hallRequests[btn.Floor][btn.Button]
			}

			if req.State == datatypes.Assigned {
				req.State = datatypes.Completed
				req.Count++
				req.AwareList = []string{localID}
				elevio.SetButtonLamp(elevio.ButtonType(datatypes.BT_CAB), btn.Floor, false)
			}

			if btn.Button == datatypes.BT_CAB {
				tempCab := allCabRequests[localID]
				tempCab[btn.Floor] = req
				allCabRequests[localID] = tempCab
			} else {
				hallRequests[btn.Floor][btn.Button] = req
			}

		//Periodically broadcast
		case <-sendTicker.C:
			myElevInfo = elevator_control.GetInfoElev()
			latestInfoElevators[localID] = myElevInfo

			newMsg := datatypes.NetworkMsg{
				SenderID:           localID,
				Available:          myElevInfo.Available,
				Behavior:           myElevInfo.Behaviour,
				Floor:              myElevInfo.CurrentFloor,
				Direction:          elevio.MotorDirection(myElevInfo.Direction),
				SenderHallRequests: hallRequests,
				AllCabRequests:     allCabRequests,
			}
			if connectedToNetwork {
				messageTx <- newMsg
			}

		//Periodically assign
		case <-assignRequestTicker.C:
			if connectedToNetwork {
				assignedMatrix := requesthandler.RequestAssigner(
					hallRequests,
					allCabRequests,
					latestInfoElevators,
					peerList,
					localID,
				)
				// Send to FSM or elevator logic
				select {
				case requestsCh <- assignedMatrix:
				default:
					// avoid blocking if no listener
				}
			}

		case p := <-peerUpdateCh:
			peerList = p.Peers
			if p.New == localID {
				connectedToNetwork = true
				fmt.Println("Joined the network!")
			}
			for _, lostID := range p.Lost {
				if lostID == localID {
					connectedToNetwork = false
					fmt.Println("We left the network!")
				}
			}

		case msg := <-messageRx:
			if msg.SenderID == localID {
				continue
			}
			if !connectedToNetwork {
				continue
			}

			latestInfoElevators[msg.SenderID] = datatypes.ElevatorInfo{
				Available:    msg.Available,
				Behaviour:    msg.Behavior,
				CurrentFloor: msg.Floor,
				Direction:    datatypes.Direction(msg.Direction),
			}

			// Merge AllCabRequests
			for elevID, incomingCab := range msg.AllCabRequests {
				localCab, known := allCabRequests[elevID]
				if !known {
					allCabRequests[elevID] = incomingCab
					continue
				}
				// compare floors
				for f := 0; f < datatypes.N_FLOORS; f++ {
					if canAcceptRequest(localCab[f], incomingCab[f]) {
						updated := incomingCab[f]
						updated.AwareList = addIfMissing(updated.AwareList, localID)
						localCab[f] = updated
					}
				}
				allCabRequests[elevID] = localCab
			}

			// Merge HallRequests
			for f := 0; f < datatypes.N_FLOORS; f++ {
				for b := 0; b < datatypes.N_HALL_BUTTONS; b++ {
					if canAcceptRequest(hallRequests[f][b], msg.SenderHallRequests[f][b]) {
						updated := msg.SenderHallRequests[f][b]
						updated.AwareList = addIfMissing(updated.AwareList, localID)
						hallRequests[f][b] = updated
					}
				}
			}
		}
	}
}
