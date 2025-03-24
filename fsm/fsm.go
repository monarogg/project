/* package fsm

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
		if requests.CanClearCab(*elevator) {
			fmt.Println("Clearing CAB call at floor", floor)
			elevator.Orders[floor][datatypes.BT_CAB] = false
			completedReqChan <- datatypes.ButtonEvent{Floor: floor, Button: datatypes.BT_CAB}
		}
		if requests.CanClearHallUp(*elevator) {
			elevator.Orders[floor][datatypes.BT_HallUP] = false
			completedReqChan <- datatypes.ButtonEvent{Floor: floor, Button: datatypes.BT_HallUP}
		} else if elevator.Orders[floor][datatypes.BT_HallDOWN] {
			oppositeDirCall = true
		}

	case datatypes.DIR_DOWN:
		if requests.CanClearCab(*elevator) {
			elevator.Orders[floor][datatypes.BT_CAB] = false
			completedReqChan <- datatypes.ButtonEvent{Floor: floor, Button: datatypes.BT_CAB}
		}
		if requests.CanClearHallDown(*elevator) {
			elevator.Orders[floor][datatypes.BT_HallDOWN] = false
			completedReqChan <- datatypes.ButtonEvent{Floor: floor, Button: datatypes.BT_HallDOWN}
		} else if elevator.Orders[floor][datatypes.BT_HallUP] {
			oppositeDirCall = true
		}

	case datatypes.DIR_STOP:
		if requests.CanClearCab(*elevator) {
			elevator.Orders[floor][datatypes.BT_CAB] = false
			completedReqChan <- datatypes.ButtonEvent{Floor: floor, Button: datatypes.BT_CAB}
		}
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
	reqChan chan [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool,
	completedReqChan chan<- datatypes.ButtonEvent,
) {
	floorSensorChan := make(chan int)
	obstructionChan := make(chan bool)

	go elevio.PollFloorSensor(floorSensorChan)
	go elevio.PollObstructionSwitch(obstructionChan)

	// Clamp direction if at end floors

	elevator := elevator_control.InitElevator()
	elevator_control.UpdateInfoElev(elevator)
	elevator_control.SetElevAvailability(true)

	doorOpenTimer := time.NewTimer(0)
	elevator_control.KillTimer(doorOpenTimer)

	movementTimer := time.NewTimer(0)
	elevator_control.KillTimer(movementTimer)

	for {
		select {
		case newOrders := <-reqChan:
			fmt.Println("FSM222: Received orders from reqChan:", newOrders)

			if elevator.State == datatypes.DoorOpen {
				fmt.Println("Already in DoorOpen: Merging new orders...")
				elevator.Orders = requests.MergeOrders(elevator.Orders, newOrders)
				break
			}

			// Set and evaluate new orders
			elevator.Orders = newOrders
			elevator.Direction, elevator.State = requests.ChooseNewDirAndBeh(elevator)
			fmt.Println("111111. New state after receiving orders:", elevator.State, "Direction:", elevator.Direction)

			switch elevator.State {
			case datatypes.Moving:
				fmt.Println("Setting motor: direction =", elevator.Direction)
				elevio.SetMotorDirection(elevator_control.DirConv(elevator.Direction))
				elevio.SetDoorOpenLamp(false)
				elevator_control.RestartTimer(movementTimer, MOVEMENT_TIMEOUT)

			case datatypes.DoorOpen:
				fmt.Println("Entering DoorOpen. Starting doorOpenTimer")
				elevio.SetDoorOpenLamp(true)
				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)

			case datatypes.Idle:
				fmt.Println("No movement needed. Remaining Idle.")
				elevio.SetDoorOpenLamp(false)
			}

			elevator_control.UpdateInfoElev(elevator)

			fmt.Println("FSM: Choosing New Direction. Orders:", elevator.Orders)

		case elevator.CurrentFloor = <-floorSensorChan:
			fmt.Println("Floor sensor update:", elevator.CurrentFloor, "Orders:", elevator.Orders[elevator.CurrentFloor])
			elevio.SetFloorIndicator(elevator.CurrentFloor)

			if elevator.State == datatypes.Moving {
				fmt.Println("Elevator is moving. Checking if it should stop...")
				elevator_control.RestartTimer(movementTimer, MOVEMENT_TIMEOUT)

				elevator_control.SetElevAvailability(true)

				fmt.Println("Checking ShouldStop: floor =", elevator.CurrentFloor, "| dir =", elevator.Direction, "| Orders:", elevator.Orders[elevator.CurrentFloor])

				if requests.ShouldStop(elevator) {
					fmt.Println("ShouldStop returned true at floor", elevator.CurrentFloor)

					elevio.SetMotorDirection(elevio.MD_Stop)
					elevator_control.KillTimer(movementTimer)

					elevator.State = datatypes.DoorOpen
					elevator.Direction = datatypes.DIR_STOP // Optional: to make it explicit

					elevio.SetDoorOpenLamp(true)
					elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
					fmt.Println("Door open timer restarted")

					if !requests.RequestsAbove(elevator) && !requests.RequestsBelow(elevator) && !requests.RequestsHere(elevator) {
						fmt.Println("No more requests. Transitioning to Idle after DoorOpen.")
					}
				}
			}

			// Clamp direction AFTER handling stop logic
			if elevator.CurrentFloor == 0 && elevator.Direction == datatypes.DIR_DOWN {
				elevator.Direction = datatypes.DIR_STOP
			} else if elevator.CurrentFloor == config.N_FLOORS-1 && elevator.Direction == datatypes.DIR_UP {
				elevator.Direction = datatypes.DIR_STOP
			}

			elevator_control.UpdateInfoElev(elevator)

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
				fmt.Println("Still has request at this floor:", elevator.CurrentFloor)
				fmt.Println("Order state:", elevator.Orders[elevator.CurrentFloor])
				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
				break
			}

			elevator.Direction, elevator.State = requests.ChooseNewDirAndBeh(elevator)

			if !requests.RequestsAbove(elevator) && !requests.RequestsBelow(elevator) && !requests.RequestsHere(elevator) {
				elevator.State = datatypes.Idle
				elevator.Direction = datatypes.DIR_STOP
			}
			
			fmt.Println("Post-doorTimer ChooseNewDirAndBeh: Dir =", elevator.Direction, ", State =", elevator.State)
			fmt.Println("Orders now:", elevator.Orders)
			fmt.Println("== Entering state switch. State =", elevator.State)

			switch elevator.State {
			case datatypes.Moving:
				elevio.SetDoorOpenLamp(false)
				elevator_control.KillTimer(doorOpenTimer)
				elevio.SetMotorDirection(elevator_control.DirConv(elevator.Direction))
				elevator_control.RestartTimer(movementTimer, MOVEMENT_TIMEOUT)
				fmt.Println("Door closed. Now moving:", elevator.Direction)
				fmt.Println("Switch case: Moving")
				
			case datatypes.Idle:
				elevio.SetDoorOpenLamp(false)
				elevator_control.KillTimer(doorOpenTimer)
				fmt.Println("Door closed. Elevator idle.")
				fmt.Println("Switch case: Idle")


			case datatypes.DoorOpen:
				elevio.SetDoorOpenLamp(true)
				elevator_control.RestartTimer(doorOpenTimer, DOOR_OPEN_DURATION)
				fmt.Println("Remaining in DoorOpen")
				fmt.Println("Switch case: DoorOpen")

			}

			elevator_control.UpdateInfoElev(elevator)
			fmt.Println("Updated State:", elevator.State, "Direction:", elevator.Direction)

		}
	}
}
 */

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
	obstructionChan := make(chan bool) // tar inn hvorvidt obstruction eller ikke

	go elevio.PollFloorSensor(floorSensorChan)
	go elevio.PollObstructionSwitch(obstructionChan)

	elevator := elevator_control.InitElevator()
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