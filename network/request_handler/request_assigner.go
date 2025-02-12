package requesthandler

import (
	"project/datatypes"
)

type HRAElevState struct {
	Behavior    string `json:"behaviour"`
	Floor       int    `json:"floor"`
	Direction   string `json:"direction"`
	CabRequests []bool `json:"cabRequests"`
}

type HRAInput struct {
	HallRequests [datatypes.N_FLOORS][2]bool `json:"hallRequests"`
	States       map[string]HRAElevState     `json:"states"`
}

func RequestAssigner(
	hallRequests [datatypes.N_FLOORS][datatypes.N_HALL_BUTTONS]datatypes.RequestType,
	allCabRequests map[string][datatypes.N_FLOORS]datatypes.RequestType,
	updatedInfoElevs map[string]datatypes.ElevatorInfo,
	peerList []string,
	localID string) [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool {

	HRAExecutablePath := "hall_request_assigner"

	hallRequestsBool := [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool{}

	for floor := 0; floor < datatypes.N_FLOORS; floor++ {
		for button := 0; button < datatypes.N_BUTTONS; button++ {
			if hallRequests[floor][button].State == datatypes.Assigned {
				// hallRequestsBool skal gi en oversikt over requests som er assigned (true)
				hallRequestsBool[floor][button] = true
			}
		}
	}

	inputStates := map[string]HRAElevState{}

	for ID, cabRequests := range allCabRequests {
		elevatorINFO, exists := updatedInfoElevs[ID]
		if !exists {
			continue
		}
		if !elevatorINFO.Available {
			continue
		}

		if !sliceContains {
			// skal oppdatere etter å ha laget en funksjon
		}

		cabRequestsBool := [datatypes.N_FLOORS]bool{}

		for floor := 0; floor < datatypes.N_FLOORS; floor++ {
			if cabRequests[floor].State == datatypes.Assigned {
				cabRequestsBool[floor] = true
			}
		}
		inputStates[ID] = HRAElevState{
			Behavior:    behToS(elevatorINFO.Behaviour), // MÅ lage funksjon behaviour to string
			Floor:       elevatorINFO.Floor,
			Direction:   dirToS(elevatorINFO.Direction), // Må lage funksjon direction to string
			CabRequests: cabRequestsBool[:],
		}
	}
	if len(inputStates) == 0 {
		return [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool{}
	}

	input := HRAInput{
		HallRequests: hallRequestsBool,
		States:       inputStates,
	}
}
