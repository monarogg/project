package requests

// skal håndtere koordinering av knappetrykk, network messages og fordeling av bestillinger mellom heisene

import (
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
		case btn := <-buttenEventChan:
			request := datatypes.RequestType{}

			if btn.Button == elevio.ButtonType(datatypes.BT_CAB) {
				request = allCabRequests[localID][btn.Button]
			} else {
				if !isNetworkConnected {
					break // dersom ikke connected skal ikke hallrequesten legges til i requests
				}
				request = hallRequests[btn.Floor][btn.Button]
			}
			// switch case for å håndtere statusendringer for en forespørsel, basert på hva som skjer ved knappetrykk
			switch request.State {
			case datatypes.Completed:
				request.State = datatypes.Unassigned
				request.AwareList = []string{localID} // setter at heis med localID er aware of denne request
				elevio.SetButtonLamp(btn.Button, btn.Floor, true)

			case datatypes.Unassigned:
				if isContainedIn(peerList, request.AwareList) { // må definere denne funksjonen TODO
					request.State = datatypes.Completed
					request.AwareList = []string{localID}
					elevio.SetButtonLamp(btn.Button, btn.Floor, true)
				}
			}
			if btn.Button == elevio.ButtonType(datatypes.BT_CAB) { // hvis det er en cab button
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
				SenderID:           localID,
				Available:          info.Available,
				Behavior:           info.Behaviour,
				Floor:              info.CurrentFloor,
				Direction:          elevio.MotorDirection(info.Direction),
				SenderHallRequests: hallRequests,
				AllCabRequests:     allCabRequests,
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
			if isContainedIn([]string{localID}, peer.Lost) {
				isNetworkConnected = false
			}

		case msg := <-sendMessageChan:
			if msg.SenderID == localID {
				break // godtar ikke message dersom avsender er seg selv
			}
			if !isNetworkConnected {
				break // godtar ikke message dersom ikke connected til network
			}
			updatedInfoElevs[msg.SenderID] = datatypes.ElevatorInfo{
				Behaviour:    msg.Behavior,
				Direction:    datatypes.Direction(msg.Direction),
				Available:    msg.Available,
				CurrentFloor: msg.Floor,
			}
			for ID, cabReqs := range msg.AllCabRequests {
				if _, IDExists := allCabRequests[ID]; !IDExists {
					// dette er da første informasjon om denne heisen
					for floor := range cabReqs {
						cabReqs[floor].AwareList = addIfMissing(cabReqs[floor].AwareList, localID) // TODO: definere denne
					}
					allCabRequests[ID] = cabReqs
					continue
				}
				for f := 0; f < datatypes.N_FLOORS; f++ {
					if !canAcceptRequest(allCabRequests[ID][f], cabReqs[f]) { // sjekker om request kan aksepteres, hvis ikke hopper over denne f
						continue
					}
					acceptedReqs := cabReqs[f]                                             // request fra aktuell f lagres acceptedReqs
					acceptedReqs.AwareList = addIfMissing(acceptedReqs.AwareList, localID) // sørger for at localID er med i AwareList

					// sjekker at requesten er unassigned og at alle peers er aware of denne request:
					if acceptedReqs.State == datatypes.Unassigned && isContainedIn(peerList, acceptedReqs.AwareList) {
						acceptedReqs.State = datatypes.Assigned
						acceptedReqs.AwareList = []string{localID}
					}

					// sjekker at request gjelder lokal heis og om den er assigned:
					if ID == localID && acceptedReqs.State == datatypes.Assigned {
						// da settes buttonlamp for å indikere at heisen skal til den f:
						elevio.SetButtonLamp(elevio.ButtonType(datatypes.BT_CAB), f, true)
					}

					// for å oppdatere allCabRequests:
					tempCabReqs := allCabRequests[ID]
					tempCabReqs[f] = acceptedReqs
					allCabRequests[ID] = tempCabReqs
				}
			}
			for f := 0; f < datatypes.N_FLOORS; f++ {
				for b := 0; b < datatypes.N_HALL_BUTTONS; b++ {
					// sjekker om inkommende request for gjeldende f og b skal aksepteres, dersom ikke - hopper over denne kombinasjonen
					if !canAcceptRequest(hallRequests[f][b], msg.SenderHallRequests[f][b]) {
						continue
					}
					acceptedReqs := msg.SenderHallRequests[f][b]
					// legger til i awareList dersom localID ikke er der:
					acceptedReqs.AwareList = addIfMissing(acceptedReqs.AwareList, localID)

					// oppdaterer basert på state i acceptedReqs
					switch acceptedReqs.State {
					case datatypes.Assigned:
						elevio.SetButtonLamp(elevio.ButtonType(b), f, true)
					case datatypes.Unassigned:
						elevio.SetButtonLamp(elevio.ButtonType(b), f, false) // skrur først av buttonlamp
						if isContainedIn(peerList, acceptedReqs.AwareList) { // sjekker om alle peers er aware av request
							acceptedReqs.State = datatypes.Assigned             // endrer da til assigned
							acceptedReqs.AwareList = []string{localID}          // endrer slik at kun localID er aware
							elevio.SetButtonLamp(elevio.ButtonType(b), f, true) // skrur på buttonlamp
						}
					case datatypes.Completed: // hvis request er completed - skru av buttonlight
						elevio.SetButtonLamp(elevio.ButtonType(b), f, false)
					}
					// oppdaterer hallRequests med aksepterte og evt endrede forespørsler:
					hallRequests[f][b] = acceptedReqs
				}
			}
		}
	}
}
