package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

var mutex = &sync.Mutex{}

// Once admitted, send messages periodically
func sendPeriodic(id string) {
	i := 0
	for {
		i++
		mutex.Lock()
		fmt.Printf("%s: message_%d\n", id, i)
		mutex.Unlock()
		time.Sleep(2 * time.Second)
	}
}

// Handle a single incoming message
func processMessage(id, target, msg string, admittedCh chan<- bool, logger *log.Logger) {
	switch msg {
	case "admission request":
		logger.Printf("%s: received <admission request>\n", id)
		mutex.Lock()
		fmt.Println("admission permitted")
		mutex.Unlock()
	case "admission permitted":
		logger.Printf("%s: received <admission permitted>\n", id)
		if target != "" {
			logger.Printf("%s: Joined the site network\n", id)
			// Signal admission, but do not stop receiving
			admittedCh <- true
		}
	default:
		logger.Printf("%s: received <%s>\n", id, msg)
	}
}

// Site listens for messages from stdin
func handleReceive(id string, target string, admittedCh chan<- bool) {
	scanner := bufio.NewScanner(os.Stdin)
	logger := log.New(os.Stderr, "", 0)

	for scanner.Scan() {
		msg := scanner.Text()
		processMessage(id, target, msg, admittedCh, logger)
	}
}

func main() {
	id := flag.String("id", "", "site ID (ex: A or B)")
	target := flag.String("target", "", "target ID (optional, used by B)")
	flag.Parse()

	if *id == "" {
		fmt.Fprintln(os.Stderr, "Error: -id parameter is required")
		os.Exit(1)
	}

	admittedCh := make(chan bool)

	go handleReceive(*id, *target, admittedCh)

	if *target == "" {
		// This is site A (or any site not joining another)
		go sendPeriodic(*id)
		select {}
	}

	// This is site B: sends admission request to site A
	for {
		mutex.Lock()
		fmt.Println("admission request")
		mutex.Unlock()

		select {
		case <-admittedCh:
			go sendPeriodic(*id) // Start sending, but keep receiving
			select {}
		case <-time.After(5 * time.Second):
			// retry
		}
	}
}
