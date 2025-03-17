package fsm

//TODO: Sørge for riktig bruk av funksjoner.

// inneholder logikken for å kontrollere en heis

import (
	"project/datatypes"
	"project/elevator_control"
	"project/elevio"
	"project/requests"
	"time"
)

const DOOR_OPEN_DURATION = 3
const MOVEMENT_TIMEOUT = 4

func RunElevFSM(reqChan <-chan [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool,
	completedReqChan chan<- datatypes.ButtonEvent) {

	floorSensorChan := make(chan int)  // tar inn etasjen
	obstructionChan := make(chan bool) // tar inn hvorvidt obstruction eller ikke

	go elevio.PollObstructionSwitch(obstructionChan)
	go elevio.PollFloorSensor(floorSensorChan)

	elevator := elevator_control.InitElevator(floorSensorChan)
	elevator_control.UpdateInfoElev(elevator)  // sørger for at systemet vet hvor heisen er
	elevator_control.SetElevAvailability(true) // ny heis - er available

	// opprette og deaktivere timere, for at de skal være forberedt på å brukes:
	doorOpenTimer := time.NewTimer(0)
	elevator_control.KillTimer(doorOpenTimer)
	movementTimer := time.NewTimer(0)
	elevator_control.KillTimer(movementTimer)

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
				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
			case datatypes.Moving:
				elevator_control.RestartTimer(movementTimer, MOVEMENT_TIMEOUT)
				elevio.SetMotorDirection(elevator_control.DirConv(elevator.Direction))
			}
			elevator_control.UpdateInfoElev(elevator) // oppdaterer info for å sørge for alle endringer gjort i switch case blir kommunisert videre
		case elevator.CurrentFloor = <-floorSensorChan:
			if elevator.State != datatypes.Moving {
				break // hvis state ikke er Moving - skal ikke sjekke floor sensor
			}
			elevator_control.RestartTimer(movementTimer, MOVEMENT_TIMEOUT)
			elevator_control.SetElevAvailability(true) // heis er aktiv - kan motta nye bestillinger

			elevio.SetFloorIndicator(elevator.CurrentFloor)
			if requests.ShouldStop(elevator) {
				elevator_control.KillTimer(movementTimer)
				elevio.SetMotorDirection(elevio.MotorDirection(datatypes.DIR_STOP))

				if requests.CanClearHallUp(elevator) {
					elevator.Direction = datatypes.DIR_UP
				} else if requests.CanClearHallDown(elevator) {
					elevator.Direction = datatypes.DIR_DOWN
				} else if requests.CanClearCab(elevator) {
					// trenger ikke oppdatere direction
				} else {
					elevator.State = datatypes.Idle
					elevator_control.UpdateInfoElev(elevator)
					break
				}

				elevio.SetDoorOpenLamp(true)
				elevator.State = datatypes.DoorOpen
			}
		case isObstructed := <-obstructionChan:
			if isObstructed {
				elevator_control.SetElevAvailability(false) // fordi obstructed
				elevator_control.KillTimer(doorOpenTimer)
			} else {
				elevator_control.SetElevAvailability(true)
				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
			}
		case <-doorOpenTimer.C:
			if elevator.State != datatypes.DoorOpen {
				break // for å sikre at door open timer case dersom door open
			}
			if requests.CanClearCab(elevator) {
				elevator.Orders[elevator.CurrentFloor][datatypes.BT_CAB] = false
				completedReqChan <- datatypes.ButtonEvent{Floor: elevator.CurrentFloor, Button: datatypes.BT_CAB}
			}

			elevator.Direction, elevator.State = requests.ChooseNewDirAndBeh(elevator)

			switch elevator.State {
			case datatypes.DoorOpen:
				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
			case datatypes.Idle:
				elevio.SetDoorOpenLamp(false)
			case datatypes.Moving:
				elevio.SetDoorOpenLamp(false)
				elevator_control.RestartTimer(movementTimer, MOVEMENT_TIMEOUT)
				elevio.SetMotorDirection(elevator_control.DirConv(elevator.Direction))
			}
			elevator_control.UpdateInfoElev(elevator)

		case <-movementTimer.C:
			elevator_control.SetElevAvailability(false)
		}
	}
}
