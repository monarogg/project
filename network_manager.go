package elevator_network

import (
	"fmt"
	"project/datatypes"
	"project/elevio"
	"project/network/bcast"
	"project/network/peers"
	"project/requests/request_handler"
	"sync"
	"time"
)

const (
	HEARTBEAT_INTERVAL = 100 * time.Millisecond
	FAILURE_THRESHOLD  = 5               // Number of consecutive missed heartbeats before considering an elevator failed
	CHECK_INTERVAL     = 3 * time.Second // Interval for checking missing elevators
	GRACE_PERIOD       = 3 * time.Second // Extra time before reassigning requests
)

type ElevatorNetwork struct {
	ID             string
	KnownElevators sync.Map // Concurrency-safe map (elevatorID -> datatypes.NetElevator)
	LastSeen       sync.Map // Concurrency-safe map (elevatorID -> time.Time)
	TxElevState    chan datatypes.NetElevator
	RxElevState    chan datatypes.NetElevator
	TxPeerEnable   chan bool
	RxPeerUpdates  chan peers.PeerUpdate
}

// Initializes and returns an ElevatorNetwork instance
func InitElevatorNetwork(id string) *ElevatorNetwork {
	network := &ElevatorNetwork{
		ID:            id,
		TxElevState:   make(chan datatypes.NetElevator),
		RxElevState:   make(chan datatypes.NetElevator),
		TxPeerEnable:  make(chan bool),
		RxPeerUpdates: make(chan peers.PeerUpdate),
	}

	// Start the transmitter/receiver for peers
	go bcast.Transmitter(17658, network.TxPeerEnable)
	go bcast.Receiver(17658, network.RxPeerUpdates)

	// Start the transmitter/receiver for elevator states
	go bcast.Transmitter(17657, network.TxElevState)
	go bcast.Receiver(17657, network.RxElevState)

	// Start broadcasting ID
	network.TxPeerEnable <- true

	// Start monitoring for missing elevators
	go network.DetectMissingElevators()

	return network
}

func (net *ElevatorNetwork) StartHeartbeat(elevator *datatypes.Elevator) {
	go func() {
		for {
			currentState := datatypes.NetElevator{
				ID:           net.ID,
				CurrentFloor: elevator.CurrentFloor,
				Direction:    elevator.Direction,
				State:        elevator.State,
				Orders:       elevator.Orders, // Keep existing orders on rejoin
				StopActive:   elevator.StopActive,
			}

			// Only update the network if the elevator has valid position
			if elevator.CurrentFloor != -1 {
				net.TxElevState <- currentState
				net.LastSeen.Store(net.ID, time.Now()) // Update last seen timestamp
			}

			time.Sleep(HEARTBEAT_INTERVAL)
		}
	}()
}


// Listens for elevator states from peers and updates last seen timestamps
func (net *ElevatorNetwork) ListenForStates() {
	go func() {
		for {
			tempState := <-net.RxElevState

			// Skip self to avoid overwriting valid state
			if tempState.ID == net.ID {
				continue
			}

			// Update known elevators
			net.KnownElevators.Store(tempState.ID, tempState)

			// Update last seen timestamp
			net.LastSeen.Store(tempState.ID, time.Now())

			// If this elevator was previously considered missing, mark it as available again
			if _, exists := net.KnownElevators.Load(tempState.ID); !exists {
				fmt.Println("Elevator", tempState.ID, "has rejoined the network.")
			}

			fmt.Println("Received state from:", tempState.ID, "| Floor:", tempState.CurrentFloor, "| Direction:", tempState.Direction, "| State:", tempState.State)
		}
	}()
}



// Periodically checks for missing elevators and removes them after a grace period
func (net *ElevatorNetwork) DetectMissingElevators() {
	for {
		time.Sleep(CHECK_INTERVAL) // Check every 3 seconds

		now := time.Now()

		net.LastSeen.Range(func(id, lastSeenTime interface{}) bool {
			elevatorID := id.(string)
			lastSeen := lastSeenTime.(time.Time)

			// Mark as missing if not seen in the failure threshold
			if now.Sub(lastSeen) > FAILURE_THRESHOLD*HEARTBEAT_INTERVAL+GRACE_PERIOD {
				fmt.Println("Elevator", elevatorID, "is missing!")
				net.RemoveMissingElevator(elevatorID)
			}
			return true
		})
	}
}

// Handles missing elevator removal
func (net *ElevatorNetwork) RemoveMissingElevator(missingID string) {
	// Check if the elevator still exists before removing
	if _, exists := net.KnownElevators.Load(missingID); !exists {
		return
	}

	// Remove from known elevators
	net.KnownElevators.Delete(missingID)

	// Remove last seen timestamp
	net.LastSeen.Delete(missingID)

	// Remove orders assigned to the missing elevator
	net.ReassignRequests(missingID)

	fmt.Println("Removed elevator:", missingID)
}

func (net *ElevatorNetwork) ReassignRequests(missingID string) {
	fmt.Println("Reassigning hall requests for missing elevator:", missingID)

	// Get the missing elevator's last known state
	val, exists := net.KnownElevators.Load(missingID)
	if !exists {
		fmt.Println("No record of missing elevator, skipping reassignment")
		return
	}
	missingElevator := val.(datatypes.NetElevator)

	// Collect hall requests that were assigned to the missing elevator
	var updatedHallRequests [datatypes.N_FLOORS][datatypes.N_HALL_BUTTONS]datatypes.RequestType
	for floor := 0; floor < datatypes.N_FLOORS; floor++ {
		for button := 0; button < datatypes.N_HALL_BUTTONS; button++ {
			if missingElevator.Orders[floor][button] {
				fmt.Printf("Request at floor %d button %d was assigned to missing elevator %s\n", floor, button, missingID)
				updatedHallRequests[floor][button] = datatypes.RequestType{State: datatypes.Unassigned}
			}
		}
	}
	

	// Gather information from other elevators
	allCabRequests := make(map[string][datatypes.N_FLOORS]datatypes.RequestType)
	updatedInfoElevs := make(map[string]datatypes.ElevatorInfo)
	var peerList []string

	net.KnownElevators.Range(func(id, value interface{}) bool {
		elevID := id.(string)
		elev := value.(datatypes.NetElevator)
	
		// Skip the missing elevator
		if elevID == missingID {
			return true
		}
	
		peerList = append(peerList, elevID)
	
		// Convert `elev.Orders` from `[N_FLOORS][N_BUTTONS]bool` to `[N_FLOORS]datatypes.RequestType`
		var convertedOrders [datatypes.N_FLOORS]datatypes.RequestType
		for floor := 0; floor < datatypes.N_FLOORS; floor++ {
			// Only extract the cab requests (assuming cab calls are always stored in `elev.Orders[floor][2]`)
			if elev.Orders[floor][elevio.BT_Cab] {
				convertedOrders[floor] = datatypes.RequestType{State: datatypes.Assigned}
			}
		}
	
		allCabRequests[elevID] = convertedOrders 
	
		updatedInfoElevs[elevID] = datatypes.ElevatorInfo{
			Available: true,
			Behaviour: elev.State,
			Direction: elev.Direction,
			Floor:     elev.CurrentFloor,
		}
	
		return true
	})
	

	// Run RequestAssigner to redistribute requests
	newOrders := requesthandler.RequestAssigner(updatedHallRequests, allCabRequests, updatedInfoElevs, peerList, net.ID)

	// Broadcast the reassigned orders
	fmt.Println("Broadcasting reassigned requests...")
	net.TxElevState <- datatypes.NetElevator{
		ID:     net.ID,
		Orders: newOrders,
	}
}
