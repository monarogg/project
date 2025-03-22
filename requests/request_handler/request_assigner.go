package requesthandler

import (
    "encoding/json"
    "fmt"
    "os/exec"
    "project/datatypes"
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
    localID string,
) [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool {

    fmt.Println("Start RequestAssigner")
    fmt.Println("Mottatt hallRequests:", hallRequests)
    fmt.Println("Mottatt allCabRequests:", allCabRequests)
    fmt.Println("Mottatt updatedInfoElevs:", updatedInfoElevs)
    fmt.Println("Mottatt peerList:", peerList)
    fmt.Println("Mottatt localID:", localID)

    hraPath := "./hall_request_assigner"

    // Build input for HRA
    hallRequestsBool := [datatypes.N_FLOORS][datatypes.N_HALL_BUTTONS]bool{}

    fmt.Println("Debug: Checking Hall Request State Before Assignment")
    for f := 0; f < datatypes.N_FLOORS; f++ {
        for b := 0; b < datatypes.N_HALL_BUTTONS; b++ {
            s := hallRequests[f][b].State
            fmt.Printf("Floor %d, Button %d, State: %d\n", f, b, s)
        }
    }

    // Only add Unassigned hall calls for external assignment
    for f := 0; f < datatypes.N_FLOORS; f++ {
        for b := 0; b < datatypes.N_HALL_BUTTONS; b++ {
            if hallRequests[f][b].State == datatypes.Unassigned && isActiveRequest(hallRequests[f][b]) {
    			hallRequestsBool[f][b] = true
}
        }
    }

    inputStates := map[string]HRAElevState{}

    fmt.Println("Debug: Checking Cab Request State Before Assignment")
    for ID, cabReqs := range allCabRequests {
        elevInfo, ok := updatedInfoElevs[ID]
        if !ok || !elevInfo.Available {
            continue
        }
        cabs := [datatypes.N_FLOORS]bool{}
        for f := 0; f < datatypes.N_FLOORS; f++ {
            if cabReqs[f].State == datatypes.Assigned {
                cabs[f] = true
            }
        }
        fmt.Println("Elevator:", ID, "Cab Requests:", cabs)

        inputStates[ID] = HRAElevState{
            Behavior:    behToS(elevInfo.Behaviour),
            Floor:       elevInfo.CurrentFloor,
            Direction:   dirToS(elevInfo.Direction),
            CabRequests: cabs[:],
        }
    }

    // If no active elevators available, do nothing
    if len(inputStates) == 0 {
        return [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool{}
    }

    input := HRAInput{
        HallRequests: hallRequestsBool,
        States:       inputStates,
    }

    fmt.Println("Final Hall Requests before sending:", hallRequestsBool)
    fmt.Println("Final Cab Requests before sending:", inputStates)

    jsonBytes, err := json.Marshal(input)
    if err != nil {
        fmt.Println("JSON Marshal Error:", err)
        return [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool{}
    }
    fmt.Println("Final JSON Payload:", string(jsonBytes))

    cmd := exec.Command(hraPath, "-i", string(jsonBytes), "--includeCab")
    out, err := cmd.CombinedOutput()
    if err != nil {
        fmt.Println("exec.Command error:", err)
        fmt.Println("Command output:", string(out))
        return [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool{}
    }

    output := new(map[string][datatypes.N_FLOORS][datatypes.N_BUTTONS]bool)
    if err = json.Unmarshal(out, &output); err != nil {
        fmt.Println("json.Unmarshal error:", err)
        return [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool{}
    }

    fmt.Println("Final assigned hallRequests for", localID, ":", (*output)[localID])
    return (*output)[localID]
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