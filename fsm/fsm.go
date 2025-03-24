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

const (
	DOOR_OPEN_DURATION = 3
	MOVEMENT_TIMEOUT   = 4
)

// clearOrders clears all orders for the current floor based on the elevator's direction.
func clearOrders(elevator *datatypes.Elevator, completedReqChan chan<- datatypes.ButtonEvent) bool {
	floor := elevator.CurrentFloor
	oppositeDirCall := false

	if requests.CanClearCab(*elevator) {
		elevator.Orders[floor][datatypes.BT_CAB] = false
		completedReqChan <- datatypes.ButtonEvent{Floor: floor, Button: datatypes.BT_CAB}
	}

	switch elevator.Direction {
	case datatypes.DIR_UP:
		if requests.CanClearHallUp(*elevator) {
			elevator.Orders[floor][datatypes.BT_HallUP] = false
			completedReqChan <- datatypes.ButtonEvent{Floor: floor, Button: datatypes.BT_HallUP}
		} else if elevator.Orders[floor][datatypes.BT_HallDOWN] {
			// leftover down call => set oppositeDirCall
			oppositeDirCall = true
		}

	case datatypes.DIR_DOWN:
		if requests.CanClearHallDown(*elevator) {
			elevator.Orders[floor][datatypes.BT_HallDOWN] = false
			completedReqChan <- datatypes.ButtonEvent{Floor: floor, Button: datatypes.BT_HallDOWN}
		} else if elevator.Orders[floor][datatypes.BT_HallUP] {
			oppositeDirCall = true
		}

	case datatypes.DIR_STOP:
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
	return oppositeDirCall
}

func RunElevFSM(
	reqChan <-chan [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool,
	completedReqChan chan<- datatypes.ButtonEvent,
) {
	floorSensorChan := make(chan int)
	obstructionChan := make(chan bool)

	go elevio.PollFloorSensor(floorSensorChan)
	go elevio.PollObstructionSwitch(obstructionChan)

	// Clamp direction if at end floors

	elevator := elevator_control.InitElevator(floorSensorChan)
	elevator_control.UpdateInfoElev(elevator)
	elevator_control.SetElevAvailability(true)

	doorOpenTimer := time.NewTimer(0)
	elevator_control.KillTimer(doorOpenTimer)

	movementTimer := time.NewTimer(0)
	elevator_control.KillTimer(movementTimer)

	for {
		select {
		case newOrders := <-reqChan:
			// 1) If we’re DoorOpen, just merge:
			if elevator.State == datatypes.DoorOpen {
				fmt.Println("Already in DoorOpen: Merging new orders...")
				elevator.Orders = requests.MergeOrders(elevator.Orders, newOrders)
				// Don’t reset the door timer, don’t re-run ChooseNewDirAndBeh
				break
			}

			// 2) If we’re Idle, or even if we’re idle, we
			// set elevator.Orders and call ChooseNewDirAndBeh:
			elevator.Orders = newOrders

			// Even if you're Idle, you want to re-check direction:
			elevator.Direction, elevator.State = requests.ChooseNewDirAndBeh(elevator)

			switch elevator.State {
			case datatypes.Moving:
				elevio.SetDoorOpenLamp(false)
				elevator_control.RestartTimer(movementTimer, MOVEMENT_TIMEOUT)
				elevio.SetMotorDirection(elevator_control.DirConv(elevator.Direction))
			case datatypes.DoorOpen:
				elevio.SetDoorOpenLamp(true)
				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
				fmt.Println("Debug 1: Starting doorOpenTimer for 3 seconds")
			case datatypes.Idle:
				elevio.SetDoorOpenLamp(false)
			}

			elevator_control.UpdateInfoElev(elevator)
			fmt.Println("FSM: Choosing New Direction. Orders:", elevator.Orders)
		case elevator.CurrentFloor = <-floorSensorChan:
			elevator_control.UpdateInfoElev(elevator)
			fmt.Println("Floor sensor update:", elevator.CurrentFloor, "Orders:", elevator.Orders[elevator.CurrentFloor])

			elevio.SetFloorIndicator(elevator.CurrentFloor)

			// Clamp at ends
			if elevator.CurrentFloor == 0 && elevator.Direction == datatypes.DIR_DOWN {
				elevator.Direction = datatypes.DIR_STOP
			} else if elevator.CurrentFloor == config.N_FLOORS-1 && elevator.Direction == datatypes.DIR_UP {
				elevator.Direction = datatypes.DIR_STOP
			}

			if elevator.State == datatypes.Moving {
				elevator_control.RestartTimer(movementTimer, MOVEMENT_TIMEOUT)
				fmt.Println("Restarted movementTimer")

				elevator_control.SetElevAvailability(true)

				if requests.ShouldStop(elevator) {
					fmt.Println("ShouldStop returned true at floor", elevator.CurrentFloor)
					elevator_control.KillTimer(movementTimer)
					elevio.SetMotorDirection(elevio.MD_Stop)
					elevio.SetDoorOpenLamp(true)

					elevator.State = datatypes.DoorOpen
					elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
					fmt.Println("Door open timer restarted")

					if !requests.RequestsAbove(elevator) && !requests.RequestsBelow(elevator) && !requests.RequestsHere(elevator) {
						fmt.Println("No more requests. Transitioning to Idle after DoorOpen.")
					}
				}
			}

		case isObstructed := <-obstructionChan:
			if isObstructed {
				elevator_control.SetElevAvailability(false)
				elevator_control.KillTimer(doorOpenTimer)
			} else {
				elevator_control.SetElevAvailability(true)
				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
				fmt.Println("Debug 3: Starting doorOpenTimer for 3 seconds")

			}

		case <-doorOpenTimer.C:
			if elevator.State != datatypes.DoorOpen {
				break
			}

			fmt.Println("DOORTIMER: Fired at floor", elevator.CurrentFloor)

			oppDirCall := clearOrders(&elevator, completedReqChan)

			// 1) If leftover opposite call was detected:
			if oppDirCall {
				// Example: we arrived going UP, but there's still a DOWN request at this floor
				// We want to clear that leftover and "announce" direction change
				// => remain in DoorOpen for 3 more seconds
				fmt.Println("OppDirCall: changing direction, staying in DoorOpen")

				// Force direction to the opposite
				if elevator.Direction == datatypes.DIR_UP {
					elevator.Direction = datatypes.DIR_DOWN
				} else if elevator.Direction == datatypes.DIR_DOWN {
					elevator.Direction = datatypes.DIR_UP
				}
				elevator.State = datatypes.DoorOpen

				// Re-start the door timer
				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
				elevio.SetDoorOpenLamp(true)

				// Skip normal "RequestsHere" check so we don't close door yet
				break
			}

			// 2) Check if more requests remain at this floor
			if requests.RequestsHere(elevator) {
				fmt.Println("Staying in DoorOpen: more requests at this floor")
				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
				break
			}

			elevator.Direction, elevator.State = requests.ChooseNewDirAndBeh(elevator)
			switch elevator.State {
			case datatypes.Moving:
				elevio.SetDoorOpenLamp(false)
				elevator_control.KillTimer(doorOpenTimer)
				elevio.SetMotorDirection(elevator_control.DirConv(elevator.Direction))
				elevator_control.RestartTimer(movementTimer, MOVEMENT_TIMEOUT)
				fmt.Println("Door closed. Now moving:", elevator.Direction)

			case datatypes.Idle:
				elevio.SetDoorOpenLamp(false)
				elevator_control.KillTimer(doorOpenTimer)
				fmt.Println("Door closed. Elevator idle.")

			case datatypes.DoorOpen:
				elevio.SetDoorOpenLamp(true)
				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
				fmt.Println("Remaining in DoorOpen")
			}

			elevator_control.UpdateInfoElev(elevator)
			fmt.Println("Updated State:", elevator.State, "Direction:", elevator.Direction)

		}
	}
}
