package main

import (
	"fmt"
	"project/elevio"
	"project/network/bcast"
	"time"
)

func main() {

	numFloors := 4

	elevio.Init("localhost:15657", numFloors)

	drv_buttons := make(chan elevio.ButtonEvent)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)
	drv_stop := make(chan bool)

	//channels for broadcasting elevator state
	txElevatorState := make(chan NetElevator)
    rxElevatorState := make(chan NetElevator)

	// start the transmitter/receiver
	go bcast.Transmitter(17657, txElevatorState)
    go bcast.Receiver(17658, rxElevatorState)

	elevator := initializeFSM()

	go func()  {
		myID := "Elev1"

		for {
			currentState := NetElevator{
				ID:				myID,
				CurrentFloor:	elevator.CurrentFloor,
				Direction:    	elevator.Direction,
                State:        	elevator.State,
                Orders:       	elevator.Orders,
                StopActive:   	elevator.StopActive,
			}
			txElevatorState <- currentState
			time.Sleep(100 * time.Millisecond)
		}
	}()


	knownElevators := make(map[string]NetElevator)

	go func() {
		for {
			tempState := <-rxElevatorState
			knownElevators[tempState.ID] = tempState
			//fmt.Println("Received state from:", tempState.ID, "Floor:", tempState.CurrentFloor)
		}
	}()	

	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)
	go elevio.PollStopButton(drv_stop)

	fmt.Println("Elevator system ready")

	for {
		select {
		case a := <-drv_buttons: // a tilsvarer knappetrykket
			// håndtere trykk på knapper
			OnRequestButtonPress(&elevator, a.Floor, a.Button)

		case a := <-drv_floors: // a blir etasjen man ankommer
			OnFloorArrival(&elevator, a)

		case a := <-drv_obstr: // håndterer dersom obstruction blir aktivert
			if a {

				elevio.SetMotorDirection(elevio.MD_Stop)
			} else {
				elevio.SetMotorDirection(elevator.Direction)
			}

		case <-drv_stop: // håndterer dersom stop blir trykket
			OnStopButtonPress(&elevator)

		}

		UpdateLights(&elevator)
	}
}