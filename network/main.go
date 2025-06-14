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
