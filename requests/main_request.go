package requests

// skal håndtere koordinering av knappetrykk, network messages og fordeling av bestillinger mellom heisene

import (
	"project/datatypes"
	"project/elevator_control"
	"project/elevio"
	"project/network/bcast"
	"project/network/peers"
	"time"
	request_handler "project/requests/request_handler"
)

const (
	PEER_PORT                      = 30060
	MSG_PORT                       = 30061
	STATUS_UPDATE_INTERVAL_MS      = 200
	REQUEST_ASSIGNMENT_INTERVAL_MS = 1000
)

func RequestControlLoop(localID string, reqChan chan<- [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool,
	completedReqChan <-chan datatypes.ButtonEvent) {

	// channel for butten event:
	buttenEventChan := make(chan elevio.ButtonEvent)
	go elevio.PollButtons(buttenEventChan)

	// channels for sending/receiving messages
	sendMessageChan := make(chan datatypes.NetworkMsg)
	receiveMessageChan := make(chan datatypes.NetworkMsg)
	// channels for motta oppdatering om peers
	peerUpdateChan := make(chan peers.PeerUpdate)

	// go rutines for network:
	go peers.Receiver(PEER_PORT, peerUpdateChan)
	go peers.Transmitter(PEER_PORT, localID, nil)
	go bcast.Receiver(MSG_PORT, receiveMessageChan)
	go bcast.Transmitter(MSG_PORT, sendMessageChan)

	broadcastTicker := time.NewTicker(STATUS_UPDATE_INTERVAL_MS * time.Millisecond)
	assignRequestTicker := time.NewTicker(REQUEST_ASSIGNMENT_INTERVAL_MS * time.Millisecond)

	peerList := []string{}

	isNetworkConnected := false

	hallRequests := [datatypes.N_FLOORS][datatypes.N_HALL_BUTTONS]datatypes.RequestType{}
	allCabRequests := make(map[string][datatypes.N_FLOORS]datatypes.RequestType)
	updatedInfoElevs := make(map[string]datatypes.ElevatorInfo)

	// initialiserer den lokale heisinformasjonen med localID:
	allCabRequests[localID] = [datatypes.N_FLOORS]datatypes.RequestType{}
	updatedInfoElevs[localID] = elevator_control.GetInfoElev()

	// hovedloop - for-løkke med select
	for {
		select {
		case btn := <- buttenEventChan:
			request := datatypes.RequestType{}

			if btn.Button == elevio.ButtonType(datatypes.BT_CAB) {
				request = allCabRequests[localID][btn.Button]
			} else {
				if !isNetworkConnected {
					break 		// dersom ikke connected skal ikke hallrequesten legges til i requests
				}
				request = hallRequests[btn.Floor][btn.Button]
			}
			// switch case for å håndtere statusendringer for en forespørsel, basert på hva som skjer ved knappetrykk
			switch request.State {
			case datatypes.Completed:
				request.State = datatypes.Unassigned
				request.AwareList = []string{localID}		// setter at heis med localID er aware of denne request
				elevio.SetButtonLamp(btn.Button, btn.Floor, true)

			case datatypes.Unassigned:
				if isSubset(peerList, request.AwareList) {		// må definere denne funksjonen TODO
					request.State = datatypes.Completed
					request.AwareList = []string{localID}
					elevio.SetButtonLamp(btn.Button, btn.Floor, true)
				}
			}
			if btn.Button == elevio.ButtonType(datatypes.BT_CAB) {		// hvis det er en cab button
				localCabReqs := allCabRequests[localID]
				localCabReqs[btn.Floor] = request
				allCabRequests[localID] = localCabReqs
			} else {
				hallRequests[btn.Floor][btn.Button] = request
			}

		case btn := <-completedReqChan:
			request := datatypes.RequestType{}
			if btn.Button == datatypes.BT_CAB {
				request = allCabRequests[localID][btn.Floor]
			} else {
				request = hallRequests[btn.Floor][btn.Button]
			}
			// switch case med kun en case, for å sikre at det kun er tilfelles Assigned som blir håndtert:
			switch request.State {
			case datatypes.Assigned:
				request.State = datatypes.Completed
				request.AwareList = []string{localID}
				request.Count++
				elevio.SetButtonLamp(elevio.ButtonType(btn.Button), btn.Floor, false)
			}
			if btn.Button == datatypes.BT_CAB {
				localCabReqs := allCabRequests[localID]
				localCabReqs[btn.Floor] = request
				allCabRequests[localID] = localCabReqs
			} else {
				hallRequests[btn.Floor][btn.Button] = request
			}
		case <-broadcastTicker.C:
			info := elevator_control.GetInfoElev()
			updatedInfoElevs[localID] = info

			newMsg := datatypes.NetworkMsg{
				SenderID: localID,
				Available: info.Available,
				Behavior: info.Behaviour,
				Floor: info.CurrentFloor,
				Direction: info.Direction,
				SenderHallRequests: hallRequests,
				AllCabRequests: allCabRequests,
			}
			if isNetworkConnected {
				sendMessageChan <- newMsg
			}

		case <-assignRequestTicker.C:
			select {
			case reqChan <- request_handler.RequestAssigner(hallRequests, allCabRequests, updatedInfoElevs, peerList, localID):
			default:

			}
		case peer := <-peerUpdateChan:
			peerList = peer.Peers

			if peer.New == localID {
				isNetworkConnected = true
			}
			if isSubset([]string{localID}, peer.Lost) {
				isNetworkConnected = false
			}

		case msg := <-sendMessageChan:
			if msg.SenderID == localID {
				break		// godtar ikke message dersom avsender er seg selv
			}
			if !isNetworkConnected {
				break		// godtar ikke message dersom ikke connected til network
			}
			updatedInfoElevs[msg.SenderID] = datatypes.ElevatorInfo{
				Behaviour: msg.Behavior,
				Direction: msg.Direction,
				Available: msg.Available,
				CurrentFloor: msg.Floor,
			}
			for ID, cabReqs := range msg.AllCabRequests {
				if _, IDExists := allCabRequests[ID]; !IDExists {
					// dette er da første informasjon om denne heisen
					for floor := range cabReqs {
						cabReqs[floor].AwareList = appendAwareList(cabReqs[floor].AwareList, localID)		// TODO: definere denne 
					}
					allCabRequests[ID] = cabReqs
					continue
				}
			for f := 0; f < datatypes.N_FLOORS; f++ {
				if !
			}
			}

		}
	}
}
