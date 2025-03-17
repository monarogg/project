package elevator_control

/// EVT FJERNE OG SØRGE FOR AT FUNKSJONALITETEN ER FORDELT MELLOM ELEVATOR_MANAGER OG FSM
import (
	"fmt"
	"project/datatypes"
	"project/elevio"
	"project/requests"
	requesthandler "project/requests/request_handler"
	"time"
)

func InitializeFSM() datatypes.Elevator { // funksjonen returnerer ferdiginitialisert instans av strukturen Elevator

	elevator := datatypes.Elevator{
		CurrentFloor: -1,                  //starter i første etasje
		Direction:    datatypes.DIR_STOP, // motoren skal stå i ro
		State:        datatypes.Idle,     //starter som inaktiv
		Orders:       [4][3]bool{},       //ingen bestillinger
	}

	return elevator
}

// endrer heisens state og oppdaterer heisens orders:
func OnRequestButtonPress(elevator *datatypes.Elevator, btnFloor int, btnType elevio.ButtonType,
	context *datatypes.ElevatorContext) {

	elevator.Orders[btnFloor][btnType] = true //legger til request i Orders

	fmt.Println("Etter knappetrykk: Orders = ", elevator.Orders)

	if _, exists := context.AllCabRequests[context.LocalID]; !exists {
		var emptyArray [datatypes.N_FLOORS]datatypes.RequestType // Opprett en tom array
		context.AllCabRequests[context.LocalID] = emptyArray
	}

	tempArray := context.AllCabRequests[context.LocalID] // henter ut arrayet
	tempArray[btnFloor] = datatypes.RequestType{State: datatypes.Assigned}
	context.AllCabRequests[context.LocalID] = tempArray // legg tilbake i arrayet etter å ha endret status

	fmt.Println("Oppdatert før RequestAssigner: AllCabRequests =", context.AllCabRequests)

	// Oppdater updatedInfoElevs før vi kaller RequestAssigner
	context.UpdatedInfoElevs[context.LocalID] = datatypes.ElevatorInfo{
		Available:    true,
		Behaviour:    elevator.State,
		Direction:    elevator.Direction,
		CurrentFloor: elevator.CurrentFloor,
	}

	// Debugging: Sjekk at updatedInfoElevs er oppdatert før RequestAssigner kalles
	fmt.Println(" Oppdatert updatedInfoElevs før RequestAssigner:", context.UpdatedInfoElevs)

	// kaller på RequestAssigner for å sjekke ny fordeling av orders:
	newOrders := requesthandler.RequestAssigner(context.HallRequests, context.AllCabRequests, context.UpdatedInfoElevs, context.PeerList, context.LocalID)

	// oppdaterer orders for denne heisen.
	elevator.Orders = newOrders

	fmt.Println(" Etter oppdatering: AllCabRequests =", context.AllCabRequests)

	switch elevator.State {
	case datatypes.DoorOpen:
		if elevator.CurrentFloor == btnFloor {
			StartDoorTimer(elevator, elevator.Config.DoorOpenDuration, context)
			requests.ClearRequestsAtFloor(elevator)
		}
	case datatypes.Moving:
		// Ønsker kun å legge til request i Orders, det er allerede gjort over.
	case datatypes.Idle:

		//Hvis heisen står i ro, og requesten er på samme etasje:
		if elevator.CurrentFloor == btnFloor {
			elevator.State = datatypes.DoorOpen
			elevio.SetMotorDirection(elevio.MD_Stop)
			elevio.SetDoorOpenLamp(true)
			requests.ClearRequestsAtFloor(elevator)
			StartDoorTimer(elevator, 2*time.Second, context)
		} else {

			// dersom heisen er inaktiv (Idle), skal velge ny retning og tilstand
			dir, beh := requests.ChooseNewDirAndBeh(elevator) // velger retning basert på Orders
			elevator.Direction = dir
			elevator.State = beh

			if beh == datatypes.Moving {
				/// MÅ VÆRE NOEN ENDRINGER MEN VET IKKE HELT HVILKE
			}
			if dirnBehaviour == elevio.MD_Stop {
				// er ingen bestillinger i Orders - heisen skal være Idle
				elevator.State = datatypes.Idle
			} else {
				// det er flere bestillinger - heisen skal være Moving
				elevator.State = datatypes.Moving
				elevio.SetMotorDirection(dirnBehaviour)
			}
		}
	}
}

func StartDoorTimer(elevator *datatypes.Elevator, duration time.Duration, context *datatypes.ElevatorContext) {
	time.AfterFunc(duration, func() { //time.Afterfunc starter en timer som varer i duration
		OnDoorTimeout(elevator, context)

	})
}

func OnFloorArrival(elevator *datatypes.Elevator, floor int, context *datatypes.ElevatorContext) {
	elevator.CurrentFloor = floor // oppdaterer current floor
	elevio.SetFloorIndicator(floor)

	if requests.ShouldStop(elevator) {
		elevio.SetMotorDirection(elevio.MD_Stop)
		elevator.Direction = elevio.MD_Stop

		elevio.SetDoorOpenLamp(true)
		elevator.State = datatypes.DoorOpen

		requests.ClearRequestsAtFloor(elevator)

		StartDoorTimer(elevator, 2*time.Second, context)
	}
}

func OnDoorTimeout(elevator *datatypes.Elevator, context *datatypes.ElevatorContext) {

	elevio.SetDoorOpenLamp(false)

	// Kaller på RequestAssigner for å sjekke ny fordeling av oppgaver
	newOrders := requesthandler.RequestAssigner(context.HallRequests, context.AllCabRequests, context.UpdatedInfoElevs, context.PeerList, context.LocalID)
	elevator.Orders = newOrders

	elevator.Direction = requests.ChooseDirection(elevator)

	if elevator.Direction == elevio.MD_Stop {
		elevator.State = datatypes.Idle
	} else {
		elevator.State = datatypes.Moving
		elevio.SetMotorDirection(elevator.Direction)
	}
}

func OnStopButtonPress(elevator *datatypes.Elevator) {
	if elevator.StopActive {
		// dersom stoppknappen så trykkes, skal stoppmodus deaktiveres:
		elevator.StopActive = false
		elevio.SetStopLamp(false)

	} else {
		elevator.StopActive = true
		elevio.SetStopLamp(true)
		elevio.SetMotorDirection(elevio.MD_Stop)
		ClearAllRequests(elevator)
		elevator.State = datatypes.Idle
		UpdateLights(elevator)
	}
}

func UpdateLights(elevator *datatypes.Elevator) {
	// skal oppdatere button lights basert på aktive orders i Orders
	for f := 0; f < len(elevator.Orders); f++ {
		for b := 0; b < 3; b++ {
			elevio.SetButtonLamp(elevio.ButtonType(b), f, elevator.Orders[f][b])
		}
	}
}

func ClearAllRequests(elevator *datatypes.Elevator) {
	for f := 0; f < len(elevator.Orders); f++ {
		for b := 0; b < 3; b++ {
			elevator.Orders[f][b] = false
			elevio.SetButtonLamp(elevio.ButtonType(b), f, false)
		}
	}
}
