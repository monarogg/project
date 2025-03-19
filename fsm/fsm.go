package fsm

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

	floorSensorChan := make(chan int)
	go elevio.PollFloorSensor(floorSensorChan)

	elevator := elevator_control.InitElevator(floorSensorChan)
	elevator_control.UpdateInfoElev(elevator)
	elevator_control.SetElevAvailability(true)

	// Initialize timers
	doorOpenTimer := time.NewTimer(0)
	elevator_control.KillTimer(doorOpenTimer)
	movementTimer := time.NewTimer(0)
	elevator_control.KillTimer(movementTimer)

	for {
		select {
		case elevator.Orders = <-reqChan:
			if elevator.State != datatypes.Idle {
				break
			}
			elevator.Direction, elevator.State = requests.ChooseNewDirAndBeh(elevator)
			switch elevator.State {
			case datatypes.DoorOpen:
				elevio.SetDoorOpenLamp(true)
				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
			case datatypes.Moving:
				elevator_control.RestartTimer(movementTimer, MOVEMENT_TIMEOUT)
				elevio.SetMotorDirection(elevator_control.DirConv(elevator.Direction))
			}
			elevator_control.UpdateInfoElev(elevator)

		case elevator.CurrentFloor = <-floorSensorChan:
			if elevator.State != datatypes.Moving {
				break
			}
			elevator_control.RestartTimer(movementTimer, MOVEMENT_TIMEOUT)
			elevator_control.SetElevAvailability(true)
			elevio.SetFloorIndicator(elevator.CurrentFloor)

			if requests.ShouldStop(elevator) {
				elevator_control.KillTimer(movementTimer)
				elevio.SetMotorDirection(elevio.MotorDirection(datatypes.DIR_STOP))

				// Clear requests at this floor
				if requests.CanClearHallUp(elevator) {
					elevator.Orders[elevator.CurrentFloor][datatypes.BT_HallUP] = false
					completedReqChan <- datatypes.ButtonEvent{Floor: elevator.CurrentFloor, Button: datatypes.BT_HallUP}
				}
				if requests.CanClearHallDown(elevator) {
					elevator.Orders[elevator.CurrentFloor][datatypes.BT_HallDOWN] = false
					completedReqChan <- datatypes.ButtonEvent{Floor: elevator.CurrentFloor, Button: datatypes.BT_HallDOWN}
				}
				if requests.CanClearCab(elevator) {
					elevator.Orders[elevator.CurrentFloor][datatypes.BT_CAB] = false
					completedReqChan <- datatypes.ButtonEvent{Floor: elevator.CurrentFloor, Button: datatypes.BT_CAB}
				}

				elevio.SetDoorOpenLamp(true)
				elevator.State = datatypes.DoorOpen
				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
			}

		case <-doorOpenTimer.C:
			if elevator.State != datatypes.DoorOpen {
				break
			}
		
			cleared := false
			for button := 0; button < datatypes.N_BUTTONS; button++ {
				if elevator.Orders[elevator.CurrentFloor][button] {
					elevator.Orders[elevator.CurrentFloor][button] = false
					completedReqChan <- datatypes.ButtonEvent{Floor: elevator.CurrentFloor, Button: datatypes.ButtonType(button)}
					cleared = true
				}
			}
		
			if cleared {
				elevio.SetDoorOpenLamp(false)
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
