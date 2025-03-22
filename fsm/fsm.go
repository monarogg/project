package fsm

import (
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

// clearOrders clears all orders for the current floor based on the elevator's direction.
func clearOrders(elevator *datatypes.Elevator, completedReqChan chan<- datatypes.ButtonEvent) {
	floor := elevator.CurrentFloor

	// Clear cab call if present.
	if requests.CanClearCab(*elevator) {
		elevator.Orders[floor][datatypes.BT_CAB] = false
		completedReqChan <- datatypes.ButtonEvent{Floor: floor, Button: datatypes.BT_CAB}
	}
	// Clear hall calls based on current direction.
	switch elevator.Direction {
	case datatypes.DIR_UP:
		if requests.CanClearHallUp(*elevator) {
			elevator.Orders[floor][datatypes.BT_HallUP] = false
			completedReqChan <- datatypes.ButtonEvent{Floor: floor, Button: datatypes.BT_HallUP}
		}
	case datatypes.DIR_DOWN:
		if requests.CanClearHallDown(*elevator) {
			elevator.Orders[floor][datatypes.BT_HallDOWN] = false
			completedReqChan <- datatypes.ButtonEvent{Floor: floor, Button: datatypes.BT_HallDOWN}
		}
	case datatypes.DIR_STOP:
		// For idle, clear one of the hall calls and choose a direction.
		if requests.CanClearHallUp(*elevator) {
			elevator.Orders[floor][datatypes.BT_HallUP] = false
			completedReqChan <- datatypes.ButtonEvent{Floor: floor, Button: datatypes.BT_HallUP}
			elevator.Direction = datatypes.DIR_UP
		} else if requests.CanClearHallDown(*elevator) {
			elevator.Orders[floor][datatypes.BT_HallDOWN] = false
			completedReqChan <- datatypes.ButtonEvent{Floor: floor, Button: datatypes.BT_HallDOWN}
			elevator.Direction = datatypes.DIR_DOWN
		}
	}
}

func RunElevFSM(
	reqChan <-chan [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool,
	completedReqChan chan<- datatypes.ButtonEvent,
) {
	floorSensorChan := make(chan int)
	obstructionChan := make(chan bool)

	go elevio.PollFloorSensor(floorSensorChan)
	go elevio.PollObstructionSwitch(obstructionChan)

	elevator := elevator_control.InitElevator(floorSensorChan)
	elevator_control.UpdateInfoElev(elevator)
	elevator_control.SetElevAvailability(true)

	doorOpenTimer := time.NewTimer(0)
	elevator_control.KillTimer(doorOpenTimer)

	movementTimer := time.NewTimer(0)
	elevator_control.KillTimer(movementTimer)

	for {
		select {
		case elevator.Orders = <-reqChan:
			if elevator.State == datatypes.Moving {
				break
			}
			if elevator.State == datatypes.DoorOpen {
				fmt.Println("1. Restarting doorOpenTimer for", DOOR_OPEN_DURATION, "seconds")

				if elevio.GetObstruction() {
					elevator_control.SetElevAvailability(false)
				} else {
					elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
				}
				break
			}
			elevator.Direction, elevator.State = requests.ChooseNewDirAndBeh(elevator)

			switch elevator.State {
			case datatypes.DoorOpen:
				elevio.SetDoorOpenLamp(true)
				fmt.Println("2. Restarting doorOpenTimer for", DOOR_OPEN_DURATION, "seconds")

				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)

			case datatypes.Moving:
				fmt.Println("Restarting movementTimer for", MOVEMENT_TIMEOUT, "seconds")

				elevator_control.RestartTimer(movementTimer, MOVEMENT_TIMEOUT)
				elevio.SetMotorDirection(elevator_control.DirConv(elevator.Direction))
			}

			elevator_control.UpdateInfoElev(elevator)
			fmt.Println("FSM: Choosing New Direction. Orders:", elevator.Orders)

		case elevator.CurrentFloor = <-floorSensorChan:
			fmt.Println("Floor sensor update:", elevator.CurrentFloor, "Orders:", elevator.Orders[elevator.CurrentFloor])
			if elevator.State == datatypes.Moving {
				fmt.Println("4. Restarting doorOpenTimer for", DOOR_OPEN_DURATION, "seconds")

				elevator_control.RestartTimer(movementTimer, MOVEMENT_TIMEOUT)
				elevator_control.SetElevAvailability(true)
				elevio.SetFloorIndicator(elevator.CurrentFloor)
				if requests.ShouldStop(elevator) {
					fmt.Println("ShouldStop returned true at floor", elevator.CurrentFloor)
					elevator_control.KillTimer(movementTimer)
					elevio.SetMotorDirection(elevio.MD_Stop)
					elevio.SetDoorOpenLamp(true)
					elevator.State = datatypes.DoorOpen
					fmt.Println("5. Restarting doorOpenTimer for", DOOR_OPEN_DURATION, "seconds")

					elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
				}
			}

		case isObstructed := <-obstructionChan:
			if isObstructed {
				elevator_control.SetElevAvailability(false)
				elevator_control.KillTimer(doorOpenTimer)
			} else {
				elevator_control.SetElevAvailability(true)
				fmt.Println("6. Restarting doorOpenTimer for", DOOR_OPEN_DURATION, "seconds")

				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
			}

		case <-doorOpenTimer.C:
			if elevator.State != datatypes.DoorOpen {
				break
			}
			fmt.Println("DOORTIMER: Fired at floor", elevator.CurrentFloor, "with orders:", elevator.Orders[elevator.CurrentFloor])

			// Force elevator direction to match the active hall call.
			if elevator.Orders[elevator.CurrentFloor][datatypes.BT_HallUP] {
				elevator.Direction = datatypes.DIR_UP
			} else if elevator.Orders[elevator.CurrentFloor][datatypes.BT_HallDOWN] {
				elevator.Direction = datatypes.DIR_DOWN
			}

			// Clear orders at this floor.
			clearOrders(&elevator, completedReqChan)

			// Recalculate state. If no orders remain, force Moving.
			if !requests.RequestsHere(elevator) {
				fmt.Println("No orders remain at floor", elevator.CurrentFloor, "; forcing transition to Moving")
				elevator.State = datatypes.Moving
			} else {
				elevator.Direction, elevator.State = requests.ChooseNewDirAndBeh(elevator)
				fmt.Println("Recomputed state:", elevator.State, "direction:", elevator.Direction)
			}

			// Update timers and outputs based on new state.
			if elevator.State == datatypes.Moving || elevator.State == datatypes.Idle {
				// Kill the door timer so it doesn't restart.
				elevator_control.KillTimer(doorOpenTimer)
				elevio.SetDoorOpenLamp(false)
				if elevator.State == datatypes.Moving {
					elevator_control.RestartTimer(movementTimer, MOVEMENT_TIMEOUT)
					elevio.SetMotorDirection(elevator_control.DirConv(elevator.Direction))
				}
				fmt.Println("Transitioned to", elevator.State, "- door closed.")
			} else {
				// If still DoorOpen, restart the timer.
				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
				elevio.SetDoorOpenLamp(true)
				fmt.Println("Remaining in DoorOpen state; door stays open.")
			}

			elevator_control.UpdateInfoElev(elevator)
			fmt.Println("FSM: Current Orders:", elevator.Orders)
		}
	}
}
