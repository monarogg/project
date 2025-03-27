package datatypes

import (
	"project/config"
	"sync"
	"time"
)

type Elevator struct {
	CurrentFloor int
	Direction    config.Direction
	State        config.ElevBehaviour
	Orders       [config.N_FLOORS][config.N_BUTTONS]bool
	Config       ElevatorConfig
	StopActive   bool
}

type ElevatorConfig struct {
	DoorOpenDuration time.Duration
}

type ElevSharedInfo struct {
	Available    bool
	Behaviour    config.ElevBehaviour
	Direction    config.Direction
	CurrentFloor int
	Mutex        sync.RWMutex
}
