package elevator_control

import (
	"project/config"
	"project/elevio"
)

// oversetter en Direction (int) til en motorretning (MotorDirection)
func DirConv(dir config.Direction) elevio.MotorDirection {
	switch dir {
	case config.DIR_DOWN:
		return elevio.MotorDirection(config.MD_DOWN)
	case config.DIR_STOP:
		return elevio.MotorDirection(config.MD_STOP)
	case config.DIR_UP:
		return elevio.MotorDirection(config.MD_UP)
	}
	return elevio.MotorDirection(config.MD_STOP)
}