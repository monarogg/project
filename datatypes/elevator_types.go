package datatypes

import (
	"project/config"
	"sync"
	"time"
)

const N_FLOORS = 4
const N_BUTTONS = 3
const N_HALL_BUTTONS = 2

type ElevBehaviour int

const (
	Idle     ElevBehaviour = 0
	Moving   ElevBehaviour = 1
	DoorOpen ElevBehaviour = 2
)

type Direction int

const (
	DIR_STOP Direction = 0
	DIR_UP   Direction = 1
	DIR_DOWN Direction = 2
)

type Elevator struct {
	CurrentFloor int
	Direction    Direction
	State        ElevBehaviour
	Orders       [config.N_FLOORS][config.N_BUTTONS]bool
	Config       ElevatorConfig
	StopActive   bool
}

type ElevSharedInfo struct {
	Available    bool
	Behaviour    ElevBehaviour
	Direction    Direction
	CurrentFloor int
	Mutex        sync.RWMutex
}

type ElevatorConfig struct {
	DoorOpenDuration time.Duration
}
