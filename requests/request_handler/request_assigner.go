package requesthandler

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"project/config"
	"project/datatypes"
)

type HRAElevState struct {
	Behavior    string `json:"behaviour"`
	Floor       int    `json:"floor"`
	Direction   string `json:"direction"`
	CabRequests []bool `json:"cabRequests"`
}

type HRAInput struct {
	HallRequests [config.NUM_FLOORS][config.NUM_HALL_BUTTONS]bool `json:"hallRequests"`
	States       map[string]HRAElevState                          `json:"states"`
}

func HRAmain(
	hallRequests [config.NUM_FLOORS][config.NUM_HALL_BUTTONS]datatypes.RequestType,
	allCabRequests map[string][config.NUM_FLOORS]datatypes.RequestType,
	updatedInfoElevs map[string]datatypes.ElevatorInfo,
	peerList []string,
	localID string,
) map[string][config.NUM_FLOORS][config.NUM_BUTTONS]bool {

	fmt.Println("Start HRAmain")
	fmt.Println("Mottatt hallRequests:", hallRequests)
	fmt.Println("Mottatt allCabRequests:", allCabRequests)
	fmt.Println("Mottatt updatedInfoElevs:", updatedInfoElevs)
	fmt.Println("Mottatt peerList:", peerList)
	fmt.Println("Mottatt localID:", localID)

	hraPath := "./cmd/hall_request_assigner"

	// Prepare hall request input: Only unassigned and active
	hallRequestsBool := [config.NUM_FLOORS][config.NUM_HALL_BUTTONS]bool{}
	for f := 0; f < config.NUM_FLOORS; f++ {
		for b := 0; b < config.NUM_HALL_BUTTONS; b++ {
			req := hallRequests[f][b]
			if req.State == datatypes.Unassigned && isActiveRequest(req) {
				hallRequestsBool[f][b] = true
			}
		}
	}

	// Prepare elevator state input
	inputStates := map[string]HRAElevState{}
	for ID, cabReqs := range allCabRequests {
		elevInfo, ok := updatedInfoElevs[ID]
		if !ok || !elevInfo.Available {
			continue
		}
		cabs := [config.NUM_FLOORS]bool{}
		for f := 0; f < config.NUM_FLOORS; f++ {
			if cabReqs[f].State == datatypes.Assigned {
				cabs[f] = true
			}
		}
		inputStates[ID] = HRAElevState{
			Behavior:    behToS(elevInfo.Behaviour),
			Floor:       elevInfo.CurrentFloor,
			Direction:   dirToS(elevInfo.Direction),
			CabRequests: cabs[:],
		}
	}

	if len(inputStates) == 0 {
		return map[string][config.NUM_FLOORS][config.NUM_BUTTONS]bool{}
	}

	input := HRAInput{
		HallRequests: hallRequestsBool,
		States:       inputStates,
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		fmt.Println("JSON Marshal Error:", err)
		return map[string][config.NUM_FLOORS][config.NUM_BUTTONS]bool{}
	}

	cmd := exec.Command(hraPath, "-i", string(jsonBytes), "--includeCab")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("exec.Command error:", err)
		fmt.Println("Command output:", string(out))
		return map[string][config.NUM_FLOORS][config.NUM_BUTTONS]bool{}
	}

	output := new(map[string][datatypes.NUM_FLOORS][datatypes.NUM_BUTTONS]bool)
	if err = json.Unmarshal(out, &output); err != nil {
		fmt.Println("json.Unmarshal error:", err)
		return map[string][config.NUM_FLOORS][config.NUM_BUTTONS]bool{}
	}

	fmt.Println("Final assigned hallRequests for", localID, ":", (*output)[localID])
	fmt.Println("Full assignment result:", *output)

	return *output
}

func dirToS(dir datatypes.Direction) string {
	switch dir {
	case datatypes.DIR_DOWN:
		return "down"
	case datatypes.DIR_STOP:
		return "stop"
	case datatypes.DIR_UP:
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

func isActiveRequest(r datatypes.RequestType) bool {
	return len(r.AwareList) > 0
}
