package elevator_control

import (
	"project/datatypes"
	"project/elevio"
	"time"
	"project/config"
)

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
func UpdateInfoElev(elevator datatypes.Elevator) {
	if (elevator.CurrentFloor == 0 && elevator.Direction == datatypes.DIR_DOWN) ||
	   (elevator.CurrentFloor == config.N_FLOORS-1 && elevator.Direction == datatypes.DIR_UP) {
		elevator.Direction = datatypes.DIR_STOP
		if elevator.State == datatypes.Moving {
			elevator.State = datatypes.Idle
		}
	}

	sharedInfoElevs.Mutex.Lock()
	defer sharedInfoElevs.Mutex.Unlock()

	sharedInfoElevs.Behaviour = elevator.State
	sharedInfoElevs.Direction = elevator.Direction
	sharedInfoElevs.CurrentFloor = elevator.CurrentFloor
}



// endrer tilgjengelighet til heisen basert på val
func SetElevAvailability(val bool) {
	sharedInfoElevs.Mutex.Lock()
	defer sharedInfoElevs.Mutex.Unlock()

	sharedInfoElevs.Available = val
}

// initialiserer heisen, vet da ikke hvilken etasje den er i - må få gyldig etasje
func InitElevator() datatypes.Elevator {
	elevio.SetDoorOpenLamp(false)

	for f := 0; f < config.N_FLOORS; f++ {
		for b := 0; b < config.N_BUTTONS; b++ {
			elevio.SetButtonLamp(elevio.ButtonType(b), f, false)
		}
	}

	elevio.SetMotorDirection(elevio.MD_Down)
	currentFloor := -1
	for currentFloor == -1 {
		currentFloor = elevio.GetFloor()
		time.Sleep(10 * time.Millisecond)
	}
	elevio.SetMotorDirection(elevio.MD_Stop)
	elevio.SetFloorIndicator(currentFloor)

	return datatypes.Elevator{
		CurrentFloor: currentFloor,
		Direction:    datatypes.DIR_STOP,
		State:        datatypes.Idle,
		Orders:       [config.N_FLOORS][config.N_BUTTONS]bool{},
	}
}

// starter/nullstiller en timer til et nytt antall sekunder
func RestartTimer(timer *time.Timer, sec int) {
	timer.Reset(time.Duration(sec) * time.Second)
}

// stopper en aktiv timer
func KillTimer(timer *time.Timer) {
	if !timer.Stop() {
		<-timer.C
	}
}

// oversetter en Direction (int) til en motorretning (MotorDirection)
func DirConv(dir datatypes.Direction) elevio.MotorDirection {
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
func OrdersChanged(old, new [config.N_FLOORS][config.N_BUTTONS]bool) bool {
	for i := range old {
		for j := range old[i] {
			if old[i][j] != new[i][j] {
				return true
			}
		}
	}
	return false
}
