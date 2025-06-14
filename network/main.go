package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
)

type DiffusionStatus struct {
	sender_id   string
	nbNeighbors int
	parent      string
}

var mutex = &sync.Mutex{}
var DiffusionStatusMap = make(map[string]*DiffusionStatus)

const (
	BlueMsg string = "blu"
	RedMsg  string = "red"
)

const (
	DiffusionStatusID string = "dsid"
	ColorDiffusion    string = "clr"
)

var (
	id *int = flag.Int("id", 0, "id of site")
	N  *int = flag.Int("N", 1, "number of sites")
	s  int  = 0
)

var portBase = 9000 // Base port for node communication

func main() {
	flag.Parse()

	// Listens on its own port
	listenPort := portBase + *id
	go startTCPServer(listenPort)

	//// TODO: connect random to create random network
	//rand.Seed(time.Now().UnixNano())
	//next := rand.Intn(*N - 1)
	//if next >= *id {
	//	next++
	//}
	//nextPort := portBase + next

	// FIXME: for now keep same struct, link to next one
	nextPort := portBase + (*id+1)%*N

	go connectToPeer(nextPort)

	// TODO: tmp wait forever
	select {}
}

func startTCPServer(port int) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		display_e("Server error: " + err.Error())
		return
	}
	display_w("Listening on port " + strconv.Itoa(port) + "...")

	for {
		conn, err := ln.Accept()
		if err != nil {
			display_e("Accept error: " + err.Error())
			continue
		}
		// Handle receiving connexion
		go handleReceivingConnection(conn)
	}
}

func connectToPeer(port int) {
	for {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
		if err != nil {
			// Connexion still not available, wait
			continue
		}
		display_w("Connected to peer on port " + strconv.Itoa(port))
		// Handle sending connexion link peer
		go handleSendingConnection(conn)
		return
	}
}

func handleReceivingConnection(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		msg := scanner.Text()
		display_w("Received: " + msg)
		if msg != "" { // FIXME: transfer the message to controller without doing anything for now
			fmt.Println(msg)
		}
	}
}

func handleSendingConnection(conn net.Conn) {
	reader := bufio.NewReader(os.Stdin)
	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			//display_e("Error reading message : " + err.Error())
			continue
		}

		// FIXME: transfer the message to next network without doing anything for now
		// each message is transfered by wave
		mutex.Lock()

		count := len(DiffusionStatusMap)
		message_id := count

		diffusionId := fmt.Sprintf("%d:message_%d", *id, message_id) // FIXME: add the current port to ID
		diffusionStatus := &DiffusionStatus{
			sender_id:   strconv.Itoa(*id),
			nbNeighbors: 1, // FIXME: shold be repalced by len(connectedSites)
			parent:      strconv.Itoa(*id),
		}

		DiffusionStatusMap[diffusionId] = diffusionStatus

		sndmsg := msg_format(DiffusionStatusID, diffusionId) +
			msg_format(ColorDiffusion, BlueMsg)
		sndmsg += msg

		_, err = conn.Write([]byte(sndmsg))
		if err != nil {
			display_e("Error sending message: " + err.Error())
			continue
		}
		mutex.Unlock()
	}
}
