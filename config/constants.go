package config

const (
	N_FLOORS       = 4
	N_BUTTONS      = 3
	N_HALL_BUTTONS = 2
)

const (
	DOOR_OPEN_DURATION = 3
	MOVEMENT_TIMEOUT   = 4
)

const (
	PEER_PORT                      = 30060
	MSG_PORT                       = 30061
	STATUS_UPDATE_INTERVAL_MS      = 200
	REQUEST_ASSIGNMENT_INTERVAL_MS = 1000
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

type MotorDirection int

const (
	MD_UP   MotorDirection = 1
	MD_DOWN MotorDirection = -1
	MD_STOP MotorDirection = 0
)

// type ButtonType int

// const (
// 	BT_HallUP   ButtonType = 0
// 	BT_HallDOWN ButtonType = 1
// 	BT_CAB      ButtonType = 2
// )
