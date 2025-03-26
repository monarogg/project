package fsm

import (
	"fmt"
	"project/config"
	"project/datatypes"
	"project/elevator_control"
	"project/elevio"
	"project/requests"
	"time"
)

func RunElevFSM(reqChan <-chan [config.N_FLOORS][config.N_BUTTONS]bool, completedReqChan chan<- datatypes.ButtonEvent) {
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
			for f := 0; f < config.N_FLOORS; f++ {
				for b := 0; b < config.N_BUTTONS; b++ {
					if elevator.Orders[f][b] {
						fmt.Printf(" - Order at floor %d, button %d\n", f, b)
					}
				}
			}

			if elevator.State != config.Idle {
				break
			}
			elevator.Direction, elevator.State = requests.ChooseNewDirAndBeh(elevator)
			fmt.Printf("FSM: Current State: %v | Floor: %d | Direction: %v\n", elevator.State, elevator.CurrentFloor, elevator.Direction)

			switch elevator.State {
			case config.DoorOpen:
				elevio.SetDoorOpenLamp(true)
				elevator_control.RestartTimer(doorOpenTimer, config.DOOR_OPEN_DURATION)
			case config.Moving:
				elevator_control.RestartTimer(movementTimer, config.MOVEMENT_TIMEOUT)
				elevio.SetMotorDirection(elevator_control.DirConv(elevator.Direction))
			}
			elevator_control.UpdateInfoElev(elevator)

		case newFloor := <-floorSensorChan:
			fmt.Println("FSM: Floor sensor triggered, floor =", newFloor)
			elevator.CurrentFloor = newFloor
			elevio.SetFloorIndicator(newFloor)

			if elevator.State != config.Moving {
				break
			}

			elevator_control.RestartTimer(movementTimer, config.MOVEMENT_TIMEOUT)
			elevator_control.SetElevAvailability(true)

			if requests.ShouldStop(elevator) {
				elevator_control.KillTimer(movementTimer)
				elevio.SetMotorDirection(elevio.MotorDirection(config.DIR_STOP))

				if requests.CanClearHallUp(elevator) {
					elevator.Orders[newFloor][config.BT_HallUP] = false
					completedReqChan <- datatypes.ButtonEvent{Floor: newFloor, Button: config.BT_HallUP}
				}
				if requests.CanClearHallDown(elevator) {
					elevator.Orders[newFloor][config.BT_HallDOWN] = false
					completedReqChan <- datatypes.ButtonEvent{Floor: newFloor, Button: config.BT_HallDOWN}
				}
				if requests.CanClearCab(elevator) {
					elevator.Orders[newFloor][config.BT_CAB] = false
					completedReqChan <- datatypes.ButtonEvent{Floor: newFloor, Button: config.BT_CAB}
				}

				elevio.SetDoorOpenLamp(true)
				elevator.State = config.DoorOpen
				elevator_control.RestartTimer(doorOpenTimer, config.DOOR_OPEN_DURATION)
			}

			elevator_control.UpdateInfoElev(elevator)

		case isObstructed := <-obstructionChan:
			if isObstructed {
				elevator_control.SetElevAvailability(false)
				elevator_control.KillTimer(doorOpenTimer)
			} else {
				elevator_control.SetElevAvailability(true)
				elevator_control.RestartTimer(doorOpenTimer, config.DOOR_OPEN_DURATION)
			}

		case <-doorOpenTimer.C:
			if elevator.State != config.DoorOpen {
				break
			}

			cleared := false
			for button := 0; button < config.N_BUTTONS; button++ {
				if elevator.Orders[elevator.CurrentFloor][button] {
					elevator.Orders[elevator.CurrentFloor][button] = false
					completedReqChan <- datatypes.ButtonEvent{Floor: elevator.CurrentFloor, Button: config.ButtonType(button)}
					cleared = true
				}
			}

			if cleared {
				elevio.SetDoorOpenLamp(false)

				elevio.SetDoorOpenLamp(false)

				if !requests.RequestsAbove(elevator) && !requests.RequestsBelow(elevator) && !requests.RequestsHere(elevator) {
					elevator.State = config.Idle
					elevator.Direction = config.DIR_STOP
				} else {
					elevator.Direction, elevator.State = requests.ChooseNewDirAndBeh(elevator)
					fmt.Printf("FSM: Current State: %v | Floor: %d | Direction: %v\n", elevator.State, elevator.CurrentFloor, elevator.Direction)

				}

				switch elevator.State {
				case config.DoorOpen:
					elevator_control.RestartTimer(doorOpenTimer, config.DOOR_OPEN_DURATION)
				case config.Moving:
					elevator_control.RestartTimer(movementTimer, config.MOVEMENT_TIMEOUT)
					elevio.SetMotorDirection(elevator_control.DirConv(elevator.Direction))
				}
			}

			elevator.Direction, elevator.State = requests.ChooseNewDirAndBeh(elevator)
			fmt.Printf("FSM: Current State: %v | Floor: %d | Direction: %v\n", elevator.State, elevator.CurrentFloor, elevator.Direction)

			switch elevator.State {
			case config.DoorOpen:
				elevator_control.RestartTimer(doorOpenTimer, config.DOOR_OPEN_DURATION)
			case config.Idle:
				elevio.SetDoorOpenLamp(false)
			case config.Moving:
				elevio.SetDoorOpenLamp(false)
				elevator_control.RestartTimer(movementTimer, config.MOVEMENT_TIMEOUT)
				elevio.SetMotorDirection(elevator_control.DirConv(elevator.Direction))
			}
			elevator_control.UpdateInfoElev(elevator)

		case <-movementTimer.C:
			elevator_control.SetElevAvailability(false)
		}
	}
}
