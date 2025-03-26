package requests

import (
	"fmt"
	"project/config"
	"project/datatypes"
	"project/elevio"
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

func getReqTypeHere(elevator datatypes.Elevator) elevio.ButtonType {
	for b := 0; b < config.N_BUTTONS; b++ {
		if elevator.Orders[elevator.CurrentFloor][b] {
			return elevio.ButtonType(b)
		}
	}
	print("buttontype not found")
	return elevio.BT_Cab
}

func ChooseNewDirAndBeh(elevator datatypes.Elevator) (config.Direction, config.ElevBehaviour) {
	if !RequestsAbove(elevator) && !RequestsBelow(elevator) && !RequestsHere(elevator) {
		return config.DIR_STOP, config.Idle
	}
	switch elevator.Direction {
	case config.DIR_UP:
		if RequestsAbove(elevator) {
			return config.DIR_UP, config.Moving
		} else if RequestsHere(elevator) {
			return config.DIR_DOWN, config.DoorOpen
		} else if RequestsBelow(elevator) {
			return config.DIR_DOWN, config.Moving
		} else {
			return config.DIR_STOP, config.Idle
		}

	case config.DIR_DOWN:
		if RequestsBelow(elevator) {
			return config.DIR_DOWN, config.Moving
		} else if RequestsHere(elevator) {
			return config.DIR_UP, config.DoorOpen
		} else if RequestsAbove(elevator) {
			return config.DIR_UP, config.Moving
		} else {
			return config.DIR_STOP, config.Idle
		}

	case config.DIR_STOP:
		if RequestsHere(elevator) {
			switch getReqTypeHere(elevator) {
			case elevio.BT_HallUp:
				return config.DIR_UP, config.DoorOpen
			case elevio.BT_HallDown:
				return config.DIR_DOWN, config.DoorOpen
			case elevio.BT_Cab:
				return config.DIR_STOP, config.DoorOpen
			}
		} else if RequestsAbove(elevator) {
			return config.DIR_UP, config.Moving
		} else if RequestsBelow(elevator) {
			return config.DIR_DOWN, config.Moving
		} else {
			return config.DIR_STOP, config.Idle
		}
	}

	fmt.Println("Debug: Choosing Direction. Orders:", elevator.Orders, "Current Floor:", elevator.CurrentFloor)
	return config.DIR_STOP, config.Idle
}

func ShouldStop(elevator datatypes.Elevator) bool {
	floor := elevator.CurrentFloor

	switch elevator.Direction {
	case config.DIR_UP:
		return elevator.Orders[floor][elevio.BT_HallUp] ||
			elevator.Orders[floor][elevio.BT_Cab] ||
			!RequestsAbove(elevator)

	case config.DIR_DOWN:
		return elevator.Orders[floor][elevio.BT_HallDown] ||
			elevator.Orders[floor][elevio.BT_Cab] ||
			!RequestsBelow(elevator)

	case config.DIR_STOP:
		return elevator.Orders[floor][elevio.BT_HallUp] ||
			elevator.Orders[floor][elevio.BT_HallDown] ||
			elevator.Orders[floor][elevio.BT_Cab]
	}

	return false
}

func CanClearCab(elevator datatypes.Elevator) bool {
	return elevator.Orders[elevator.CurrentFloor][elevio.BT_Cab]
}

func CanClearHallUp(elevator datatypes.Elevator) bool {
	currentFloor := elevator.CurrentFloor
	if !elevator.Orders[currentFloor][elevio.BT_HallUp] {
		return false
	}
	switch elevator.Direction {
	case config.DIR_UP, config.DIR_STOP:
		return true
	case config.DIR_DOWN:
		return !RequestsBelow(elevator) && !elevator.Orders[currentFloor][elevio.BT_HallDown]
	}
	return false
}

func CanClearHallDown(elevator datatypes.Elevator) bool {
	currentFloor := elevator.CurrentFloor
	if !elevator.Orders[currentFloor][elevio.BT_HallDown] {
		return false
	}
	switch elevator.Direction {
	case config.DIR_DOWN, config.DIR_STOP:
		return true
	case config.DIR_UP:
		return !RequestsAbove(elevator) && !elevator.Orders[currentFloor][elevio.BT_HallUp]
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
