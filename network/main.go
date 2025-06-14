package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DiffusionStatus struct {
	sender_id   string
	nbNeighbors int
	parent      string
}

var mutex = &sync.Mutex{}
var DiffusionStatusMap = make(map[string]*DiffusionStatus)
var connectedSites = make(map[string]*net.Conn)

// value
const (
	BlueMsg string = "blu"
	RedMsg  string = "red"
)

// key
const (
	SiteIdField       string = "sid" // site id of sender
	SiteIdDestField   string = "did" // site id of destination
	DiffusionStatusID string = "dsid"
	ColorDiffusion    string = "clr"
	MessageContent    string = "mct"
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

	// Implement progressive random topology
	connectToRandomTopology()

	// Wait a bit to ensure connections are established
	time.Sleep(1 * time.Second)

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
		display_w("New connection from " + conn.RemoteAddr().String())
		// Handle receiving connexion
		go handleReceivingConnection(conn)
	}
}

func connectToPeer(port int) {
	addr := fmt.Sprintf("localhost:%d", port)

	// Avoid connecting to the same peer multiple times
	if _, exists := connectedSites[addr]; exists {
		return
	}

	// Try to connect with retries (for progressive joining)
	maxRetries := 30
	retryDelay := 500 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			if attempt == 0 {
				display_w(fmt.Sprintf("Waiting for peer on %s to become available...", addr))
			}
			time.Sleep(retryDelay)
			continue
		}
		connectedSites[addr] = &conn
		printConnectedSites()
		display_w("Connected to peer on " + addr)

		// Send port
		fmt.Fprintf(conn, "PORT:%d\n", portBase+*id)

		go handleSendingConnection(conn)
		return
	}

	display_e(fmt.Sprintf("Failed to connect to %s after %d attempts", addr, maxRetries))
}

func handleReceivingConnection(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)

	var remotePort int
	if scanner.Scan() {
		msg := scanner.Text()
		if strings.HasPrefix(msg, "PORT:") {
			remotePort, _ = strconv.Atoi(strings.TrimPrefix(msg, "PORT:"))
			addr := fmt.Sprintf("localhost:%d", remotePort)

			// We connect to the port if not already connected
			if _, exists := connectedSites[addr]; !exists {
				display_w("Back-connecting to " + addr)
				go connectToPeer(remotePort)
			}
		} else {
			display_w("Unexpected message: " + msg)
		}
	}

	// Message processing loop
	for scanner.Scan() {
		msg := scanner.Text()
		display_w("Received: " + msg)
		if msg != "" { // FIXME: transfer the message to controller without doing anything for now
			msg_diffusion_id := findval(msg, DiffusionStatusID, false)
			msg_color := findval(msg, ColorDiffusion, false)
			msg_sender := findval(msg, SiteIdField, false)
			msg_content := findval(msg, MessageContent, false)

			current_duffusion_status := DiffusionStatusMap[msg_diffusion_id]

			if current_duffusion_status == nil {
				current_duffusion_status := &DiffusionStatus{
					sender_id:   "", //FIXME : shold add ful id
					nbNeighbors: 1,  // FIXME: shold be repalced by len(connectedSites)
					parent:      "",
				}
				DiffusionStatusMap[msg_diffusion_id] = current_duffusion_status

			}

			if msg_color == BlueMsg {
				if current_duffusion_status.parent == "" {
					// send message to the controleur
					fmt.Println(msg)

					// update diffusion status
					current_duffusion_status.parent = msg_sender
					current_duffusion_status.nbNeighbors -= 1

					if current_duffusion_status.nbNeighbors > 0 {
						// FIXME : send to all neibors except the parent
						sndmsg := prepareWaveMessages(msg_diffusion_id, BlueMsg, id, "", msg_content)
						sendWaveMassages(nil, msg_sender, sndmsg, nil)
					} else {
						sndmsg := prepareWaveMessages(msg_diffusion_id, RedMsg, id, msg_sender, msg_content)
						// send only to parent
						_, err := conn.Write([]byte(sndmsg))
						if err != nil {
							display_e("Error sending message to " + current_duffusion_status.parent + ": " + err.Error())
							continue
						}

					}

				} else {
					// Has already received this message
					// FIXME : send message to site's parent

					sndmsg := prepareWaveMessages(msg_diffusion_id, RedMsg, id, msg_sender, msg_content)
					// send only to parent
					_, err := conn.Write([]byte(sndmsg))
					if err != nil {
						display_e("Error sending message to " + current_duffusion_status.parent + ": " + err.Error())
						continue
					}
				}

			} else if msg_color == RedMsg {
				current_duffusion_status.nbNeighbors -= 1
				if current_duffusion_status.nbNeighbors == 0 {
					if current_duffusion_status.parent == strconv.Itoa(*id) {
						// send message to the controleur
						fmt.Println(msg)
					} else {
						// forward the message to the wave initiator
						sndmsg := prepareWaveMessages(msg_diffusion_id, RedMsg, id, current_duffusion_status.parent, msg_content)
						// send only to parent
						_, err := conn.Write([]byte(sndmsg))
						if err != nil {
							display_e("Error sending message to " + current_duffusion_status.parent + ": " + err.Error())
							continue
						}
					}
				}
			}
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

		sndmsg := prepareWaveMessages(diffusionId, BlueMsg, id, "", msg)

		sendWaveMassages(nil, strconv.Itoa(*id), sndmsg, nil)
		mutex.Unlock()
	}
}

// ===========================

// Helper function to print connected sites
func printConnectedSites() {
	mutex.Lock()
	defer mutex.Unlock()

	display_e(fmt.Sprintf("Connected sites (%d total):", len(connectedSites)))
	for addr, conn := range connectedSites {
		if conn != nil && *conn != nil {
			display_e(fmt.Sprintf("  - %s (active)", addr))
		} else {
			display_e(fmt.Sprintf("  - %s (nil connection)", addr))
		}
	}
	display_e("")
}

func connectToRandomTopology() {
	// All sites use the same logic: choose number of connections based on their ID
	// Site A (id=0): chooses between 0 and 0 (always 0)
	// Site B (id=1): chooses between 1 and 1 (always 1)
	// Site C and beyond: choose between 1 and min(id, 3)

	// Create a local random generator
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(*id)))

	// Determine the range for number of connections
	var minConnections, maxConnections int

	if *id == 0 {
		minConnections = 0
		maxConnections = 0
	} else {
		minConnections = 1
		maxConnections = min(*id, 3)
	}

	// Choose number of connections within the range
	var numConnections int
	if minConnections == maxConnections {
		numConnections = minConnections
	} else {
		numConnections = rng.Intn(maxConnections - minConnections + 1) + minConnections
	}

	display_w(fmt.Sprintf("Site %d attempting %d connections to existing sites", *id, numConnections))

	// If no connections needed, return early
	if numConnections == 0 {
		display_w(fmt.Sprintf("Site %d starting alone", *id))
		return
	}

	// Create list of existing sites (0 to id-1)
	existingSites := make([]int, *id)
	for i := 0; i < *id; i++ {
		existingSites[i] = i
	}

	// Shuffle the list and take the first numConnections
	for i := len(existingSites) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		existingSites[i], existingSites[j] = existingSites[j], existingSites[i]
	}

	// Connect to the selected sites
	connectionsToMake := numConnections
	if connectionsToMake > len(existingSites) {
		connectionsToMake = len(existingSites)
	}

	for i := 0; i < connectionsToMake; i++ {
		targetSite := existingSites[i]
		targetPort := portBase + targetSite
		display_e(fmt.Sprintf("Site %d connecting to Site %d (port %d)", *id, targetSite, targetPort))
		go connectToPeer(targetPort)
		// Small delay between connections to avoid overwhelming
		time.Sleep(100 * time.Millisecond)
	}
}
