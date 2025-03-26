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

			// Filter AwareList to remove unavailable elevators
			filteredAware := []string{}
			for _, id := range req.AwareList {
				if info, ok := updatedInfoElevs[id]; ok && info.Available {
					filteredAware = append(filteredAware, id)
				}
			}
			req.AwareList = filteredAware

			if req.State == datatypes.Assigned && !contains(req.AwareList, localID) {
				if len(req.AwareList) == 0 || !contains(peerList, req.AwareList[0]) {
					fmt.Printf("[DEMOTE] Lost assigned elevator for Floor %d Button %d. Resetting to Unassigned.\n", f, b)
					req.State = datatypes.Unassigned
				}
			}

			hallRequests[f][b] = req

			if req.State != datatypes.Completed && isActiveRequest(req) {
				hallRequestsBool[f][b] = true
			}
			fmt.Printf("[ASSIGNER] Ready for assignment: Floor %d Button %d | State=%v | AwareList=%v\n",
				f, b, req.State, req.AwareList)

		}
	}

	// Prepare elevator state input
	inputStates := map[string]HRAElevState{}
	for ID, cabReqs := range allCabRequests {
		elevInfo, ok := updatedInfoElevs[ID]
		if !ok || !elevInfo.Available || !contains(peerList, ID) {
			continue
		}
		cabs := [config.N_FLOORS]bool{}
		for f := 0; f < config.N_FLOORS; f++ {
			if isActiveRequest(cabReqs[f]) {
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

	fmt.Println("Sending to HRA JSON:\n", string(jsonBytes))

	cmd := exec.Command(hraPath, "-i", string(jsonBytes), "--includeCab", "--costFn=Basic")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("exec.Command error:", err)
		fmt.Println("Command output:", string(out))
		return map[string][config.N_FLOORS][config.N_BUTTONS]bool{}
	}

	output := new(map[string][datatypes.N_FLOORS][datatypes.N_BUTTONS]bool)

	if err = json.Unmarshal(out, &output); err != nil {
		fmt.Println("json.Unmarshal error:", err)
		return map[string][config.N_FLOORS][config.N_BUTTONS]bool{}
	}

	fmt.Println("Final assigned hallRequests for", localID, ":", (*output)[localID])
	fmt.Println("Full assignment result:", *output)

	// Update internal hallRequests with assignments
	for f := 0; f < config.N_FLOORS; f++ {
		for b := 0; b < config.N_HALL_BUTTONS; b++ {
			for assignedTo, assignedMatrix := range *output {
				if assignedMatrix[f][b] {
					hallRequests[f][b] = datatypes.RequestType{
						State:     datatypes.Assigned,
						AwareList: []string{assignedTo},
					}
					fmt.Printf("[ASSIGNED] Floor %d Button %d to %s\n", f, b, assignedTo)
				}
			}
		}
	}

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

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}
