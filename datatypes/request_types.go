package datatypes

type RequestState int

const (
	Completed  RequestState = 0
	Unassigned RequestState = 1
	Assigned   RequestState = 2
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
	Direction          Direction
	Floor              int
	SenderHallRequests [N_FLOORS][N_HALL_BUTTONS]RequestType
	AllCabRequests     map[string][N_FLOORS]RequestType
}
