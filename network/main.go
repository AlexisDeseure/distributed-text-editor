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
	timeID int
	id     *int = flag.Int("id", 0, "id of site")
	N      *int = flag.Int("N", 1, "number of sites")
	s      int  = 0
)

var portBase = 9000 // Base port for node communication

func main() {
	timeID = int(time.Now().UnixNano()) //actual time in nano seconds
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

func connectToPeer(port int, timeID int) {
	addr := fmt.Sprintf("localhost:%d", port)

	// Avoid connecting to the same peer multiple times
	if _, exists := connectedSites[strconv.Itoa(timeID)]; exists {
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
		// connectedSites[addr] = &conn
		connectedSites[strconv.Itoa(timeID)] = &conn
		printConnectedSites()
		display_w("Connected to peer on " + addr)

		// Send port
		// fmt.Fprintf(conn, "PORT:%d\n", portBase+*id)
		// send the ID and port based on time creation to the connected site
		fmt.Fprintf(conn, "ID:%d-PORT:%d\n", timeID, portBase+*id)

		go handleSendingConnection(conn)
		return
	}

	display_e(fmt.Sprintf("Failed to connect to %s after %d attempts", addr, maxRetries))
}

func handleReceivingConnection(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)

	var remoteSiteIDStr string
	var remotePort int // Nouvelle variable pour stocker le port distant
	var err error
	// var remotePort int
	// var remoteSiteID int
	if scanner.Scan() {
		msg := scanner.Text()
		// if strings.HasPrefix(msg, "PORT:") {
		// 	remotePort, _ = strconv.Atoi(strings.TrimPrefix(msg, "PORT:"))
		// 	addr := fmt.Sprintf("localhost:%d", remotePort)

		// 	// We connect to the port if not already connected
		// 	if _, exists := connectedSites[addr]; !exists {
		// 		display_w("Back-connecting to " + addr)
		// 		go connectToPeer(remotePort)
		// 	}
		// } else {
		// 	display_w("Unexpected message: " + msg)
		// }
		if strings.HasPrefix(msg, "ID:") && strings.Contains(msg, "-PORT:") {
			parts := strings.Split(msg, "-PORT:")
			if len(parts) != 2 {
				display_e("Malformed initial message: " + msg + " from " + conn.RemoteAddr().String())
				conn.Close()
				return
			}

			// Extraire le timeID
			remoteSiteIDStr = strings.TrimPrefix(parts[0], "ID:")
			_, err = strconv.Atoi(remoteSiteIDStr) // Juste pour valider que c'est un nombre
			if err != nil {
				display_e("Invalid remote ID received: " + remoteSiteIDStr + " from " + conn.RemoteAddr().String())
				conn.Close()
				return
			}

			// Extraire le port
			remotePort, err = strconv.Atoi(parts[1])
			if err != nil {
				display_e("Invalid remote Port received: " + parts[1] + " from " + conn.RemoteAddr().String())
				conn.Close()
				return
			}

			display_w(fmt.Sprintf("Received remote ID: %s and Port: %d from %s", remoteSiteIDStr, remotePort, conn.RemoteAddr().String()))

			mutex.Lock()
			// We connect to the port if not already connected
			if existingConn, exists := connectedSites[remoteSiteIDStr]; exists && existingConn != nil && *existingConn != nil {
				display_w(fmt.Sprintf("Connection to ID: %s (address: %s) already exists. Closing redundant incoming connection from %s.",
					remoteSiteIDStr, (*existingConn).RemoteAddr().String(), conn.RemoteAddr().String()))
				conn.Close() // Fermer la connexion entrante redondante
				mutex.Unlock()
				return
			}
			connectedSites[remoteSiteIDStr] = &conn // Stocke un pointeur vers la connexion
			mutex.Unlock()
			printConnectedSites()

			// Envoyer notre propre timeID et port en retour pour que le site distant puisse nous identifier
			fmt.Fprintf(conn, "ID:%d-PORT:%d\n", timeID, portBase+*id)

			// La logique de back-connecting via 'go connectToPeer(remotePort)' n'est plus nécessaire ici.
			// L'échange d'ID et l'ajout à connectedSites garantissent la connaissance mutuelle.
			// Si le 'connectToPeer' initial échoue, les retries le géreront.

		} else {
			display_w("Unexpected initial message (not ID-PORT format): " + msg + " from " + conn.RemoteAddr().String())
			conn.Close() // Fermer la connexion si le format initial est incorrect
			return
		}
	} else if err = scanner.Err(); err != nil {
		display_e("Error reading initial message: " + err.Error())
		conn.Close()
		return
	} else { // EOF ou connexion fermée avant le message initial
		display_e("Connection closed before initial ID-PORT message from " + conn.RemoteAddr().String())
		conn.Close()
		return
	}

	// 	if strings.HasPrefix(msg, "ID:") {
	// 		remoteSiteID, _ = strconv.Atoi(strings.TrimPrefix(msg, "ID:"))
	// 		display_w("Received remote ID: " + strconv.Itoa(remoteSiteID) + " from " + conn.RemoteAddr().String())
	// 		// We connect to the port if not already connected
	// 		neiborTimerID := strconv.Itoa(remoteSiteID)
	// 		if _, exists := connectedSites[neiborTimerID]; !exists {
	// 			display_w("Back-connecting to " + neiborTimerID)
	// 			go connectToPeer(port, remoteSiteID)
	// 		}

	// 	} else {
	// 		display_w("Unexpected initial message (not an ID): " + msg + " from " + conn.RemoteAddr().String())
	// 		conn.Close() // Fermer la connexion si le format initial est incorrect
	// 		return
	// 	}
	// }

	// Message processing loop
	for scanner.Scan() {
		msg := scanner.Text()
		display_w("Received: " + msg)
		if msg != "" { // FIXME: transfer the message to controller without doing anything for now
			mutex.Lock()
			msg_diffusion_id := findval(msg, DiffusionStatusID, false)
			msg_color := findval(msg, ColorDiffusion, false)
			msg_sender := findval(msg, SiteIdField, false)
			msg_content := findval(msg, MessageContent, false)

			current_duffusion_status := DiffusionStatusMap[msg_diffusion_id]

			if current_duffusion_status == nil {
				current_duffusion_status := &DiffusionStatus{
					sender_id:   "",                  //FIXME : shold add ful id
					nbNeighbors: len(connectedSites), // FIXME: shold be repalced by len(connectedSites)
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
						sndmsg := prepareWaveMessages(msg_diffusion_id, BlueMsg, timeID, "", msg_content)
						sendWaveMassages(connectedSites, msg_sender, sndmsg)
					} else {
						sndmsg := prepareWaveMessages(msg_diffusion_id, RedMsg, timeID, msg_sender, msg_content)
						// send only to parent
						conn := connectedSites[msg_sender]
						_, err := (*conn).Write([]byte(sndmsg))
						if err != nil {
							display_e("Error sending message to " + current_duffusion_status.parent + ": " + err.Error())
							continue
						}

					}

				} else {
					// Has already received this message
					// FIXME : send message to site's parent

					sndmsg := prepareWaveMessages(msg_diffusion_id, RedMsg, timeID, msg_sender, msg_content)
					// send only to parent
					conn := connectedSites[msg_sender]
					_, err := (*conn).Write([]byte(sndmsg))
					if err != nil {
						display_e("Error sending message to " + current_duffusion_status.parent + ": " + err.Error())
						continue
					}
				}

			} else if msg_color == RedMsg {
				current_duffusion_status.nbNeighbors -= 1
				if current_duffusion_status.nbNeighbors == 0 {
					if current_duffusion_status.parent == strconv.Itoa(timeID) {
						// send message to the controleur
						fmt.Println(msg)
					} else {
						// forward the message to the wave initiator
						sndmsg := prepareWaveMessages(msg_diffusion_id, RedMsg, timeID, current_duffusion_status.parent, msg_content)
						// send only to parent
						conn := connectedSites[current_duffusion_status.parent]
						_, err := (*conn).Write([]byte(sndmsg))
						if err != nil {
							display_e("Error sending message to " + current_duffusion_status.parent + ": " + err.Error())
							continue
						}
					}
				}
			}
			mutex.Unlock()
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

		diffusionId := fmt.Sprintf("%d:message_%d", timeID, message_id) // FIXME: add the current port to ID
		diffusionStatus := &DiffusionStatus{
			sender_id:   strconv.Itoa(timeID),
			nbNeighbors: len(connectedSites), // FIXME: shold be repalced by len(connectedSites)
			parent:      strconv.Itoa(timeID),
		}

		DiffusionStatusMap[diffusionId] = diffusionStatus

		sndmsg := prepareWaveMessages(diffusionId, BlueMsg, timeID, "", msg)

		sendWaveMassages(connectedSites, strconv.Itoa(timeID), sndmsg)
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
		numConnections = rng.Intn(maxConnections-minConnections+1) + minConnections
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
		go connectToPeer(targetPort, timeID)
		// Small delay between connections to avoid overwhelming
		time.Sleep(100 * time.Millisecond)
	}
}
