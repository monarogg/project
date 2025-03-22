package datatypes

import (
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
	Behaviour    ElevBehaviour
	Direction    Direction
	CurrentFloor int
}

type NetworkMsg struct {
	SenderID           string
	Available          bool
	Behavior           ElevBehaviour
	Direction          elevio.MotorDirection
	Floor              int
	SenderHallRequests [N_FLOORS][N_HALL_BUTTONS]RequestType
	AllCabRequests     map[string][N_FLOORS]RequestType
}
