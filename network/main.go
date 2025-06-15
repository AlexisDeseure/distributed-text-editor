package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
var connectedSitesWaitingAdmission = make(map[string]*net.Conn)
var knownSites []string // contains the ids of known sites in the network

// values
const (
	MsgAccessRequest     string = "maq"
	MsgAccessGranted     string = "mag"
	BlueMsg              string = "blu"
	RedMsg               string = "red"
	DiffusionMessage     string = "dif"
	KnownSiteListMessage string = "mks" // Messages list of sites to add to estampille tab
)

// key
const (
	TypeField         string = "typ"
	SiteIdField       string = "sid" // site id of sender
	SiteIdDestField   string = "did" // site id of destination
	DiffusionStatusID string = "dsid"
	ColorDiffusion    string = "clr"
	MessageContent    string = "mct"
	KnownSiteList     string = "ksl" // list of sites to add to estampille tab

)

var (
	id      *string = flag.String("id", "0", "unique id of site (timestamp)") // get the timestamp id from site.sh
	port    *int    = flag.Int("port", 9000, "port of site (default is 9000)")
	targets *string = flag.String("targets", "", "comma-separated list of targets (e.g., 'hostA:portA,hostB:portB')")
	s       int     = 0
	// ip      string  = getLocalIP()
)

func main() {
	flag.Parse()

	// Setup signal handling
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		display_w(fmt.Sprintf("Received signal: %s", sig))
		unregisterAllConns(&connectedSites)
		unregisterAllConns(&connectedSitesWaitingAdmission)
		display_w("All connections unregistered. Exiting.")
		os.Exit(0)
	}()

	targetsList := processTargetFlags(*targets)

	if targetsList == nil {
		display_d("Starting as a primary site, no targets specified.")
		// Listens on its own port
		go startTCPServer()

		// Wait a bit to ensure connections are established
		time.Sleep(1 * time.Second)

		// Read stdin
		go readController()

	} else {
		display_d("Starting as a secondary site, connecting to targets starting with " + targetsList[0])
		for _, addr := range targetsList {
			connectToPeer(addr) // get the ID of the site that has been connected and etablish connection
		}
	}

	time.Sleep(5 * time.Second)
	printConnectedSites()

	// Wait forever
	select {}
}

func startTCPServer() {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		display_e("Server error: " + err.Error())
		return
	}
	display_d("Listening on port " + strconv.Itoa(*port) + "...")

	for {
		conn, err := ln.Accept()
		if err != nil {
			display_e("Accept error: " + err.Error())
			continue
		}
		addr := conn.RemoteAddr().String()
		display_d("New connection from " + addr)
		registerConn(addr, conn, &connectedSitesWaitingAdmission) //MODIFY??
		go readConn(conn, addr)
	}
}

func connectToPeer(addr string) {

	// Avoid connecting to the same peer multiple times
	if isConnected(addr) {
		return
	}

	// Try to connect with retries (for progressive joining)
	maxRetries := 30
	retryDelay := 500 * time.Millisecond
	var conn net.Conn
	var err error
	for attempt := 0; attempt < maxRetries; attempt++ {
		conn, err = net.Dial("tcp", addr)
		if err != nil {
			if attempt == 0 {
				display_w(fmt.Sprintf("Waiting for peer on %s to become available...", addr))
			}
			time.Sleep(retryDelay)
			continue
		}
		display_w("Connected to peer on " + addr)
		break
	}
	// connection established send access request to network
	if conn == nil {
		display_e(fmt.Sprintf("Failed to connect to %s after %d attempts", addr, maxRetries))
		return
	}

	mutex.Lock()
	accessRequestMsg := msg_format(TypeField, MsgAccessRequest) +
		msg_format(SiteIdField, *id)
	writeToConn(conn, accessRequestMsg)
	display_d("Connected to " + addr + ", access request demanded")
	mutex.Unlock()
	// Wait for admission response
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		msg := scanner.Text()
		mutex.Lock()
		msg_type := findval(msg, TypeField, true)
		if msg_type == MsgAccessGranted {
			senderId := findval(msg, SiteIdField, true)
			knownSiteList := findval(msg, KnownSiteList, true)
			//also send the known site of the sender
			display_w("Access granted by " + addr + " (sender ID: " + senderId + ")")
			mutex.Unlock()
			addKnownSite(senderId)

			//add the other site to known sites
			registerConn(senderId, conn, &connectedSites)
			sites := strings.Split(knownSiteList, ",")

			for _, site := range sites {
				if site != "" {
					// add all the known site to the list
					knownSites = append(knownSites, site)
				}
			}

			// Convert known site list into JSON tab
			jsonknownSites, err := json.Marshal(knownSites)
			if err != nil {
				fmt.Println("Erreur JSON :", err)
				return
			}
			stringknownSites := string(jsonknownSites)

			// send the known site list to the controleur
			knownSiteMessage := msg_format(TypeField, KnownSiteListMessage) +
				msg_format(KnownSiteList, stringknownSites) +
				msg_format(SiteIdField, *id)
			fmt.Println(knownSiteMessage)

			go readConn(conn, addr)
			return
		}
	}
}

func readConn(conn net.Conn, addr string) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)

	// Message processing loop
	for scanner.Scan() {
		msg := scanner.Text()
		mutex.Lock()
		msg_type := findval(msg, TypeField, true)

		switch msg_type {
		case MsgAccessRequest:
			senderId := findval(msg, SiteIdField, true)
			display_d("Received access request from " + addr + " (sender ID: " + senderId + ")")
			if len(connectedSites) == 0 {
				// If no connected sites, automatically grant access
				display_w("No connected sites. Automatically granting access to " + addr + " (sender ID: " + senderId + ")")
				mutex.Unlock()
				_ = getAndRemoveConn(addr, &connectedSitesWaitingAdmission)
				registerConn(senderId, conn, &connectedSites)
				addKnownSite(senderId)
				// Send all the known site to the new site of the network
				jsonknownSites, err := json.Marshal(knownSites)
				if err != nil {
					fmt.Println("Erreur JSON :", err)
					return
				}
				stringknownSites := string(jsonknownSites)
				knownSiteMessage := msg_format(TypeField, KnownSiteListMessage) +
					msg_format(KnownSiteList, stringknownSites) +
					msg_format(SiteIdField, *id)
				writeToConn(conn, knownSiteMessage)

				// informer le controleur pour qu'il puisse l'ajouter dans le tableau des horloges
				// default send all the known site to be shure they are known by controleur
				fmt.Println(knownSiteMessage)

				mutex.Lock()
				sndmsg := msg_format(TypeField, MsgAccessGranted) +
					msg_format(SiteIdField, *id)
				writeToConn(conn, sndmsg)
			} else if isKnownSite(senderId) {
				// If the sender is a known site, grant access
				display_w("Granting access to known site " + addr + " (sender ID: " + senderId + ")")
				mutex.Unlock()
				_ = getAndRemoveConn(addr, &connectedSitesWaitingAdmission)
				registerConn(senderId, conn, &connectedSites)
				mutex.Lock()
				sndmsg := msg_format(TypeField, MsgAccessGranted) +
					msg_format(SiteIdField, *id)
				writeToConn(conn, sndmsg)
			} else {
				// todo wave for admission : exemple envoyer le message au controleur qui l'ajoute dans la
				// file d'attente répartie et retourne une demande d'accès à la section critique : quand il l'obtient il pourra renvoyer
				// un message de release avec potentiellement du texte et un nouveau champs iniquant l'ajout du site
			}
		case DiffusionMessage:
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
						sndmsg := prepareWaveMessages(msg_diffusion_id, BlueMsg, *id, "", msg_content)
						sendWaveMessages(connectedSites, msg_sender, sndmsg)
					} else {
						sndmsg := prepareWaveMessages(msg_diffusion_id, RedMsg, *id, msg_sender, msg_content)
						// send only to parent
						conn := connectedSites[msg_sender]
						_, err := writeToConn(*conn, sndmsg)
						if err != nil {
							display_e("Error sending message to " + current_duffusion_status.parent + ": " + err.Error())
							continue
						}

					}

				} else {
					// Has already received this message
					// FIXME : send message to site's parent

					sndmsg := prepareWaveMessages(msg_diffusion_id, RedMsg, *id, msg_sender, msg_content)
					// send only to parent
					conn := connectedSites[msg_sender]
					_, err := writeToConn(*conn, sndmsg)
					if err != nil {
						display_e("Error sending message to " + current_duffusion_status.parent + ": " + err.Error())
						continue
					}
				}

			} else if msg_color == RedMsg {
				current_duffusion_status.nbNeighbors -= 1
				if current_duffusion_status.nbNeighbors == 0 {
					if current_duffusion_status.parent == *id {
						// send message to the controleur
						fmt.Println(msg)
					} else {
						// forward the message to the wave initiator
						sndmsg := prepareWaveMessages(msg_diffusion_id, RedMsg, *id, current_duffusion_status.parent, msg_content)
						// send only to parent
						conn := connectedSites[current_duffusion_status.parent]
						_, err := writeToConn(*conn, sndmsg)
						if err != nil {
							display_e("Error sending message to " + current_duffusion_status.parent + ": " + err.Error())
							continue
						}
					}
				}
			}
		}

		mutex.Unlock()
	}
}

func readController() {
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

		diffusionId := fmt.Sprintf("%s:message_%d", *id, message_id) // FIXME: add the current port to ID
		diffusionStatus := &DiffusionStatus{
			sender_id:   *id,
			nbNeighbors: len(connectedSites), // FIXME: shold be repalced by len(connectedSites)
			parent:      *id,
		}

		DiffusionStatusMap[diffusionId] = diffusionStatus

		sndmsg := prepareWaveMessages(diffusionId, BlueMsg, *id, "", msg)

		sendWaveMessages(connectedSites, *id, sndmsg)
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
}

// func connectToRandomTopology() {
// 	// All sites use the same logic: choose number of connections based on their ID
// 	// Site A (id=0): chooses between 0 and 0 (always 0)
// 	// Site B (id=1): chooses between 1 and 1 (always 1)
// 	// Site C and beyond: choose between 1 and min(id, 3)

// 	// Create a local random generator
// 	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(*id)))

// 	// Determine the range for number of connections
// 	var minConnections, maxConnections int

// 	if *id == 0 {
// 		minConnections = 0
// 		maxConnections = 0
// 	} else {
// 		minConnections = 1
// 		maxConnections = min(*id, 3)
// 	}

// 	// Choose number of connections within the range
// 	var numConnections int
// 	if minConnections == maxConnections {
// 		numConnections = minConnections
// 	} else {
// 		numConnections = rng.Intn(maxConnections-minConnections+1) + minConnections
// 	}

// 	display_w(fmt.Sprintf("Site %d attempting %d connections to existing sites", *id, numConnections))

// 	// If no connections needed, return early
// 	if numConnections == 0 {
// 		display_w(fmt.Sprintf("Site %d starting alone", *id))
// 		return
// 	}

// 	// Create list of existing sites (0 to id-1)
// 	existingSites := make([]int, *id)
// 	for i := 0; i < *id; i++ {
// 		existingSites[i] = i
// 	}

// 	// Shuffle the list and take the first numConnections
// 	for i := len(existingSites) - 1; i > 0; i-- {
// 		j := rng.Intn(i + 1)
// 		existingSites[i], existingSites[j] = existingSites[j], existingSites[i]
// 	}

// 	// Connect to the selected sites
// 	connectionsToMake := numConnections
// 	if connectionsToMake > len(existingSites) {
// 		connectionsToMake = len(existingSites)
// 	}

// 	for i := 0; i < connectionsToMake; i++ {
// 		targetSite := existingSites[i]
// 		targetPort := portBase + targetSite
// 		display_e(fmt.Sprintf("Site %d connecting to Site %d (port %d)", *id, targetSite, targetPort))
// 		go connectToPeer(targetPort, *id)
// 		// Small delay between connections to avoid overwhelming
// 		time.Sleep(100 * time.Millisecond)
// 	}
// }
