package config

import "time"

const (
	NUM_FLOORS       = 4
	NUM_BUTTONS      = 3
	NUM_HALL_BUTTONS = 2
)

const (
	PEER_PORT = 45455
	MSG_PORT  = 45456

	STATUS_UPDATE_INTERVAL      = 100 * time.Millisecond
	REQUEST_ASSIGNMENT_INTERVAL = 500 * time.Millisecond
)
