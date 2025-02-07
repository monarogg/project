package main

import (
	"project/elevio"
)

func RequestsAbove(elevator *Elevator) bool { // skal returnere true/false om det er noen aktive orders i etasjer over
	for f := elevator.CurrentFloor + 1; f < len(elevator.Orders); f++ {
		for _, order := range elevator.Orders[f] {
			if order {
				return true
			}
		}
	}
	return false
}

func RequestsBelow(elevator *Elevator) bool { // skal returnere true/false om det er noen aktive orders i etasjer under
	for f := elevator.CurrentFloor - 1; f >= 0; f-- {
		for _, order := range elevator.Orders[f] {
			if order {
				return true
			}
		}
	}
	return false
}

func RequestsHere(elevator *Elevator) bool {
	for _, order := range elevator.Orders[elevator.CurrentFloor] {
		if order {
			return true
		}
	}
	return false
}

func RequestsHereMatchingDir(elevator *Elevator, dir elevio.MotorDirection) bool {
    for b, active := range elevator.Orders[elevator.CurrentFloor] {
        if !active {
            continue
        }
        // Stopper allltid på cab call
        if elevio.ButtonType(b) == elevio.BT_Cab {
            return true
        }
        // Stopper hvis retningen fra hall matcher nåværende retning 
        if dir == elevio.MD_Up && elevio.ButtonType(b) == elevio.BT_HallUp {
            return true
        }
        if dir == elevio.MD_Down && elevio.ButtonType(b) == elevio.BT_HallDown {
            return true
        }
    }
    return false
}

func AddOrder(elevator *Elevator, floor int, button elevio.ButtonType) {
	elevator.Orders[floor][button] = true
	elevio.SetButtonLamp(button, floor, true)

}

//func ClearRequestsAtFloor(elevator *Elevator) {
//	// iterer over knappetypene (0, 1, 2)
//	for b := 0; b < 3; b++ {
//		// sletter alle bestillinger i den etasjen man er i:
//		elevator.Orders[elevator.CurrentFloor][b] = false
//		elevio.SetButtonLamp(elevio.ButtonType(b), elevator.CurrentFloor, false)
//	}
//}


// Ny ClearRequestsAtFloor som kun  sletter om retningen om den stemmer med kjøreretning, og cab calls
func ClearRequestsAtFloor(elevator *Elevator) {
    floor := elevator.CurrentFloor
    for b := 0; b < 3; b++ {
        if !elevator.Orders[floor][b] {
            continue
        }
        buttonType := elevio.ButtonType(b)
        if buttonType == elevio.BT_Cab {
            // Always clear cab calls
            elevator.Orders[floor][b] = false
        }

		if elevator.Direction == elevio.MD_Up && buttonType == elevio.BT_HallUp {
            elevator.Orders[floor][b] = false
        }

		if elevator.Direction == elevio.MD_Down && buttonType == elevio.BT_HallDown {
            elevator.Orders[floor][b] = false
        } 
		
		if elevator.Direction == elevio.MD_Stop {
            elevator.Orders[floor][b] = false
        }
    }
}

func ClearAllRequests(elevator *Elevator) {
	for f := 0; f < len(elevator.Orders); f++ {
		for b := 0; b < 3; b++ {
			elevator.Orders[f][b] = false
			elevio.SetButtonLamp(elevio.ButtonType(b), f, false)
		}
	}
}

func ChooseDirection(elevator *Elevator) elevio.MotorDirection { //velger retning basert på nåværende retning og bestillinger
	switch elevator.Direction {
	case elevio.MD_Up:
		if RequestsAbove(elevator) {
			return elevio.MD_Up
		}
		if RequestsHere(elevator) || RequestsBelow(elevator) {
			return elevio.MD_Down
		}
	case elevio.MD_Down:
		if RequestsBelow(elevator) {
			return elevio.MD_Down
		}
		if RequestsHere(elevator) || RequestsAbove(elevator) {
			return elevio.MD_Up
		}
	case elevio.MD_Stop: // dersom den står stille prioriterer den bestillinger som er over
		if RequestsAbove(elevator) {
			return elevio.MD_Up
		}
		if RequestsBelow(elevator) {
			return elevio.MD_Down
		}
	}
	return elevio.MD_Stop

}

func ShouldStop(elevator *Elevator) bool {
    switch elevator.Direction {
    case elevio.MD_Up:
        return RequestsHereMatchingDir(elevator, elevio.MD_Up) || !RequestsAbove(elevator)
    case elevio.MD_Down:
        return RequestsHereMatchingDir(elevator, elevio.MD_Down) || !RequestsBelow(elevator)
    case elevio.MD_Stop:
        return RequestsHere(elevator)
    }
    return false
}