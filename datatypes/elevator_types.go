package datatypes

import (
	"project/config"
	"sync"
	"time"
)

// type ElevBehaviour int

// const (
// 	Idle     ElevBehaviour = 0
// 	Moving   ElevBehaviour = 1
// 	DoorOpen ElevBehaviour = 2
// )

// type Direction int

// const (
// 	DIR_STOP Direction = 0
// 	DIR_UP   Direction = 1
// 	DIR_DOWN Direction = 2
// )

type Elevator struct {
	CurrentFloor int
	Direction    config.Direction
	State        config.ElevBehaviour
	Orders       [config.N_FLOORS][config.N_BUTTONS]bool
	Config       ElevatorConfig
	StopActive   bool
}

type ElevSharedInfo struct {
	Available    bool
	Behaviour    config.ElevBehaviour
	Direction    config.Direction
	CurrentFloor int
	Mutex        sync.RWMutex
}

type ElevatorConfig struct {
	DoorOpenDuration time.Duration
}
