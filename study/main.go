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

var mutex = &sync.Mutex{}
var connectedSites = make(map[string]*net.UDPAddr)
var sitesMutex = &sync.RWMutex{}
var logger *log.Logger

// Once admitted, send messages periodically
func sendPeriodic(id string, conn *net.UDPConn, targetAddr *net.UDPAddr) {
	i := 0
	for {
		i++
		message := fmt.Sprintf("message_%d<%s>", i, id)

		// Sites can broadcast messages to all connected sites in the network
		sitesMutex.RLock()
		sentCount := 0
		for siteID, addr := range connectedSites {
			conn.WriteToUDP([]byte(message), addr)
			logger.Printf("[%s] BROADCAST: %s to %s\n", id, message, siteID)
			sentCount++
		}
		sitesMutex.RUnlock()

		// Sites can also send messages to a specific target
		if targetAddr != nil {
			conn.WriteToUDP([]byte(message), targetAddr)
			logger.Printf("[%s] SENT: %s to target\n", id, message)
			sentCount++
		}

		// Print locally for debugging
		mutex.Lock()
		logger.Printf("[%s] LOCAL: %s (sent to %d site%s)\n", id, message, sentCount, pluralize(sentCount))
		mutex.Unlock()

		time.Sleep(2 * time.Second)
	}
}

func pluralize(sentCount int) string {
	if sentCount < 2 {
		return ""
	}
	return "s"
}

// Handle a single incoming message
func processMessage(id, target, msg string, admittedCh chan<- bool, logger *log.Logger, conn *net.UDPConn, senderAddr *net.UDPAddr) {
	switch msg {
	case "admission request":
		logger.Printf("[%s] RECEIVED: admission request from %s\n", id, senderAddr)
		response := "admission permitted"
		if senderAddr != nil {
			conn.WriteToUDP([]byte(response), senderAddr)
			// For site A, register the connecting site
			if target == "" {
				// Extract the site ID from the sender's address for tracking
				// For simplicity, we'll use the full address as the key
				siteKey := senderAddr.String()
				sitesMutex.Lock()
				connectedSites[siteKey] = senderAddr
				sitesMutex.Unlock()
				logger.Printf("[%s] REGISTERED: site at %s\n", id, siteKey)
			}
		} else {
			logger.Printf("[%s] LOCAL: %s\n", id, response)
		}
	case "admission permitted":
		logger.Printf("[%s] RECEIVED: admission permitted\n", id)
		if target != "" {
			logger.Printf("[%s] JOINED: network successfully\n", id)
			// Signal admission
			admittedCh <- true
		}
	default:
		if strings.HasPrefix(msg, id+":") {
			// This is our own message echoed back, ignore it
			return
		}
		logger.Printf("[%s] RECEIVED: %s\n", id, msg)

		// Site A should forward messages to all other connected sites
		if target == "" && senderAddr != nil {
			sitesMutex.RLock()
			forwardCount := 0
			for _, addr := range connectedSites {
				// Don't send back to the sender
				if addr.String() != senderAddr.String() {
					conn.WriteToUDP([]byte(msg), addr)
					forwardCount++
				}
			}
			sitesMutex.RUnlock()
			if forwardCount > 0 {
				logger.Printf("[%s] FORWARDED: message to %d sites\n", id, forwardCount)
			}
		}
	}
}

// Site listens for UDP messages
func handleReceive(id string, target string, admittedCh chan<- bool, conn *net.UDPConn) {
	buffer := make([]byte, 1024)

	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			logger.Printf("[%s] ERROR: reading UDP message: %v\n", id, err)
			continue
		}

		msg := string(buffer[:n])

		// For site A, we need to know the target address for responses
		var targetAddr *net.UDPAddr
		if target == "" {
			targetAddr = addr // Reply to sender
		}

		processMessage(id, target, msg, admittedCh, logger, conn, targetAddr)
	}
}

func main() {
	id := flag.String("id", "", "site ID (ex: A or B)")
	port := flag.Int("port", 0, "UDP port to listen on")
	targetHost := flag.String("target-host", "localhost", "target host (default: localhost)")
	targetPort := flag.Int("target-port", 0, "target port to connect to")
	flag.Parse()

	// Initialize global logger
	logger = log.New(os.Stderr, "", 0)

	if *id == "" || *port == 0 {
		logger.Printf("[ERROR] -id and -port parameters are required\n")
		logger.Printf("[ERROR] Usage: -id <site_id> -port <listen_port> [-target-host <host>] [-target-port <port>]\n")
		os.Exit(1)
	}

	// Create UDP connection
	listenAddr, err := net.ResolveUDPAddr("udp", ":"+strconv.Itoa(*port))
	if err != nil {
		logger.Printf("[%s] ERROR: resolving listen address: %v\n", *id, err)
		os.Exit(1)
	}

	conn, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		logger.Printf("[%s] ERROR: creating UDP listener: %v\n", *id, err)
		os.Exit(1)
	}
	defer conn.Close()

	logger.Printf("[%s] LISTENING: on UDP port %d\n", *id, *port)

	admittedCh := make(chan bool)

	// Determine target address if specified
	var targetAddr *net.UDPAddr
	var target string
	if *targetPort != 0 {
		target = "A" // Assume we're connecting to site A
		targetAddr, err = net.ResolveUDPAddr("udp", *targetHost+":"+strconv.Itoa(*targetPort))
		if err != nil {
			logger.Printf("[%s] ERROR: resolving target address: %v\n", *id, err)
			os.Exit(1)
		}
		logger.Printf("[%s] CONNECTING: to %s\n", *id, targetAddr)
	}

	go handleReceive(*id, target, admittedCh, conn)

	if targetAddr == nil {
		// This is site A (or any site not joining another)
		logger.Printf("[%s] STARTED: as primary site\n", *id)
		go sendPeriodic(*id, conn, nil)
		select {}
	}

	// This is site B: sends admission request to site A
	logger.Printf("[%s] REQUESTING: admission to network\n", *id)
	for {
		message := "admission request"
		conn.WriteToUDP([]byte(message), targetAddr)

		select {
		case <-admittedCh:
			// Once admitted, register site A in our connected sites
			siteKey := targetAddr.String()
			sitesMutex.Lock()
			connectedSites[siteKey] = targetAddr
			sitesMutex.Unlock()
			logger.Printf("[%s] REGISTERED: target site A at %s\n", *id, siteKey)

			go sendPeriodic(*id, conn, targetAddr) // Start sending
			select {}
		case <-time.After(5 * time.Second):
			// Retry
		}
	}
}
