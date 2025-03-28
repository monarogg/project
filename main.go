package main

import (
	"flag"
	"fmt"
	"project/datatypes"
	"project/elevio"
	"project/fsm"
	"project/requests"
)

func main() {

	idFlag := flag.String("id", "", "Unique ID for this elevator")
	portFlag := flag.String("port", "15657", "Simulator port")
	flag.Parse()

	if *idFlag == "" {
		fmt.Println("Error: -id must be provided")
		return
	}

	myID := *idFlag
	port := *portFlag

	elevio.Init("localhost:"+port, datatypes.N_FLOORS)

	requestsCh := make(chan [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool)
	completedRequestCh := make(chan datatypes.ButtonEvent)

	go fsm.RunElevFSM(requestsCh, completedRequestCh)
	go requests.RequestControlLoop(myID, requestsCh, completedRequestCh)

	select {}
}
