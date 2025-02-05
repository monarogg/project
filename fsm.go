package main

import (
	"project/elevio"
	"time"
)

type ElevatorState int

const (
	Idle     ElevatorState = 0
	Moving   ElevatorState = 1
	DoorOpen ElevatorState = 2
)

type Elevator struct {
	CurrentFloor int
	Direction    elevio.MotorDirection
	State        ElevatorState
	Orders       [4][3]bool
	Config       ElevatorConfig
	StopActive   bool
}

type ElevatorConfig struct {
	DoorOpenDuration time.Duration
}

func initializeFSM() Elevator { // funksjonen returnerer ferdiginitialisert instans av strukturen Elevator

	elevator := Elevator{
		CurrentFloor: 0,              //starter i første etasje
		Direction:    elevio.MD_Stop, // motoren skal stå i ro
		State:        Idle,           //starter som inaktiv
		Orders:       [4][3]bool{},   //ingen bestillinger
	}

	return elevator
}

func OnRequestButtonPress(elevator *Elevator, btnFloor int, btnType elevio.ButtonType) {

	elevator.Orders[btnFloor][btnType] = true //legger til request i Orders

	switch elevator.State {
	case DoorOpen:
		if elevator.CurrentFloor == btnFloor {
			StartDoorTimer(elevator, elevator.Config.DoorOpenDuration)
			ClearRequestsAtFloor(elevator)
		}
	case Moving:
		// Ønsker kun å legge til request i Orders, det er allerede gjort over.
	case Idle:

		//Hvis heisen står i ro, og requesten er på samme etasje:
		if elevator.CurrentFloor == btnFloor {
			elevator.State = DoorOpen
			elevio.SetMotorDirection(elevio.MD_Stop)
			elevio.SetDoorOpenLamp(true)
			ClearRequestsAtFloor(elevator)
			StartDoorTimer(elevator, 2*time.Second)
		} else {

			// dersom heisen er inaktiv (Idle), skal velge ny retning og tilstand
			dirnBehaviour := ChooseDirection(elevator) // velger retning basert på Orders
			elevator.Direction = dirnBehaviour

			if dirnBehaviour == elevio.MD_Stop {
				// er ingen bestillinger i Orders - heisen skal være Idle
				elevator.State = Idle
			} else {
				// det er flere bestillinger - heisen skal være Moving
				elevator.State = Moving
				elevio.SetMotorDirection(dirnBehaviour)
			}
		}
	}
}

func StartDoorTimer(elevator *Elevator, duration time.Duration) {
	time.AfterFunc(duration, func() { //time.Afterfunc starter en timer som varer i duration
		OnDoorTimeout(elevator)

	})
}

func OnFloorArrival(elevator *Elevator, floor int) {
	elevator.CurrentFloor = floor // oppdaterer current floor
	elevio.SetFloorIndicator(floor)

	if ShouldStop(elevator) {
		elevio.SetMotorDirection(elevio.MD_Stop)
		elevator.Direction = elevio.MD_Stop

		elevio.SetDoorOpenLamp(true)
		elevator.State = DoorOpen

		ClearRequestsAtFloor(elevator)

		StartDoorTimer(elevator, 2*time.Second)
	}
}

// funksjon som skal brukes når døren har vært åpen tilstrekkelig lenge (dør skal lukkes osv.):
//func OnDoorTimeout(elevator *Elevator) {
//	elevio.SetDoorOpenLamp(false)
//
//	elevator.Direction = ChooseDirection(elevator)
//	if elevator.Direction == elevio.MD_Stop {
//		elevator.State = Idle
//	} else {
//		elevator.State = Moving
//		elevio.SetMotorDirection(elevator.Direction)
//
//	}
//}

func OnDoorTimeout(elevator *Elevator) {
	elevio.SetDoorOpenLamp(false)
	elevator.Direction = ChooseDirection(elevator)

	if elevator.Direction == elevio.MD_Stop {
		elevator.State = Idle
	} else {
		elevator.State = Moving
		elevio.SetMotorDirection(elevator.Direction)
	}
}

func OnStopButtonPress(elevator *Elevator) {
	if elevator.StopActive {
		// dersom stoppknappen så trykkes, skal stoppmodus deaktiveres:
		elevator.StopActive = false
		elevio.SetStopLamp(false)

	} else {
		elevator.StopActive = true
		elevio.SetStopLamp(true)
		elevio.SetMotorDirection(elevio.MD_Stop)
		ClearAllRequests(elevator)
		elevator.State = Idle
		UpdateLights(elevator)
	}
}

func UpdateLights(elevator *Elevator) {
	// skal oppdatere button lights basert på aktive orders i Orders
	for f := 0; f < len(elevator.Orders); f++ {
		for b := 0; b < 3; b++ {
			elevio.SetButtonLamp(elevio.ButtonType(b), f, elevator.Orders[f][b])
		}
	}
}
