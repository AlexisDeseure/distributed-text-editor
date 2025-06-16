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
	message     string
	nbNeighbors int
	parent      string
}

type WaitingObject struct {
	Conn *net.Conn
	Addr string
}
type WaitingMap map[string]*WaitingObject

var mutex = &sync.Mutex{}
var DiffusionStatusMap = make(map[string]*DiffusionStatus)
var connectedSites = make(map[string]*net.Conn)                 // connections which are in the network
var connectedSitesWaitingAdmission = make(map[string]*net.Conn) // connections waiting for admission
var waitingConnections = make(WaitingMap)                       // connections waiting for processing (to be recuperated with both site id and address in the controller reading routine)
var knownSites []string                                         // contains the ids of known sites in the network

// values
const (
	MsgAccessRequest       string = "maq"
	MsgAccessGranted       string = "mag"
	GetSharedText          string = "gst"
	BlueMsg                string = "blu"
	RedMsg                 string = "red"
	DiffusionMessage       string = "dif"
	KnownSiteListMessage   string = "mks" // Messages list of sites to add to estampille tab
	InitializationMessage  string = "ini" // Initialization message to set the initial state
	AddSiteCriticalSection string = "asl" // add site to critical section
	MsgRequestSc           string = "rqs" // request critical section
	MsgReleaseSc           string = "rls" // release critical section
	MsgReceiptSc           string = "rcs" // receipt of critical section
)

// key
const (
	TypeField         string = "typ"
	SiteIdField       string = "sid" // site id of sender
	SiteIdDestField   string = "did" // site id of destination
	DiffusionStatusID string = "dsid"
	ColorDiffusion    string = "clr"
	MessageContent    string = "mct"
	UptField          string = "upt" // update text for the app
	KnownSiteList     string = "ksl" // list of sites to add to estampille tab
	SitesToAdd        string = "sta"
)

var (
	id      *string = flag.String("id", "0", "unique id of site (timestamp)") // get the timestamp id from site.sh
	port    *int    = flag.Int("port", 9000, "port of site (default is 9000)")
	targets *string = flag.String("targets", "", "comma-separated list of targets (e.g., 'hostA:portA,hostB:portB')")
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
		mutex.Lock()
		unregisterAllConns(&connectedSites)
		unregisterAllConns(&connectedSitesWaitingAdmission)
		mutex.Unlock()
		display_w("All connections unregistered. Exiting.")
		os.Exit(0)
	}()

	targetsList := processTargetFlags(*targets)

	if targetsList == nil {
		display_d("Starting as a primary site, no targets specified.")

		// Send the launching message to the controller
		initMessage := msg_format(TypeField, InitializationMessage) +
			msg_format(SiteIdField, "") // convention for reception in app
		fmt.Println(initMessage)

	} else {
		display_d("Starting as a secondary site, connecting to targets starting with " + targetsList[0])
		for _, addr := range targetsList {
			connectToPeer(addr) // get the ID of the site that has been connected and etablish connection
		}
		if len(connectedSites) == 0 {
			display_e("No connections established. Exiting.")
			os.Exit(1)
		}

	}

	// Listens on its own port
	go startTCPServer()
	// Wait a bit to ensure connections are established
	time.Sleep(1 * time.Second)

	go readController()

	// Wait forever
	select {}
}

func startTCPServer() {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		display_e("Server error: " + err.Error())
		os.Exit(1)
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
		mutex.Lock()
		registerConn(addr, conn, &connectedSitesWaitingAdmission)
		mutex.Unlock()
		go readConn(conn, addr)
	}
}

func connectToPeer(addr string) {

	// Avoid connecting to the same peer multiple times
	mutex.Lock() // we mutex.Lock() to ensure read safety
	if isConnected(addr) {
		mutex.Unlock()
		return
	}
	mutex.Unlock()

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
			originalText := findval(msg, UptField, true)
			//also add the known site of the sender
			if knownSiteList == "" { //correspond to case 2 : current site is already in the network
				// so we already have the shared text and the known sites of the network
				display_d("Already in the network, access granted by a new connection " + addr + " (sender ID: " + senderId + ")")
			} else { // case 1 or 3 : we are a new site in the network
				display_d("Access granted to the network by " + addr + " (sender ID: " + senderId + ")")
				addKnownSite(senderId) //add the other site to known sites
				sites := []string{}    // initialize the list of sites
				err := json.Unmarshal([]byte(knownSiteList), &sites)
				if err != nil {
					display_e("Erreur JSON :" + err.Error())
					return
				}
				for _, site := range sites {
					if site != "" {
						// add all the known site to the list
						addKnownSite(site)
					}
				}
				// Convert known site list into JSON tab
				jsonknownSites, err := json.Marshal(knownSites)
				if err != nil {
					display_e("Erreur JSON :" + err.Error())
					return
				}
				stringknownSites := string(jsonknownSites)
				// send the known site list to the controleur
				initMessage := msg_format(TypeField, InitializationMessage) +
					msg_format(KnownSiteList, stringknownSites) +
					msg_format(SiteIdField, *id) +
					msg_format(UptField, originalText)
				fmt.Println(initMessage)
			}
			registerConn(senderId, conn, &connectedSites)
			mutex.Unlock()
			go readConn(conn, addr)
			return
		}
		mutex.Unlock()
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
			if len(connectedSites) == 0 { // case 1 : solo primary site
				// If no connected sites, automatically grant access
				display_d("No connected sites. Automatically granting access to " + addr + " (sender ID: " + senderId + ") : waiting for application to send the shared text")
				addWaitingSiteMap(senderId, &conn, addr)
				getCurrentSharedTextMsg := msg_format(TypeField, GetSharedText) +
					msg_format(SiteIdField, senderId) // hear we pass the senderId to the new site to get it again when obtaining the text
				fmt.Println(getCurrentSharedTextMsg)

			} else if isKnownSite(senderId) { // case 2 : known site : it is already in the network and have the shared text
				// If the sender is a known site, grant access
				display_d("Granting access to known site " + addr + " (sender ID: " + senderId + ")")
				_ = getAndRemoveConn(addr, &connectedSitesWaitingAdmission)
				registerConn(senderId, conn, &connectedSites)
				sndmsg := msg_format(TypeField, MsgAccessGranted) +
					msg_format(SiteIdField, *id)
				writeToConn(conn, sndmsg)
			} else { // case 3 : classic admission
				// If the sender is not known and there are connected sites, add it to the waiting list in controller to wait for admission
				// using the critical section protocol
				display_d("Waiting for admission of " + addr + " by the network (sender ID: " + senderId + ")")
				addWaitingSiteMap(senderId, &conn, addr)
				sndmsg := msg_format(TypeField, AddSiteCriticalSection) +
					msg_format(SiteIdField, senderId)
				fmt.Println(sndmsg) // send the message to the controleur to add the site in the critical section
			}

		case DiffusionMessage:

			msg_diffusion_id := findval(msg, DiffusionStatusID, false)
			msg_color := findval(msg, ColorDiffusion, false)
			msg_content := findval(msg, MessageContent, false)
			senderID := findval(msg, SiteIdField, false)
			formated_msg_content, _ := jsonToMsg(msg_content, true)
			current_diffusion_status := DiffusionStatusMap[msg_diffusion_id]

			if current_diffusion_status == nil {
				current_diffusion_status := &DiffusionStatus{
					message:     formated_msg_content,
					nbNeighbors: len(connectedSites),
					parent:      "",
				}
				DiffusionStatusMap[msg_diffusion_id] = current_diffusion_status

			}

			if msg_color == BlueMsg {
				display_d("Received blue message from " + senderID + " with content: " + formated_msg_content)
				if current_diffusion_status.parent == "" {
					// send message to the controleur + treat it if it is a MsgReleaseSc
					msg_initial_type := findval(formated_msg_content, TypeField, true)
					if msg_initial_type == MsgReleaseSc {
						// if there is sites added to the network, we need to add them to known sites and inform
						// the controller
						sitesToAdd := findval(formated_msg_content, SitesToAdd, true)
						sitesToAddList := []string{} // list of sites to add to the network
						err := json.Unmarshal([]byte(sitesToAdd), &sitesToAddList)
						if err != nil {
							display_e("JSON decoding error for sitesToAdd: " + err.Error())
							continue
						}
						if len(sitesToAddList) > 0 { // we have sites to add to the network
							for _, site := range sitesToAddList {
								addKnownSite(site) // add the site to the known sites
							}
							jsonknownSites, err := json.Marshal(knownSites)
							if err != nil {
								display_e("Erreur JSON :" + err.Error())
								return
							}
							// Send the new known sites to the controller to update his clock map
							stringknownSites := string(jsonknownSites)
							knownSiteMessage := msg_format(TypeField, KnownSiteListMessage) +
								msg_format(KnownSiteList, stringknownSites) +
								msg_format(SiteIdField, *id)
							fmt.Println(knownSiteMessage)
							display_d("New sites added to the network by " + senderID + " : " + strings.Join(sitesToAddList, ", "))
						}
					}
					fmt.Println(formated_msg_content) // transfer the message to the controller without the diffusion elements

					// update diffusion status
					current_diffusion_status.parent = senderID
					current_diffusion_status.nbNeighbors -= 1

					if current_diffusion_status.nbNeighbors > 0 {
						sndmsg := prepareWaveMessages(msg_diffusion_id, BlueMsg, formated_msg_content)
						sendWaveMessages(connectedSites, senderID, sndmsg)
						display_d("Forwarding blue message to neighbors, except the sender: " + senderID)
					} else {
						sndmsg := prepareWaveMessages(msg_diffusion_id, RedMsg, formated_msg_content)
						// send only to parent (the sender of the message)
						conn := connectedSites[senderID]
						_, err := writeToConn(*conn, sndmsg)
						if err != nil {
							display_e("Error sending message to " + current_diffusion_status.parent + ": " + err.Error())
							continue
						}
						display_d("No more neighbors to forward the blue message, sending red message to parent: " + current_diffusion_status.parent)
					}
				} else {
					// Has already received blue message for this diffusion : sites aren't related
					sndmsg := prepareWaveMessages(msg_diffusion_id, RedMsg, formated_msg_content)
					conn := connectedSites[senderID]
					_, err := writeToConn(*conn, sndmsg)
					if err != nil {
						display_e("Error sending message to " + current_diffusion_status.parent + ": " + err.Error())
						continue
					}
					display_d("Already received blue message for this diffusion, sending red message to sender: " + senderID)
				}

			} else if msg_color == RedMsg {
				current_diffusion_status.nbNeighbors -= 1
				if current_diffusion_status.nbNeighbors <= 0 {
					if current_diffusion_status.parent == *id {
						// send message to the controleur
						fmt.Println(formated_msg_content)
						display_d("END of diffusion for message ID " + msg_diffusion_id)
					} else {
						// forward the message to the wave initiator by passsing it to the parent
						sndmsg := prepareWaveMessages(msg_diffusion_id, RedMsg, formated_msg_content)
						// send only to parent
						conn := connectedSites[current_diffusion_status.parent]
						_, err := writeToConn(*conn, sndmsg)
						if err != nil {
							display_e("Error sending message to " + current_diffusion_status.parent + ": " + err.Error())
							continue
						}
						display_d("No more neighbors from which to receive the red message, forwarding to parent: " + current_diffusion_status.parent)
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
			// display_e("Error reading message : " + err.Error())
			continue
		}
		msg = strings.TrimSuffix(msg, "\n")

		mutex.Lock()
		rcvtype := findval(msg, TypeField, true)

		switch rcvtype {
		case GetSharedText: // The demand for the current shared text has been received (case 1)
			text := findval(msg, UptField, true)
			sitesToAdd := findval(msg, SitesToAdd, true)
			sitesToAddList := []string{} // list of sites to add to the network
			err := json.Unmarshal([]byte(sitesToAdd), &sitesToAddList)
			if err != nil {
				display_e("JSON decoding error for sitesToAdd: " + err.Error())
				continue
			}
			for _, site := range sitesToAddList {
				addKnownSite(site)
			}
			// Send the new known sites to the controller
			jsonknownSites, err := json.Marshal(knownSites)
			if err != nil {
				display_e("Erreur JSON :" + err.Error())
				return
			}
			stringknownSites := string(jsonknownSites)
			knownSiteMessage := msg_format(TypeField, KnownSiteListMessage) +
				msg_format(KnownSiteList, stringknownSites) +
				msg_format(SiteIdField, *id)
			// default send all the known site to be shure they are known by controleur to add it in his clock map
			fmt.Println(knownSiteMessage)

			for _, site := range sitesToAddList {
				addr := waitingConnections[site].Addr
				conn := *waitingConnections[site].Conn
				delete(waitingConnections, site) // remove the waiting connection
				_ = getAndRemoveConn(addr, &connectedSitesWaitingAdmission)
				registerConn(site, conn, &connectedSites)
				sndmsg := msg_format(TypeField, MsgAccessGranted) +
					msg_format(SiteIdField, *id) + // we send our id to the site which asked to join the network
					msg_format(KnownSiteList, stringknownSites) + // Send all the known sites to the new sites of the network
					msg_format(UptField, text)
				writeToConn(conn, sndmsg)
			}

		default:
			if rcvtype == MsgReleaseSc || rcvtype == MsgReceiptSc || rcvtype == MsgRequestSc {
				// Push the critical section message to the network (if any with more site than only the primary site)
				// using the diffusion protocol
				if len(connectedSites) == 0 {
					display_d("No connected sites to send the message: " + msg)
					// return the message to the controller
					fmt.Println(msg)
				} else {
					count := len(DiffusionStatusMap)
					diffusionId := fmt.Sprintf("%s:message_%d", *id, count)
					diffusionStatus := &DiffusionStatus{
						message:     msg,
						nbNeighbors: len(connectedSites),
						parent:      *id,
					}
					DiffusionStatusMap[diffusionId] = diffusionStatus
					sndmsg := prepareWaveMessages(diffusionId, BlueMsg, msg)
					sendWaveMessages(connectedSites, *id, sndmsg) // we send to all neighbors (sender id is current id by convention)
				}
			}
			// display_d("Received message from controller: " + msg)

		}
		mutex.Unlock()
	}
}
