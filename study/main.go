package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DiffusionStatus struct {
	id           string
	nbNeighbors int
	parent       string
}

var mutex = &sync.Mutex{}
var connectedSites = make(map[string]*net.UDPAddr)
var DiffusionStatusMap = make(map[string]*DiffusionStatus)

// var sitesMutex = &sync.RWMutex{}
var logger *log.Logger
var site_id_from_others string

var (
	fieldsep  = "~"
	keyvalsep = "`"
)

const (
	MsgAccessRequest string = "maq"
	MsgAccessGranted string = "mag"
	DiffusionMessage string = "dif"
	OtherMsgPrefix   string = "oth"
	BlueMsg          string = "blu"
	RedMsg           string = "red"
)
const (
	TypeField      string = "typ"
	ContentField   string = "con"
	SenderId       string = "sid"
	DiffusionType  string = "dit"
	ColorDiffusion string = "clr"
)

// msg_format constructs a key-value string using predefined separators
func msg_format(key string, val string) string {
	return fieldsep + keyvalsep + key + keyvalsep + val
}

// findval searches a formatted message for a given key and returns its value
func findval(msg string, key string, verbose bool) string {
	if len(msg) < 4 {
		if verbose {
			logger.Printf("[Error] Message length too short: %s", msg)
		}
		return ""
	}

	sep := msg[0:1]
	tab_allkeyvals := strings.Split(msg[1:], sep)

	for _, keyval := range tab_allkeyvals {

		if len(keyval) < 4 {
			continue
		}

		equ := keyval[0:1]
		tabkeyval := strings.Split(keyval[1:], equ)
		if tabkeyval[0] == key {
			return tabkeyval[1]
		}
	}
	if verbose {
		err_msg := fmt.Sprintf("Key %s not found in message", key)
		logger.Printf("[Warning] %s\n", err_msg)
	}
	return ""
}

// Once admitted, send messages periodically
func sendPeriodic(id string, conn *net.UDPConn) {
	i := 0
	for {
		if len(connectedSites) > 0 {
			i++
			mutex.Lock()

			content := fmt.Sprintf("message_%d", i)

			diffusionId := fmt.Sprintf("%s:%s", site_id_from_others, content)

			diffusionStatus := &DiffusionStatus{
				id:           site_id_from_others,
				nbNeighbors: len(connectedSites),
				parent:       site_id_from_others,
			}

			DiffusionStatusMap[diffusionId] = diffusionStatus

			sndmsg := msg_format(TypeField, DiffusionMessage) +
				msg_format(DiffusionType, OtherMsgPrefix) +
				msg_format(SenderId, site_id_from_others) + 
				msg_format(ContentField, diffusionId) +
				msg_format(ColorDiffusion, BlueMsg)

			nb_send := len(connectedSites)
			for _, addr := range connectedSites {
				_, err := conn.WriteToUDP([]byte(sndmsg), addr)
				if err != nil {
					logger.Printf("[%s] ERROR: failed to send diffusion message to %s: %v\n", id, addr, err)
					nb_send--
				}
			}

			logger.Printf("[%s] DIFFUSION: %s to %d neighbors\n", id, diffusionId, nb_send)

			mutex.Unlock()

			time.Sleep(30 * time.Second)
		}
	}
}

func pluralize(sentCount int) string {
	if sentCount < 2 {
		return ""
	}
	return "s"
}

// Handle a single incoming message
func processMessage(id, msg string, admittedCh chan<- bool, logger *log.Logger, conn *net.UDPConn, senderAddr *net.UDPAddr) {
	msg_type := findval(msg, TypeField, true)

	switch msg_type {
	case MsgAccessRequest:
		logger.Printf("[%s] RECEIVED: admission request from %s\n", id, senderAddr)

		if site_id_from_others == "" {
			site_id_from_others = findval(msg, SenderId, true)
		}

		if len(connectedSites) == 0 {
			response := msg_format(TypeField, MsgAccessGranted) +
				msg_format(SenderId, senderAddr.String()) // it is not the real sender id hear but the id of the site who ask for admission

			conn.WriteToUDP([]byte(response), senderAddr)
			// Extract the site ID from the sender's address for tracking
			// For simplicity, we'll use the full address as the key
			siteKey := senderAddr.String()
			connectedSites[siteKey] = senderAddr
			logger.Printf("[%s] REGISTERED: site at %s\n", id, siteKey)

		} else {
			// Use a wave diffusion algorithm to handle admission requests and inform existing sites
			// It corresponds to the initiator : start of the diffusion
			diffusionId := fmt.Sprintf("%s:%s->%s", MsgAccessRequest, senderAddr.String(), site_id_from_others)

			diffusionStatus := &DiffusionStatus{
				id:           senderAddr.String(),
				nbNeighbors: len(connectedSites),
				parent:       site_id_from_others,
			}

			DiffusionStatusMap[diffusionId] = diffusionStatus

			sndmsg := msg_format(TypeField, DiffusionMessage) +
				msg_format(DiffusionType, MsgAccessRequest) +
				msg_format(SenderId, senderAddr.String()) + // Use the sender's address as the sender ID : the site who ask for admission
				msg_format(ContentField, diffusionId) +
				msg_format(ColorDiffusion, BlueMsg)

			nb_send := len(connectedSites)
			for _, addr := range connectedSites {
				_, err := conn.WriteToUDP([]byte(sndmsg), addr)
				if err != nil {
					logger.Printf("[%s] ERROR: failed to send diffusion message to %s: %v\n", id, addr, err)
					nb_send--
				}
			}

			logger.Printf("[%s] DIFFUSION: %s to %d neighbors with status\n", id, diffusionId, nb_send)
		}

	case MsgAccessGranted:
		logger.Printf("[%s] RECEIVED: admission permitted\n", id)

		// Extract site ID from the message
		site_id_from_others = findval(msg, SenderId, true)
		// Try to send to admittedCh, but don't block if no one is listening
		select {
		case admittedCh <- true:
			logger.Printf("[%s] JOINED: network successfully with address %s\n", id, site_id_from_others)
		default:
			// This case is hit if admittedCh is not ready to receive (e.g., already received or closed)
			// This prevents blocking if multiple "admission permitted" messages are received.
			logger.Printf("[%s] INFO: Admission signal to channel was not sent (possibly already admitted or channel closed) for address %s\n", id, site_id_from_others)
		}

	case DiffusionMessage:
		diffusionType := findval(msg, DiffusionType, true)
		diffusionId := findval(msg, ContentField, true)
		color := findval(msg, ColorDiffusion, true)

		if _, exists := DiffusionStatusMap[diffusionId]; !exists {
			diffusionStatus := &DiffusionStatus{
				id:           findval(msg, SenderId, true),
				nbNeighbors:  len(connectedSites),
				parent:       "",
			}
			DiffusionStatusMap[diffusionId] = diffusionStatus
			logger.Printf("[%s] NEW DIFFUSION: %s\n", id, diffusionId)
		}

		switch color {
		case BlueMsg:
			switch diffusionType {
			case OtherMsgPrefix:
				logger.Printf("[%s] RECEIVED: blue message %s from %s\n", id, findval(msg, ContentField, true), senderAddr)
			case MsgAccessRequest:
				logger.Printf("[%s] RECEIVED: blue message for diffusion type %s from %s\n", id, diffusionType, senderAddr)
			}

			if DiffusionStatusMap[diffusionId].parent == "" {
				// This is the first message, so we set the parent to the sender
				DiffusionStatusMap[diffusionId].parent = senderAddr.String()
				DiffusionStatusMap[diffusionId].nbNeighbors -= 1
				if DiffusionStatusMap[diffusionId].nbNeighbors > 0 {
					// Forward the message to all connected sites except the sender
					nb_forwarded := 0
					for _, addr := range connectedSites {
						if addr.String() != senderAddr.String() {
							_, err := conn.WriteToUDP([]byte(msg), addr)
							if err != nil {
								logger.Printf("[%s] ERROR: failed to forward diffusion %s to %s: %v\n", id, diffusionId, addr, err)
							} else {
								nb_forwarded++
							}
						}
					}
					logger.Printf("[%s] FORWARDED: admission request to %d neighbors\n", id, nb_forwarded)
				} else {
					// Send a red message to the sender which is the parent of this site for this diffusion
					response := msg_format(TypeField, DiffusionMessage) +
						msg_format(DiffusionType, MsgAccessRequest) +
						msg_format(SenderId, DiffusionStatusMap[diffusionId].id) +
						msg_format(ContentField, diffusionId) +
						msg_format(ColorDiffusion, RedMsg)
					_, err := conn.WriteToUDP([]byte(response), senderAddr)
					if err != nil {
						logger.Printf("[%s] ERROR: failed to send red message to %s: %v\n", id, senderAddr, err)
					} else {
						logger.Printf("[%s] SENT: red message to %s indicating no more neighbors to forward diffusion\n", id, senderAddr)
						// Delete the diffusion status since we are done with it
						delete(DiffusionStatusMap, diffusionId)
					}
				}
			} else {
				// Send a red message to the sender which is not the parent of this site for this diffusion
				response := msg_format(TypeField, DiffusionMessage) +
					msg_format(DiffusionType, MsgAccessRequest) +
					msg_format(SenderId, DiffusionStatusMap[diffusionId].id) +
					msg_format(ContentField, diffusionId) +
					msg_format(ColorDiffusion, RedMsg)
				_, err := conn.WriteToUDP([]byte(response), senderAddr)
				if err != nil {
					logger.Printf("[%s] ERROR: failed to send red message to %s: %v\n", id, senderAddr, err)
				} else {
					logger.Printf("[%s] SENT: red message to %s indicating no more neighbors to forward admission request\n", id, senderAddr)
				}
			}
		case RedMsg:
			logger.Printf("[%s] RECEIVED: red message for diffusion type %s from %s\n", id, diffusionType, senderAddr)

			DiffusionStatusMap[diffusionId].nbNeighbors -= 1
			if DiffusionStatusMap[diffusionId].nbNeighbors <= 0 {
				if DiffusionStatusMap[diffusionId].parent == site_id_from_others {
					// This means we are the parent of this diffusion, so we can admit the site
					// it is the explicit termination of the admission request
					switch diffusionType {
					case MsgAccessRequest:
						response := msg_format(TypeField, MsgAccessGranted) +
						msg_format(SenderId, DiffusionStatusMap[diffusionId].id) // it is not the real sender id hear but the id of the site who ask for admission
						targetAddr, err := net.ResolveUDPAddr("udp", DiffusionStatusMap[diffusionId].id)
						if err != nil {
							logger.Printf("[%s] ERROR: failed to resolve address %s: %v\n", id, DiffusionStatusMap[diffusionId].id, err)
						} else {
							conn.WriteToUDP([]byte(response), targetAddr)
							// Extract the site ID from the sender's address for tracking
							// For simplicity, we'll use the full address as the key
							connectedSites[DiffusionStatusMap[diffusionId].id] = targetAddr
							logger.Printf("[%s] REGISTERED: site at %s\n", id, DiffusionStatusMap[diffusionId].id)
						}
					case OtherMsgPrefix:
						logger.Printf("[%s] All sites have received the message id %s, no more neighbors to forward it\n", id, diffusionId)
					}
					// Delete the diffusion status since we are done with it
					delete(DiffusionStatusMap, diffusionId)
					
				} else {
					// Send a red message to the parent indicating no more neighbors to forward admission request
					response := msg_format(TypeField, DiffusionMessage) +
						msg_format(DiffusionType, MsgAccessRequest) +
						msg_format(SenderId, DiffusionStatusMap[diffusionId].id) +
						msg_format(ContentField, diffusionId) +
						msg_format(ColorDiffusion, RedMsg)

					parentAddr, err := net.ResolveUDPAddr("udp", DiffusionStatusMap[diffusionId].parent)
					if err != nil {
						logger.Printf("[%s] ERROR: failed to resolve address %s: %v\n", id, DiffusionStatusMap[diffusionId].parent, err)
					} else {
						conn.WriteToUDP([]byte(response), parentAddr)
						logger.Printf("[%s] SENT: red message to %s indicating no more neighbors to forward admission request\n", id, DiffusionStatusMap[diffusionId].parent)
						// Delete the diffusion status since we are done with it
						delete(DiffusionStatusMap, diffusionId)
					}
				}
			}
		}

	// default: // to remove in the future
	// 	logger.Printf("[%s] RECEIVED: %s from %s\n", id, msg, senderAddr)
	// 	// If the message is not an admission request or response, we assume it's a broadcast message
	// 	// and we forward it to all connected sites except the sender
	// 	senderId := findval(msg, SenderId, true)

	// 	if senderId == site_id_from_others {
	// 		// This is a message from us, so we ignore it
	// 		logger.Printf("[%s] IGNORED: message from self (%s)\n", id, senderAddr)
	// 		return
	// 	}

	// 	forwardCount := 0
	// 	for _, addr := range connectedSites {
	// 		// Don't send back to the sender
	// 		if addr.String() != senderAddr.String() {
	// 			conn.WriteToUDP([]byte(msg), addr)
	// 			forwardCount++
	// 		}
	// 	}
	// 	if forwardCount > 0 {
	// 		logger.Printf("[%s] FORWARDED: message to %d sites\n", id, forwardCount)
	// 	}
	}
}

// Site listens for UDP messages
func handleReceive(id string, admittedCh chan<- bool, conn *net.UDPConn) {
	buffer := make([]byte, 1024)

	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		mutex.Lock()

		if err != nil {
			logger.Printf("[%s] ERROR: reading UDP message: %v\n", id, err)
			continue
		}

		msg := string(buffer[:n])

		processMessage(id, msg, admittedCh, logger, conn, addr)
		mutex.Unlock()
	}
}

func processTargetFlags(id string, targetHostsStr string, targetPortsStr string, logger *log.Logger) ([]*net.UDPAddr, error) {
	var targetHostList []string
	if targetHostsStr != "" {
		targetHostList = strings.Split(targetHostsStr, ",")
	}

	var targetPortStrList []string
	if targetPortsStr != "" {
		targetPortStrList = strings.Split(targetPortsStr, ",")
	}

	var targetAddrs []*net.UDPAddr

	if len(targetHostList) > 0 || len(targetPortStrList) > 0 {
		if len(targetHostList) != len(targetPortStrList) {
			return nil, fmt.Errorf("[%s] ERROR: -target-hosts and -target-ports must have the same number of entries", id)
		}
		if len(targetHostList) == 0 { // Should be caught by the above, but as a safeguard
			return nil, fmt.Errorf("[%s] ERROR: If -target-ports is specified, -target-hosts must also be specified, and vice-versa", id)
		}

		targetPortList := make([]int, len(targetPortStrList))
		for i, pStr := range targetPortStrList {
			portVal, err := strconv.Atoi(strings.TrimSpace(pStr))
			if err != nil {
				return nil, fmt.Errorf("[%s] ERROR: invalid port in -target-ports list '%s': %v", id, pStr, err)
			}
			targetPortList[i] = portVal
		}

		uniqueTargets := make(map[string]struct{}) // To store "host:port" strings for deduplication

		for i, hStr := range targetHostList {
			host := strings.TrimSpace(hStr)
			p := targetPortList[i]
			targetSpec := fmt.Sprintf("%s:%d", host, p)

			if _, exists := uniqueTargets[targetSpec]; exists {
				logger.Printf("[%s] INFO: Duplicate target %s specified, ignoring.\n", id, targetSpec)
				continue
			}

			addr, err := net.ResolveUDPAddr("udp", targetSpec)
			if err != nil {
				logger.Printf("[%s] WARNING: resolving target address %s failed: %v. Skipping this target.\n", id, targetSpec, err)
				continue // Skip this target
			}
			targetAddrs = append(targetAddrs, addr)
			uniqueTargets[targetSpec] = struct{}{}
			logger.Printf("[%s] INFO: Added target peer %s for connection attempt.\n", id, addr.String())
		}

		if len(targetHostList) > 0 && len(targetAddrs) == 0 {
			return nil, fmt.Errorf("[%s] ERROR: No valid target peers to connect to after processing inputs (all specified targets were duplicates or failed to resolve)", id)
		}
	}
	return targetAddrs, nil
}

func handleSecondarySiteAdmission(id string, conn *net.UDPConn, targetAddrs []*net.UDPAddr, admittedCh chan bool, logger *log.Logger) {
	logger.Printf("[%s] STARTED: as secondary site, attempting to join network via %d peer(s)\n", id, len(targetAddrs))

	stopRequestingCh := make(chan struct{})

	go func() {
		for {
			select {
			case <-stopRequestingCh:
				logger.Printf("[%s] INFO: Stopping admission requests as admission has been granted.\n", id)
				return
			default:
				mutex.Lock()
				message := msg_format(TypeField, MsgAccessRequest)
				activeTargets := 0
				for _, tAddr := range targetAddrs {
					message += msg_format(SenderId, tAddr.String()) // Use the target address as the sender ID
					// for the request to help primary site to have his real id (because we use wsl the ip isn't the same as the device so a
					// site can get his real ip address)
					_, err := conn.WriteToUDP([]byte(message), tAddr)
					if err != nil {
						logger.Printf("[%s] ERROR: sending admission request to %s: %v\n", id, tAddr.String(), err)
					} else {
						activeTargets++
					}
				}
				mutex.Unlock()
				if activeTargets > 0 {
					logger.Printf("[%s] SENT: admission requests to %d target peer(s).\n", id, activeTargets)
				} else {
					logger.Printf("[%s] WARNING: No active targets to send admission requests to.\n", id)
				}
				time.Sleep(15 * time.Second) // Interval between request bursts
			}
		}
	}()

	// Wait for admission from any of the target peers
	<-admittedCh
	close(stopRequestingCh) // Signal the request goroutine to stop

	// Once admitted, register all initial target sites in our connected sites
	mutex.Lock()
	logger.Printf("[%s] REGISTERING: all %d initial target peers.\n", id, len(targetAddrs))
	for _, tAddr := range targetAddrs {
		siteKey := tAddr.String()
		if _, exists := connectedSites[siteKey]; !exists {
			connectedSites[siteKey] = tAddr
			logger.Printf("[%s] REGISTERED: initial target peer %s\n", id, siteKey)
		} else {
			logger.Printf("[%s] INFO: Initial target peer %s was already registered (possibly the admitting peer or added concurrently).\n", id, siteKey)
		}
	}
	mutex.Unlock()

	go sendPeriodic(id, conn) // Start sending periodic messages
	select {}                 // Keep main goroutine alive
}

func main() {
	idFlag := flag.String("id", "0", "site ID (ex: A or B)")
	portFlag := flag.Int("port", 0, "UDP port to listen on")
	targetHostsStrFlag := flag.String("target-hosts", "", "comma-separated list of target hosts (e.g., 'hostA,hostB')")
	targetPortsStrFlag := flag.String("target-ports", "", "comma-separated list of target ports (e.g., '8001,8002')")
	flag.Parse()

	// Initialize global logger
	logger = log.New(os.Stderr, "", 0)

	if *portFlag == 0 {
		logger.Printf("[ERROR] -port parameter is required\n")
		logger.Printf("[ERROR] Usage: -port <listen_port> [-target-hosts <hosts>] [-target-ports <ports>]\n")
		os.Exit(1)
	}

	targetAddrs, err := processTargetFlags(*idFlag, *targetHostsStrFlag, *targetPortsStrFlag, logger)
	if err != nil {
		logger.Printf("%v\n", err)
		os.Exit(1)
	}

	// Create UDP connection
	listenAddr, err := net.ResolveUDPAddr("udp", ":"+strconv.Itoa(*portFlag))
	if err != nil {
		logger.Printf("[%s] ERROR: resolving listen address: %v\n", *idFlag, err)
		os.Exit(1)
	}

	conn, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		logger.Printf("[%s] ERROR: creating UDP listener: %v\n", *idFlag, err)
		os.Exit(1)
	}
	defer conn.Close()

	logger.Printf("[%s] LISTENING: on UDP port %d\n", *idFlag, *portFlag)

	admittedCh := make(chan bool)

	go handleReceive(*idFlag, admittedCh, conn)

	if len(targetAddrs) == 0 {
		// This is the primary site : it starts the network and waits for connections
		logger.Printf("[%s] STARTED: as primary site\n", *idFlag)
		mutex.Lock()
		site_id_from_others = "" // do not know the id yet, it will be set when the first admission request is received
		mutex.Unlock()
		go sendPeriodic(*idFlag, conn)
		select {}
	} else {
		handleSecondarySiteAdmission(*idFlag, conn, targetAddrs, admittedCh, logger)
	}
}
