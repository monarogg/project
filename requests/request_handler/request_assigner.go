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
	HallRequests [config.N_FLOORS][config.N_HALL_BUTTONS]bool `json:"hallRequests"`
	States       map[string]HRAElevState                      `json:"states"`
}

func RequestAssigner(
	hallRequests [config.N_FLOORS][config.N_HALL_BUTTONS]datatypes.RequestType,
	allCabRequests map[string][config.N_FLOORS]datatypes.RequestType,
	updatedInfoElevs map[string]datatypes.ElevatorInfo,
	peerList []string,
	localID string,
) map[string][config.N_FLOORS][config.N_BUTTONS]bool {

	fmt.Println("Start RequestAssigner")
	fmt.Println("Mottatt hallRequests:", hallRequests)
	fmt.Println("Mottatt allCabRequests:", allCabRequests)
	fmt.Println("Mottatt updatedInfoElevs:", updatedInfoElevs)
	fmt.Println("Mottatt peerList:", peerList)
	fmt.Println("Mottatt localID:", localID)

	hraPath := "./hall_request_assigner"

	hallRequestsBool := [config.N_FLOORS][config.N_HALL_BUTTONS]bool{}

	for f := 0; f < config.N_FLOORS; f++ {
		for b := 0; b < config.N_HALL_BUTTONS; b++ {
			req := hallRequests[f][b]
			if req.State == config.Unassigned && isActiveRequest(req) {
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
		cabs := [config.N_FLOORS]bool{}
		for f := 0; f < config.N_FLOORS; f++ {
			if cabReqs[f].State == config.Assigned {
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
		return map[string][config.N_FLOORS][config.N_BUTTONS]bool{}
	}

	input := HRAInput{
		HallRequests: hallRequestsBool,
		States:       inputStates,
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		fmt.Println("JSON Marshal Error:", err)
		return map[string][config.N_FLOORS][config.N_BUTTONS]bool{}
	}

	cmd := exec.Command(hraPath, "-i", string(jsonBytes), "--includeCab")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("exec.Command error:", err)
		fmt.Println("Command output:", string(out))
		return map[string][config.N_FLOORS][config.N_BUTTONS]bool{}
	}

	output := new(map[string][config.N_FLOORS][config.N_BUTTONS]bool)

	if err = json.Unmarshal(out, &output); err != nil {
		fmt.Println("json.Unmarshal error:", err)
		return map[string][config.N_FLOORS][config.N_BUTTONS]bool{}
	}

	fmt.Println("Final assigned hallRequests for", localID, ":", (*output)[localID])
	fmt.Println("Full assignment result:", *output)

	return *output
}

func dirToS(dir config.Direction) string {
	switch dir {
	case config.DIR_DOWN:
		return "down"
	case config.DIR_STOP:
		return "stop"
	case config.DIR_UP:
		return "up"
	}
	return "stop"
}

func behToS(beh config.ElevBehaviour) string {
	switch beh {
	case config.Idle:
		return "idle"
	case config.DoorOpen:
		return "doorOpen"
	case config.Moving:
		return "moving"
	}
	return "idle"
}

func isActiveRequest(r datatypes.RequestType) bool {
	return len(r.AwareList) > 0
}

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}
