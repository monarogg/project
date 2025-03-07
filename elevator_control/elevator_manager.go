package elevator_control

import (
	"project/datatypes"
	"project/elevio"
	"time"
)

// var sharedInfo datatypes.ElevSharedInfo

// type ElevatorManager struct {
// 	elevators map[string]*datatypes.Elevator
// 	contexts  map[string]*datatypes.ElevatorContext
// 	mutex     sync.Mutex
// }

// // oppretter global instans av ElevatorManager
// var manager = ElevatorManager{
// 	elevators: make(map[string]*datatypes.Elevator),
// 	contexts:  make(map[string]*datatypes.ElevatorContext),
// }

// // Henter / oppretter en elevator for en spesifikk heis:
// func GetElevator(elevatorID string) *datatypes.Elevator {
// 	manager.mutex.Lock()
// 	defer manager.mutex.Unlock()

// 	if elev, exists := manager.elevators[elevatorID]; exists {
// 		return elev
// 	}

// 	// hvis heisen ikke finnes: opprett en:
// 	newElevator := &datatypes.Elevator{
// 		CurrentFloor: 0,
// 		State:        datatypes.Idle,
// 		Orders:       [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool{},
// 		StopActive:   false,
// 		Config:       datatypes.ElevatorConfig{},
// 	}

// 	manager.elevators[elevatorID] = newElevator
// 	return newElevator
// }

// // Henter / oppretter en ElevatorContext for en spesifikk heis:

// func GetElevatorContext(elevatorID string) *datatypes.ElevatorContext {
// 	manager.mutex.Lock()
// 	defer manager.mutex.Unlock()

// 	if ctx, exists := manager.contexts[elevatorID]; exists {
// 		return ctx
// 	}

// 	// hvis contexten ikke finnes, opprett ny:
// 	newCtx := &datatypes.ElevatorContext{
// 		AllCabRequests:   make(map[string][datatypes.N_FLOORS]datatypes.RequestType),
// 		UpdatedInfoElevs: make(map[string]datatypes.ElevatorInfo),
// 		LocalID:          elevatorID,
// 	}

// 	manager.contexts[elevatorID] = newCtx
// 	return newCtx
// }

// func updateInfoElev(e datatypes.Elevator) {
// 	sharedInfo.Mutex.RLock() // Read lock, when gorutines only read data, no writing
// 	defer sharedInfo.Mutex.RUnlock()

// 	sharedInfo.Behaviour = e.State
// 	sharedInfo.Direction = e.Direction
// 	sharedInfo.CurrentFloor = e.CurrentFloor

// }

// func setElevAvailability(val bool) {
// 	sharedInfo.Mutex.Lock() // gorutine skal ha eksklusiv tilgang
// 	defer sharedInfo.Mutex.Unlock()

// 	sharedInfo.Available = val
// }

//// NY FRA HER:

var sharedInfoElevs datatypes.ElevSharedInfo

// henter informasjon om heis, returnerer en kopi av heisens tilstand
func GetInfoElev() datatypes.ElevatorInfo {
	sharedInfoElevs.Mutex.RLock()
	defer sharedInfoElevs.Mutex.RUnlock()

	return datatypes.ElevatorInfo{
		Available:    sharedInfoElevs.Available,
		Behaviour:    sharedInfoElevs.Behaviour,
		Direction:    sharedInfoElevs.Direction,
		CurrentFloor: sharedInfoElevs.CurrentFloor,
	}
}

// oppdaterer info om heis
func updateInfoElev(e datatypes.Elevator) {
	sharedInfoElevs.Mutex.Lock()
	defer sharedInfoElevs.Mutex.Unlock()

	sharedInfoElevs.Behaviour = e.State
	sharedInfoElevs.Direction = e.Direction
	sharedInfoElevs.CurrentFloor = e.CurrentFloor
}

// endrer tilgjengelighet til heisen basert på val
func setElevAvailability(val bool) {
	sharedInfoElevs.Mutex.Lock()
	defer sharedInfoElevs.Mutex.Unlock()

	sharedInfoElevs.Available = val
}

// initialiserer heisen, vet da ikke hvilken etasje den er i - må få gyldig etasje
func initElevator(chanFloorSensor <-chan int) datatypes.Elevator {
	elevio.SetDoorOpenLamp(false) // slår av lampe for door open

	// slår av alle etasjelys
	for f := 0; f < datatypes.N_FLOORS; f++ {
		for b := 0; b < datatypes.N_BUTTONS; b++ {
			elevio.SetButtonLamp(elevio.ButtonType(b), f, false)
		}
	}

	elevio.SetMotorDirection(elevio.MD_Down) // setter retning ned for å finne gyldig etasje
	currentFloor := <-chanFloorSensor        // venter på etasje sensor til å angi en etasje
	elevio.SetMotorDirection(elevio.MD_Stop) // stopper heisen i den funnede etasjen
	elevio.SetFloorIndicator(currentFloor)   // oppdaterer heisens etasje med lampe

	return datatypes.Elevator{CurrentFloor: currentFloor, Direction: dirConv(datatypes.DIR_STOP), Orders: [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool{}, State: datatypes.Idle}
}

// starter/nullstiller en timer til et nytt antall sekunder
func restartTimer(timer *time.Timer, sec int) {
	timer.Reset(time.Duration(sec) * time.Second)
}

// stopper en aktiv timer
func killTimer(timer *time.Timer) {
	if !timer.Stop() {
		<-timer.C
	}
}

// oversetter en Direction (int) til en motorretning (MotorDirection)
func dirConv(dir datatypes.Direction) elevio.MotorDirection {
	switch dir {
	case datatypes.DIR_DOWN:
		return elevio.MotorDirection(datatypes.MD_DOWN)
	case datatypes.DIR_STOP:
		return elevio.MotorDirection(datatypes.MD_STOP)
	case datatypes.DIR_UP:
		return elevio.MotorDirection(datatypes.MD_UP)
	}
	return elevio.MotorDirection(datatypes.MD_STOP)
}
