package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"time"
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
	go startServer(listenPort)

	// TODO: connect random to create random network
	rand.Seed(time.Now().UnixNano())
	next := rand.Intn(*N - 1)
	if next >= *id {
		next++
	}
	nextPort := portBase + next

	go connectToPeer(nextPort)

	// TODO: tmp wait forever
	select {}
}

func startServer(port int) {
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
		go handleConnection(conn)
	}
}

func connectToPeer(port int) {
	for {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
		if err != nil {
			//display_w("Retrying connection to port " + strconv.Itoa(port) + "...")
			continue
		}
		display_w("Connected to peer on port " + strconv.Itoa(port))
		handleConnection(conn)
		return
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		msg := scanner.Text()
		display_w("Received: " + msg)
		// Here you could forward to controller through FIFO or channel
	}
}
