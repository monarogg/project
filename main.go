package main

import (
	"flag"
	"fmt"
	"project/elevator_control"
	"project/elevator_network"
	"project/elevio"
)

func main() {
	idFlag := flag.String("id", "ElevDefault", "Unique ID for this elevator")
	portFlag := flag.String("port", "15657", "Simulator port")

	flag.Parse()

	myID := *idFlag
	port := *portFlag

	numFloors := 4
	elevio.Init("localhost:"+port, numFloors)

	// Initialize elevator network
	elevatorNetwork := elevator_network.InitElevatorNetwork(myID)

	// Initialize elevator
	elevator := elevator_control.InitializeFSM()
	context := elevator_control.GetElevatorContext(myID)

	// Start broadcasting heartbeat
	elevatorNetwork.StartHeartbeat(&elevator)

	// Listen for elevator states
	elevatorNetwork.ListenForStates()

	// Start detecting missing elevators
	go elevatorNetwork.DetectMissingElevators()

	drv_buttons := make(chan elevio.ButtonEvent)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)
	drv_stop := make(chan bool)
	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)
	go elevio.PollStopButton(drv_stop)

	fmt.Println("Elevator system ready. MyID =", myID)

	for {
		select {
		case a := <-drv_buttons:
			elevator_control.OnRequestButtonPress(&elevator, a.Floor, a.Button, context)

		case a := <-drv_floors:
			elevator_control.OnFloorArrival(&elevator, a, context)

		case a := <-drv_obstr:
			if a {
				elevio.SetMotorDirection(elevio.MD_Stop)
			} else {
				elevio.SetMotorDirection(elevator.Direction)
			}

		case <-drv_stop:
			elevator_control.OnStopButtonPress(&elevator)
		}

		elevator_control.UpdateLights(&elevator)
	}
}
