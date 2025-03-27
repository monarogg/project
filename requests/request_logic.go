package requests

import (
	"fmt"
	"project/config"
	"project/datatypes"
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
			case datatypes.BT_HallUP:
				return config.DIR_UP, config.DoorOpen
			case datatypes.BT_HallDOWN:
				return config.DIR_DOWN, config.DoorOpen
			case datatypes.BT_CAB:
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

	fmt.Printf("DEBUG ShouldStop: Floor=%d Dir=%v Up=%v Down=%v Cab=%v Above=%v Below=%v\n",
		floor,
		elevator.Direction,
		elevator.Orders[floor][datatypes.BT_HallUP],
		elevator.Orders[floor][datatypes.BT_HallDOWN],
		elevator.Orders[floor][datatypes.BT_CAB],
		RequestsAbove(elevator),
		RequestsBelow(elevator),
	)

	switch elevator.Direction {
	case config.DIR_UP:
		if elevator.Orders[floor][datatypes.BT_HallUP] ||
			elevator.Orders[floor][datatypes.BT_CAB] {
			return true
		}
		// End of line fallback:
		if !RequestsAbove(elevator) && elevator.Orders[floor][datatypes.BT_HallDOWN] {
			return true
		}

	case config.DIR_DOWN:
		if elevator.Orders[floor][datatypes.BT_HallDOWN] ||
			elevator.Orders[floor][datatypes.BT_CAB] {
			return true
		}
		if !RequestsBelow(elevator) && elevator.Orders[floor][datatypes.BT_HallUP] {
			return true
		}

	case config.DIR_STOP:
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
	case config.DIR_UP, config.DIR_STOP:
		return true
	case config.DIR_DOWN:
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
	case config.DIR_DOWN, config.DIR_STOP:
		return true
	case config.DIR_UP:
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
