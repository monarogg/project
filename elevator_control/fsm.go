package elevator_control

// inneholder logikken for å kontrollere en heis

import (
	"project/datatypes"
	"project/elevio"
	"project/requests"
	"time"
)

const DOOR_TIMEOUT_SEC = 3
const MOBILITY_TIMOEUT_SEC = 4

// ENDRE NAVN!

func RunElevFSM(reqChan <-chan [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool,
	completedReqChan <-chan datatypes.ButtonEvent) {

	floorSensorChan := make(chan int)  // tar inn etasjen
	obstructionChan := make(chan bool) // tar inn hvorvidt obstruction eller ikke

	go elevio.PollObstructionSwitch(obstructionChan)
	go elevio.PollFloorSensor(floorSensorChan)

	elevator := initElevator(floorSensorChan)
	updateInfoElev(elevator)  // sørger for at systemet vet hvor heisen er
	setElevAvailability(true) // ny heis - er available

	// opprette og deaktivere timere, for at de skal være forberedt på å brukes:
	doorOpenTimer := time.NewTimer(0)
	killTimer(doorOpenTimer)
	movementTimer := time.NewTimer(0)
	killTimer(movementTimer)

	// en hovedloop - uendelig for-løkke som inneholder select
	for {
		select {
		case elevator.Orders = <-reqChan: // heisen mottar oppdatert liste av aktive requests
			if elevator.State != datatypes.Idle {
				break // hvis state ikke er Idle - skal ikke finne new Direction og State
			}
			elevator.Direction, elevator.State = requests.ChooseNewDirAndBeh(elevator)
			// switch case på State for enten Moving eller DoorOpen:
			switch elevator.State {
			case datatypes.DoorOpen:
				elevio.SetDoorOpenLamp(true)
				restartTimer(doorOpenTimer, DOOR_TIMEOUT_SEC)
			case datatypes.Moving:
				restartTimer(movementTimer, MOBILITY_TIMOEUT_SEC)
				elevio.SetMotorDirection(dirConv(elevator.Direction))
			}
			updateInfoElev(elevator) // oppdaterer info for å sørge for alle endringer gjort i switch case blir kommunisert videre
		case elevator.CurrentFloor = <-floorSensorChan:
			if elevator.State != datatypes.Moving {
				break // hvis state ikke er Moving - skal ikke sjekke floor sensor
			}
			restartTimer(movementTimer, MOBILITY_TIMOEUT_SEC)
			setElevAvailability(true) // heis er aktiv - kan motta nye bestillinger

			elevio.SetFloorIndicator(elevator.CurrentFloor)
			if requests.ShouldStop(elevator) {
				killTimer(movementTimer)
				elevio.SetMotorDirection(elevio.MotorDirection(datatypes.DIR_STOP))

				if requests.CanClearHallUp(elevator) {
					elevator.Direction = datatypes.DIR_UP
				} else if requests.CanClearHallDown(elevator) {
					elevator.Direction = datatypes.DIR_DOWN
				} else if requests.CanClearCab(elevator) {
					// trenger ikke oppdatere direction
				} else {
					elevator.State = datatypes.Idle
					updateInfoElev(elevator)
					break
				}
			}

		}
	}
}
