package datatypes

import (
	"project/elevio"
	"time"
)

const N_FLOORS = 4
const N_BUTTONS = 3
const N_HALL_BUTTONS = 2

type ElevatorState int

const (
	Idle     ElevatorState = 0
	Moving   ElevatorState = 1
	DoorOpen ElevatorState = 2
)

type Elevator struct {
	CurrentFloor int
	Direction    elevio.MotorDirection
	State        ElevatorState
	Orders       [4][3]bool
	Config       ElevatorConfig
	StopActive   bool
}

type ElevatorConfig struct {
	DoorOpenDuration time.Duration
}
