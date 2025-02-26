package elevator_control

import (
	datatypes "project/DataTypes"
	"sync"
)

type ElevatorManager struct {
	elevators map[string]*datatypes.Elevator
	contexts  map[string]*datatypes.ElevatorContext
	mutex     sync.Mutex
}

// oppretter global instans av ElevatorManager
var manager = ElevatorManager{
	elevators: make(map[string]*datatypes.Elevator),
	contexts:  make(map[string]*datatypes.ElevatorContext),
}

// Henter / oppretter en elevator for en spesifikk heis:
func GetElevator(elevatorID string) *datatypes.Elevator {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	if elev, exists := manager.elevators[elevatorID]; exists {
		return elev
	}

	// hvis heisen ikke finnes: opprett en:
	newElevator := &datatypes.Elevator{
		CurrentFloor: 0,
		State:        datatypes.Idle,
		Orders:       [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool{},
		StopActive:   false,
		Config:       datatypes.ElevatorConfig{},
	}

	manager.elevators[elevatorID] = newElevator
	return newElevator
}

// Henter / oppretter en ElevatorContext for en spesifikk heis:

func GetElevatorContext(elevatorID string) *datatypes.ElevatorContext {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	if ctx, exists := manager.contexts[elevatorID]; exists {
		return ctx
	}

	// hvis contexten ikke finnes, opprett ny:
	newCtx := &datatypes.ElevatorContext{
		AllCabRequests:   make(map[string][datatypes.N_FLOORS]datatypes.RequestType),
		UpdatedInfoElevs: make(map[string]datatypes.ElevatorInfo),
		LocalID:          elevatorID,
	}

	manager.contexts[elevatorID] = newCtx
	return newCtx
}
