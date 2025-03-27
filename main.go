package main

import (
	"flag"
	"fmt"
	"project/config"
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
		fmt.Println("Error: ID flag is required, eg. --id=Elev1")
		return
	}

	myID := *idFlag
	port := *portFlag

	elevio.Init("localhost:"+port, config.NUM_FLOORS)

	requestsChan := make(chan [config.NUM_FLOORS][config.NUM_BUTTONS]bool)
	completedRequestChan := make(chan datatypes.ButtonEvent)

	go fsm.RunElevFSM(requestsChan, completedRequestChan)
	go requests.DistributedRequestLoop(myID, requestsChan, completedRequestChan)

	select {}
}
