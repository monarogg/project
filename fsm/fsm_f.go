package fsm

/* import (
	//"project/config"
	"fmt"
	"project/datatypes"
	"project/elevator_control"
	"project/elevio"
	"project/requests"
	"time"
)

const (
	DOOR_OPEN_DURATION = 3
	MOVEMENT_TIMEOUT   = 4
)

func RunElevFSM(reqChan <-chan [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool, completedReqChan chan<- datatypes.ButtonEvent) {
	floorSensorChan := make(chan int)
	obstructionChan := make(chan bool)

	go elevio.PollFloorSensor(floorSensorChan)
	go elevio.PollObstructionSwitch(obstructionChan)

	elevator := elevator_control.InitElevator()
	elevator_control.UpdateInfoElev(elevator)
	elevator_control.SetElevAvailability(true)

	doorOpenTimer := time.NewTimer(0)
	elevator_control.KillTimer(doorOpenTimer)
	movementTimer := time.NewTimer(0)
	elevator_control.KillTimer(movementTimer)

	for {
		select {
		case elevator.Orders = <-reqChan:
			fmt.Println("FSM: Received new orders:")
			for f := 0; f < datatypes.N_FLOORS; f++ {
				for b := 0; b < datatypes.N_BUTTONS; b++ {
					if elevator.Orders[f][b] {
						fmt.Printf(" - Order at floor %d, button %d\n", f, b)
					}
				}
			}

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

			// Clamp direction AND stop motor if at terminal floor
			if elevator.CurrentFloor == 0 && elevator.Direction == datatypes.DIR_DOWN {
				elevator.Direction = datatypes.DIR_STOP
				elevio.SetMotorDirection(elevio.MD_Stop)
				elevator_control.KillTimer(movementTimer)
				elevator.State = datatypes.DoorOpen
			} else if elevator.CurrentFloor == 3 && elevator.Direction == datatypes.DIR_UP {
				elevator.Direction = datatypes.DIR_STOP
				elevio.SetMotorDirection(elevio.MD_Stop)
				elevator_control.KillTimer(movementTimer)
				elevator.State = datatypes.DoorOpen
			}

			elevator_control.UpdateInfoElev(elevator)

		case isObstructed := <-obstructionChan:
			if isObstructed {
				elevator_control.SetElevAvailability(false)
				elevator_control.KillTimer(doorOpenTimer)
			} else {
				elevator_control.SetElevAvailability(true)
				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
			}

		case <-doorOpenTimer.C:
			if elevator.State != datatypes.DoorOpen {
				break
			}
				elevio.SetDoorOpenLamp(false)

				if !requests.RequestsAbove(elevator) && !requests.RequestsBelow(elevator) && !requests.RequestsHere(elevator) {
					elevator.State = datatypes.Idle
					elevator.Direction = datatypes.DIR_STOP
				} else {
					elevator.Direction, elevator.State = requests.ChooseNewDirAndBeh(elevator)
				}

				switch elevator.State {
				case datatypes.DoorOpen:
					elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
				case datatypes.Moving:
					elevator_control.RestartTimer(movementTimer, MOVEMENT_TIMEOUT)
					elevio.SetMotorDirection(elevator_control.DirConv(elevator.Direction))
				}
				elevator_control.UpdateInfoElev(elevator)

		case <-movementTimer.C:
			elevator_control.SetElevAvailability(false)
		}
	}
}
 */