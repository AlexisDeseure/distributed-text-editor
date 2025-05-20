package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// message types
const (

	// message type to be sent/received to/from other sites
	MsgRequestSc       string = "rqs" // request critical section
	MsgReleaseSc       string = "rls" // release critical section
	MsgReceiptSc       string = "rcs" // receipt of critical section
	MsgCut             string = "cut" // give the vectorial clock value
	MsgAppShallDie     string = "shd" // indicates that app shall die
	MsgAcknowledgement string = "ack" // tell the site's controler the number of lines of log file of sender id
	MsgPropagateText   string = "prp" // give the correct initial text to the next controller

	// message type to be receive from application
	MsgAppRequest  string = "rqa" // request critical section
	MsgAppRelease  string = "rla" // release critical section
	MsgAppDied     string = "apd" // indicates that app died
	MsgInitialSize string = "siz" // give the number of lines in the log file
	MsgInitialText string = "txt" // give the initial local text of app when the app begins

	// message type to be sent to application
	MsgAppStartSc    string = "ssa" // start critical section
	MsgAppUpdate     string = "upa" // update critical section
	MsgReturnNewText string = "ret" // give the new initial common text content to the site
)

// message fields
const (
	TypeField               string = "typ" // type of message
	UptField                string = "upt" // content of update for application
	StampField              string = "stp" // site stamp value
	SiteIdField             string = "sid" // site id of sender
	SiteIdDestField         string = "did" // site id of destination
	VectorialClockField     string = "vcl" // vectorial clock value
	cutNumber               string = "cnb" // number of next cut
	NumberVirtualClockSaved string = "nbv" // number of virtual clock saved
)

var (
	id *int = flag.Int("id", 0, "id of site")
	N  *int = flag.Int("N", 1, "number of sites")
	s  int  = 0
)

var text string = ""

var (
	outputDir        *string = flag.String("o", "./output", "output directory")
	localCutFilePath         = fmt.Sprintf("%s/cut.json", *outputDir)
)

func main() {
	flag.Parse()

	if *id < 0 || *id >= *N {
		display_e("Invalid site id")
		return
	}

	if *N < 1 {
		display_e("Invalid number of sites")
		return
	}

	var sndmsg string                          // message to be sent
	var rcvtyp string                          // type of the received message
	var rcvmsg string                          // received message
	var vcrcv []int = make([]int, *N)          // received vectorial clock
	var stamprcv int                           // received stamp
	var idrcv int                              // id of the controller who sent the received message
	var destidrcv int                          // id of the controller which the received message is destined to
	var vectorialClock []int = make([]int, *N) // vectorial clock initialized to 0
	var currentAction int = 0                  // action counter

	tab := CreateDefaultTab(*N)
	tabinit := CreateTabInit(*N)
	reader := bufio.NewReader(os.Stdin)

	for {

		// transform the vectorial clock into a json at the beginning of the loop
		// to avoid nill/undefined jsonVc value
		jsonVc, err := json.Marshal(vectorialClock)

		if err != nil {
			display_e("JSON encoding error: " + err.Error())
		}

		rcvmsgRaw, err := reader.ReadString('\n')
		if err != nil {
			display_e("Error reading message : " + err.Error())
			continue
		}

		rcvmsg = strings.TrimSuffix(rcvmsgRaw, "\n")

		rcvtyp = findval(rcvmsg, TypeField, true)
		if rcvtyp == "" {
			continue
		}

		// if there is no "stp" in the message, stamprcv is 0 so new s will be stamp+1
		// if there is "stp" in the message, s will be max(s, stamprcv) + 1
		s_hrcv := findval(rcvmsg, StampField, false)
		stamprcv, _ = strconv.Atoi(s_hrcv)

		s_id := findval(rcvmsg, SiteIdField, false)
		idrcv, _ = strconv.Atoi(s_id)
		if idrcv < 0 || idrcv >= *N {
			display_e("Invalid site id received")
			continue
		}

		s_destid := findval(rcvmsg, SiteIdDestField, false)
		destidrcv, _ = strconv.Atoi(s_destid)

		nbcut := findval(rcvmsg, cutNumber, false)

		// if the message is a Receipt and is not for this site, ignore it
		if rcvtyp != MsgReceiptSc || destidrcv == *id {

			// update the stamp of the site
			s = resetStamp(s, stamprcv)

			// get a possible vectorial clock from the message
			tmp_vcrc := findval(rcvmsg, VectorialClockField, false)

			if tmp_vcrc != "" {

				// update the vectorial clock if the message is not from the application
				var err error
				err = json.Unmarshal([]byte(tmp_vcrc), &vcrcv)
				if err != nil {
					display_e(rcvmsg + " : Error unmarshalling vectorial clock: " + err.Error())
				}

				// update the vectorial clock
				vectorialClock = updateVectorialClock(vectorialClock, vcrcv)
				jsonVc, err = json.Marshal(vectorialClock)
				if err != nil {
					display_e("JSON encoding error: " + err.Error())
				}
			}
		}

		sndmsg = ""

		// each message type will be processed differently
		switch rcvtyp {

		// This message is sent by the site to request access to the critical section
		// so that other sites cannot access it
		case MsgAppRequest:

			tab[*id].Type = MsgRequestSc
			tab[*id].Clock = s
			display_d("Request message received from application")

			sndmsg = msg_format(TypeField, MsgRequestSc) +
				msg_format(StampField, strconv.Itoa(s)) +
				msg_format(SiteIdField, strconv.Itoa(*id)) +
				msg_format(VectorialClockField, string(jsonVc))
			display_d("Requesting critical section")

		// This message is sent by the site to ask the release of the critical section
		// so that other sites can access it again
		case MsgAppRelease:

			tab[*id].Type = MsgReleaseSc
			tab[*id].Clock = s
			msg := findval(rcvmsg, UptField, true)
			display_d("Release message received from application")

			sndmsg = msg_format(TypeField, MsgReleaseSc) +
				msg_format(StampField, strconv.Itoa(s)) +
				msg_format(UptField, msg) +
				msg_format(SiteIdField, strconv.Itoa(*id)) +
				msg_format(VectorialClockField, string(jsonVc))
			display_d("Releasing critical section")

		// This message is sent by another controller to announce that the critical section is temporarily locked
		case MsgRequestSc:

			if idrcv != *id {
				tab[idrcv].Type = MsgRequestSc
				tab[idrcv].Clock = stamprcv
				display_d("Request message received")

				// forward the message to the next site as id != idrcv
				fmt.Println(rcvmsg)
				display_d("Forwarding request message")

				// send receipt to the sender by the successor (ring topology)
				sndmsg = msg_format(TypeField, MsgReceiptSc) +
					msg_format(StampField, strconv.Itoa(s)) +
					msg_format(SiteIdField, strconv.Itoa(*id)) +
					msg_format(SiteIdDestField, strconv.Itoa(idrcv)) +
					msg_format(VectorialClockField, string(jsonVc))
				display_d("Sending receipt")

				verifyScApproval(tab)
			}

		// This message is sent by another controller to announce that the critical section has been released
		case MsgReleaseSc:

			if idrcv != *id {
				tab[idrcv].Type = MsgReleaseSc
				tab[idrcv].Clock = stamprcv
				display_d("Release message received")

				// forward the message to the next site as id != idrcv
				fmt.Println(rcvmsg)
				display_d("Forwarding release message")

				// send the updated message to the application
				sndmsg = msg_format(TypeField, MsgAppUpdate) +
					msg_format(UptField, findval(rcvmsg, UptField, true))
				display_d("Sending update message to application")

				verifyScApproval(tab)
			}

		// This message is sent by another controller to give a receipt after receiving a previous message
		case MsgReceiptSc:

			if idrcv != *id {
				if destidrcv == *id {
					if tab[idrcv].Type != MsgRequestSc {
						tab[idrcv].Type = MsgReceiptSc
						tab[idrcv].Clock = stamprcv
					}
					display_d("Receipt received")

					verifyScApproval(tab)
				} else {
					// forward the message to the next site as id != destidrcv and id != idrcv
					sndmsg = rcvmsg
					display_d("Forwarding receipt message")
				}
			}

		// This message is sent by the site and contains the number of lines in the local log file (save file)
		case MsgInitialSize:

			display_d("Initial size received")
			msg := findval(rcvmsg, UptField, true)
			size, err := strconv.Atoi(msg)
			if err != nil {
				display_e("Error while converting string to int: " + err.Error())
			}
			tabinit[*id] = size

			// An acknowledgment message is sent to every other controller
			// it contains the number of lines in the local save file of this controller
			sndmsg = msg_format(TypeField, MsgAcknowledgement) +
				msg_format(SiteIdField, strconv.Itoa(*id)) +
				msg_format(UptField, strconv.Itoa(size))
			display_d("Acknowledgement message sent")

		// This message is sent by the site and contains each line stored in the local log file (save file)
		case MsgInitialText:

			// This message is always received after MsgInitialSize due to FIFO channels.
			// Thus, verification is only performed here in cases where all sites have acknowledged
			// their number of lines and this site has the maximum. If all the MsgAcknowledgement
			// messages were received before the MsgInitialText message, the verification process
			// in the "case" would have always return an empty string, so verifying here is very important.
			display_d("Initial text receive")
			text = findval(rcvmsg, UptField, true)
			sndmsg = verifyIfMaxNbLinesSite(tabinit) // verify if the site has the max and return the message to send
			// if it is correct else return an empty string

		// This message is received from every other controller and contains the number of lines in their own local save file
		case MsgAcknowledgement:

			if idrcv != *id {

				msg := findval(rcvmsg, UptField, true)
				size, err := strconv.Atoi(msg)
				if err != nil {
					display_e("Error while converting string to int: " + err.Error())
				}

				display_d("Message acknowledgement received from site " +
					strconv.Itoa(idrcv) +
					" with " +
					strconv.Itoa(size) +
					" lines")
				// forward the message to the next site as id != idrcv
				fmt.Println(rcvmsg)
				display_d("Forwarding acknowledgement message")

				tabinit[idrcv] = size
				sndmsg = verifyIfMaxNbLinesSite(tabinit) // verify if the site has the max and return the message to send
				// if it is correct else return an empty string

			}

		// This message is sent by the controller with the longest local save file
		// it contains the lines of its save file, to replace the save file of this one
		case MsgPropagateText:

			if idrcv != *id {
				display_d("Initialization text received from site " + strconv.Itoa(idrcv))
				// forward the message to the next site as id != idrcv
				fmt.Println(rcvmsg)
				display_d("Forwarding initial text message")

				text = findval(rcvmsg, UptField, false)

				sndmsg = msg_format(TypeField, MsgReturnNewText) +
					msg_format(UptField, text)
				display_d("Sending initial text to app")
			}

		// This message is sent by the site when the user kills the window
		case MsgAppDied:

			// Other controllers will receive a message so they can stop cleanly
			sndmsg = msg_format(TypeField, MsgAppShallDie)
			display_d("App died : sending message to other sites")

		// This message is sent by another controller and commands this one to stop
		case MsgAppShallDie:
			display_d("App died message received : forwarding it and closing controler")
			// The message is forwarded to other controllers and to the site
			fmt.Println(rcvmsg)
			os.Stdout.Sync()
			return

		// This message is sent by the site to request a cut
		// It is then propagated to other controllers
		case MsgCut:

			nbvls, err := strconv.Atoi(findval(rcvmsg, NumberVirtualClockSaved, false))
			if err != nil {
				display_e("Error : " + err.Error())
			}
			if nbvls < *N {
				if nbvls == 0 {
					display_d("Cut message received from application")
				} else {
					display_d("Cut message received from a controler")
				}

				siteActionNumber := fmt.Sprintf("site_%d_action_%d", *id, currentAction+1)

				saveCutJson(nbcut, vectorialClock, siteActionNumber, localCutFilePath)
				nbvls++

				sndmsg = msg_format(TypeField, MsgCut) +
					msg_format(cutNumber, nbcut) +
					msg_format(NumberVirtualClockSaved, strconv.Itoa(nbvls))
			} else {
				sndmsg = ""
				display_d("Cut saved to file")
			}

		}

		// send message to successor
		if sndmsg != "" {
			currentAction++
			fmt.Println(sndmsg)
		}

	}
}
