package config

import "time"

const (
	N_FLOORS       = 4
	N_BUTTONS      = 3
	N_HALL_BUTTONS = 2

	DOOR_OPEN_DURATION = 3
	MOVEMENT_TIMEOUT   = 4

	PEER_PORT                   = 30060
	MSG_PORT                    = 30061
	STATUS_UPDATE_INTERVAL      = 200 * time.Millisecond
	REQUEST_ASSIGNMENT_INTERVAL = 1000 * time.Millisecond
)

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

type RequestState int

const (
	Unassigned RequestState = 0
	Assigned   RequestState = 1
	Completed  RequestState = 2
)
