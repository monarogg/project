package requests

import (
	"fmt"
	"project/config"
	"project/datatypes"
)

const (
	N_FLOORS  = config.N_FLOORS
	N_BUTTONS = config.N_BUTTONS
)

func RequestsAbove(elevator datatypes.Elevator) bool {
	for f := elevator.CurrentFloor + 1; f < config.N_FLOORS; f++ {
		for _, order := range elevator.Orders[f] {
			if order {
				return true
			}
		}
	}
	return false
}

func RequestsBelow(elevator datatypes.Elevator) bool { // skal returnere true/false om det er noen aktive orders i etasjer under
	for f := 0; f < elevator.CurrentFloor; f++ {
		for _, order := range elevator.Orders[f] {
			if order {
				return true
			}
		}
	}
	return false
}

func RequestsHere(elevator datatypes.Elevator) bool {
	fmt.Println("RequestsHere check at floor", elevator.CurrentFloor, "orders:", elevator.Orders[elevator.CurrentFloor])
	for b := 0; b < config.N_BUTTONS; b++ {
		if elevator.Orders[elevator.CurrentFloor][b] {
			return true
		}
	}
	return false
}

func getReqTypeHere(elevator datatypes.Elevator) datatypes.ButtonType {
	for b := 0; b < config.N_BUTTONS; b++ {
		if elevator.Orders[elevator.CurrentFloor][b] {
			return datatypes.ButtonType(b)
		}
	}
	print("buttontype not found")
	return datatypes.BT_CAB
}

func ChooseNewDirAndBeh(elevator datatypes.Elevator) (datatypes.Direction, datatypes.ElevBehaviour) {
	if !RequestsAbove(elevator) && !RequestsBelow(elevator) && !RequestsHere(elevator) {
		return datatypes.DIR_STOP, datatypes.Idle
	}
	switch elevator.Direction {
	case datatypes.DIR_UP:
		if RequestsAbove(elevator) {
			return datatypes.DIR_UP, datatypes.Moving
		} else if RequestsHere(elevator) {
			return datatypes.DIR_DOWN, datatypes.DoorOpen
		} else if RequestsBelow(elevator) {
			return datatypes.DIR_DOWN, datatypes.Moving
		} else {
			return datatypes.DIR_STOP, datatypes.Idle
		}

	case datatypes.DIR_DOWN:
		if RequestsBelow(elevator) {
			return datatypes.DIR_DOWN, datatypes.Moving
		} else if RequestsHere(elevator) {
			return datatypes.DIR_UP, datatypes.DoorOpen
		} else if RequestsAbove(elevator) {
			return datatypes.DIR_UP, datatypes.Moving
		} else {
			return datatypes.DIR_STOP, datatypes.Idle
		}

	case datatypes.DIR_STOP:
		if RequestsHere(elevator) {
			switch getReqTypeHere(elevator) {
			case datatypes.BT_HallUP:
				return datatypes.DIR_UP, datatypes.DoorOpen
			case datatypes.BT_HallDOWN:
				return datatypes.DIR_DOWN, datatypes.DoorOpen
			case datatypes.BT_CAB:
				return datatypes.DIR_STOP, datatypes.DoorOpen
			}
		} else if RequestsAbove(elevator) {
			return datatypes.DIR_UP, datatypes.Moving
		} else if RequestsBelow(elevator) {
			return datatypes.DIR_DOWN, datatypes.Moving
		} else {
			return datatypes.DIR_STOP, datatypes.Idle
		}
	}

	fmt.Println("Debug: Choosing Direction. Orders:", elevator.Orders, "Current Floor:", elevator.CurrentFloor)
	return datatypes.DIR_STOP, datatypes.Idle
}

func ShouldStop(elevator datatypes.Elevator) bool {
	floor := elevator.CurrentFloor

	switch elevator.Direction {
	case datatypes.DIR_UP:
		return elevator.Orders[floor][datatypes.BT_HallUP] ||
			elevator.Orders[floor][datatypes.BT_CAB] ||
			!RequestsAbove(elevator)

	case datatypes.DIR_DOWN:
		return elevator.Orders[floor][datatypes.BT_HallDOWN] ||
			elevator.Orders[floor][datatypes.BT_CAB] ||
			!RequestsBelow(elevator)

	case datatypes.DIR_STOP:
		return elevator.Orders[floor][datatypes.BT_HallUP] ||
			elevator.Orders[floor][datatypes.BT_HallDOWN] ||
			elevator.Orders[floor][datatypes.BT_CAB]
	}

	return false
}

func CanClearCab(elevator datatypes.Elevator) bool {
	return elevator.Orders[elevator.CurrentFloor][datatypes.BT_CAB]
}

func CanClearHallUp(elevator datatypes.Elevator) bool {
	currentFloor := elevator.CurrentFloor
	if !elevator.Orders[currentFloor][datatypes.BT_HallUP] {
		return false
	}
	switch elevator.Direction {
	case datatypes.DIR_UP, datatypes.DIR_STOP:
		return true
	case datatypes.DIR_DOWN:
		return !RequestsBelow(elevator) && !elevator.Orders[currentFloor][datatypes.BT_HallDOWN]
	}
	return false
}

func CanClearHallDown(elevator datatypes.Elevator) bool {
	currentFloor := elevator.CurrentFloor
	if !elevator.Orders[currentFloor][datatypes.BT_HallDOWN] {
		return false
	}
	switch elevator.Direction {
	case datatypes.DIR_DOWN, datatypes.DIR_STOP:
		return true
	case datatypes.DIR_UP:
		return !RequestsAbove(elevator) && !elevator.Orders[currentFloor][datatypes.BT_HallUP]
	}
	return false
}

func MergeOrders(oldOrders, newOrders [config.N_FLOORS][config.N_BUTTONS]bool) [config.N_FLOORS][config.N_BUTTONS]bool {
	for f := 0; f < config.N_FLOORS; f++ {
		for b := 0; b < config.N_BUTTONS; b++ {
			// If newOrders is true, keep it true
			if newOrders[f][b] {
				oldOrders[f][b] = true
			}
		}
	}
	return oldOrders
}
