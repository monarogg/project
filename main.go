package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os/exec"
	"project/datatypes"
	"project/elevio"
	"project/fsm"
	"project/requests"
	"strconv"
	"strings"
	"time"
)

const (
	udpPort           = ":40000"
	heartBeatInterval = 1 * time.Second
	heartBeatTimeout  = 3 * time.Second
)

func runElevator(myID, port string) {

	elevio.Init("localhost:"+port, datatypes.N_FLOORS)

	requestsCh := make(chan [datatypes.N_FLOORS][datatypes.N_BUTTONS]bool)
	completedRequestCh := make(chan datatypes.ButtonEvent)

	go fsm.RunElevFSM(requestsCh, completedRequestCh)
	go requests.RequestControlLoop(myID, requestsCh, completedRequestCh)

	select {}
}

func runPrimary(myID, port string) {
	fmt.Println("Starting in primary mode")
	// starter backup-prosess i ny terminal med korrekte flagg

	// FORM MAC:
	// command := fmt.Sprintf(`tell application "Terminal" to do script "/Users/monarogg/Documents/SANNTID/project && go run main.go -role=backup -id=%s -port=%s"`, myID, port)
	// err := exec.Command("osascript", "-e", command).Start()
	// if err != nil {
	// 	log.Fatal("kunne ikke starte backup-prosess: ", err)
	// }

	// FOR LINUX:
	err := exec.Command("gnome-terminal", "--", "go", "run", "main.go", "-role=backup", "id="+myID).Start()
	if err != nil {
		log.Fatal("kunne ikke starte backup.prosess", err)
	}

	// starter sending av heartbeat:
	go sendHeartbeat()

	// starter elevator:
	runElevator(myID, port)
}

func runBackup(myID, port string) {
	fmt.Println("Starting in backup mode")

	addr, err := net.ResolveUDPAddr("udp", udpPort)
	if err != nil {
		log.Fatal("Error with resolving address")
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal("Error with setting up socket")
	}
	defer conn.Close()

	buffer := make([]byte, 1024)
	// setter initiell timeout for heartbeat:
	conn.SetReadDeadline(time.Now().Add(heartBeatTimeout))

	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Ingen heartbeat mottatt innen tidsbegrensning, går videre til primary")
			runPrimary(myID, port)
			return
		}
		heartbeatStr := strings.TrimSpace(string(buffer[:n]))
		heartbeatVal, err := strconv.Atoi(heartbeatStr)
		if err != nil {
			fmt.Println("Ugyldig heartbeat-melding", err)
			continue
		}
		fmt.Println("Mottok heartbeat: ", heartbeatVal)
		conn.SetReadDeadline(time.Now().Add(heartBeatTimeout))
	}
}

func sendHeartbeat() {
	bcastAddr, err := net.ResolveUDPAddr("udp", "255.255.255.255"+udpPort)
	if err != nil {
		log.Fatal("Error with resolving address", err)
	}

	conn, err := net.DialUDP("udp", nil, bcastAddr)
	if err != nil {
		log.Fatal("Error with creating UDP-connection", err)
	}
	defer conn.Close()

	heartbeat := 0
	for {
		heartbeat++
		msg := strconv.Itoa(heartbeat)
		_, err := conn.Write([]byte(msg))
		if err != nil {
			log.Fatal("Error with sending heartbeat", err)
		}
		fmt.Println("Sendt heartbeat: ", msg)
		time.Sleep(heartBeatInterval)
	}
}
func main() {

	idFlag := flag.String("id", "", "Unique ID for this elevator")
	portFlag := flag.String("port", "15657", "Simulator port")
	roleFlag := flag.String("role", "primary", "Role: primary eller backup")
	flag.Parse()

	if *idFlag == "" {
		fmt.Println("Error: -id must be provided")
		return
	}

	myID := *idFlag
	port := *portFlag

	if *roleFlag == "primary" {
		runPrimary(myID, port)
	} else if *roleFlag == "backup" {
		runBackup(myID, port)
	} else {
		fmt.Println("Ugyldig rolle. Bruk enten 'primary' eller 'backup'")
	}
	select {}
}
