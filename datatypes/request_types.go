package datatypes

import (
	"project/config"
	"project/elevio"
)

type RequestState int

const (
	Unassigned RequestState = 0
	Assigned   RequestState = 1
	Completed  RequestState = 2
)

type RequestType struct {
	State     RequestState
	Count     int
	AwareList []string
}

type ElevatorInfo struct {
	Available    bool
	Behaviour    config.ElevBehaviour
	Direction    config.Direction
	CurrentFloor int
}

type NetworkMsg struct {
	SenderID           string
	Available          bool
	Behavior           config.ElevBehaviour
	Direction          elevio.MotorDirection
	Floor              int
	SenderHallRequests [config.N_FLOORS][config.N_HALL_BUTTONS]RequestType
	AllCabRequests     map[string][config.N_FLOORS]RequestType
	DebugLog           string
}
