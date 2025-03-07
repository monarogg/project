package requesthandler

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"project/datatypes"
	"project/elevio"
)

type HRAElevState struct {
	Behavior    string `json:"behaviour"`
	Floor       int    `json:"floor"`
	Direction   string `json:"direction"`
	CabRequests []bool `json:"cabRequests"`
}

type HRAInput struct {
	HallRequests [datatypes.N_FLOORS][datatypes.N_HALL_BUTTONS]bool `json:"hallRequests"`
	States       map[string]HRAElevState                            `json:"states"`
}

func RequestAssigner(
	hallRequests [datatypes.N_FLOORS][datatypes.N_HALL_BUTTONS]datatypes.RequestType,
	allCabRequests map[string][datatypes.N_FLOORS]datatypes.RequestType,
	updatedInfoElevs map[string]datatypes.ElevatorInfo,
	peerList []string,
	localID string) [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool {

	fmt.Println("Start RequestAssigner")
	fmt.Println("Mottatt hallRequests:", hallRequests)
	fmt.Println("Mottatt allCabRequests:", allCabRequests)
	fmt.Println("Mottatt updatedInfoElevs:", updatedInfoElevs)
	fmt.Println("Mottatt peerList:", peerList)
	fmt.Println("Mottatt localID:", localID)

	HRAExecutablePath := "./hall_request_assigner"

	hallRequestsBool := [datatypes.N_FLOORS][datatypes.N_HALL_BUTTONS]bool{}

	for floor := 0; floor < datatypes.N_FLOORS; floor++ {
		for button := 0; button < datatypes.N_HALL_BUTTONS; button++ {
			if button < datatypes.N_HALL_BUTTONS && hallRequests[floor][button].State == datatypes.Assigned {
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

		if !sliceContains(peerList, ID) && ID != localID { // sjekker om ID ikke er i peerlist, og sjekker at ID ikke er lik localID
			continue
		}

		cabRequestsBool := [datatypes.N_FLOORS]bool{}

		for floor := 0; floor < datatypes.N_FLOORS; floor++ {
			if cabRequests[floor].State == datatypes.Assigned {
				cabRequestsBool[floor] = true
			}
		}
		inputStates[ID] = HRAElevState{
			Behavior:    behToS(elevatorINFO.Behaviour),
			Floor:       elevatorINFO.Floor,
			Direction:   dirToS(elevatorINFO.Direction),
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

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		fmt.Println("error with json.Marshal: ", err)
		return [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool{}
	}

	fmt.Println("JSON Payload:", string(jsonBytes)) //debug
	cmd := exec.Command(HRAExecutablePath, "-i", string(jsonBytes), "--includeCab")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("exec.Command error: ", err)
		fmt.Println("Command output: ", string(out))
		// returnerer en tom matrise dersom noe går galt
		return [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool{}
	}

	output := new(map[string][datatypes.N_FLOORS][datatypes.N_BUTTONS]bool)
	err = json.Unmarshal(out, &output) // Unmarshal brukes til å dekode JSON-data fra en strøm (out) og lagre det i en go-variabel
	if err != nil {
		fmt.Println("json.Unmarshal error: ", err)
		return [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool{}
	}

	// fmt.Println(" RequestAssigner MOTTOK:")
	// fmt.Println("   - hallRequests =", hallRequests)
	// fmt.Println("   - allCabRequests =", allCabRequests)
	// fmt.Println("   - updatedInfoElevs =", updatedInfoElevs)
	// fmt.Println("   - peerList =", peerList)

	fmt.Println("Final assigned hallRequests for", localID, ":", (*output)[localID])
	return (*output)[localID] // dereferer pekeren, henter verdien av output, altså selve map objektet

}

func sliceContains(slice []string, elem string) bool { // skal returnere en boolsk verdi avhengig av om slicen inneholder elem
	for _, e := range slice {
		if e == elem {
			return true
		}
	}
	return false
}

func dirToS(dir elevio.MotorDirection) string {
	switch dir {
	case elevio.MD_Down:
		return "down"
	case elevio.MD_Stop:
		return "stop"
	case elevio.MD_Up:
		return "up"
	}
	return "stop"
}

func behToS(beh datatypes.ElevBehaviour) string {
	switch beh {
	case datatypes.Idle:
		return "idle"
	case datatypes.DoorOpen:
		return "doorOpen"
	case datatypes.Moving:
		return "moving"
	}
	return "idle"
}
