package elevator_control

import (
	"project/config"
	"project/elevio"
)

// oversetter en Direction (int) til en motorretning (MotorDirection)
func DirConv(dir config.Direction) elevio.MotorDirection {
	switch dir {
	case config.DIR_DOWN:
		return elevio.MD_Down
	case config.DIR_STOP:
		return elevio.MD_Stop
	case config.DIR_UP:
		return elevio.MD_Up
	default:
		return elevio.MD_Stop
	}
}
