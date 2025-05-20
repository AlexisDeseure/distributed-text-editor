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
	MsgRequestSc          string = "rqs" // request critical section
	MsgReleaseSc          string = "rls" // release critical section
	MsgReceiptSc          string = "rcs" // receipt of critical section
	MsgCut                string = "cut" // save the vectorial clock value
	MsgAppShallDie        string = "shd" // app shall die
	MsgAcknowledgement    string = "ack" // tell controller n°0 that one controller is ready to compare its log size
	MsgCompareSize        string = "cmp" // number of lines and id so that it can be compared with other sizes
	MsgRequestPropagation string = "rqp" // request controller with the largest log file to send it to the others
	MsgPropagateText      string = "prp" // send the associated text to the next controller

	// message type to be receive from application
	MsgAppRequest  string = "rqa" // request critical section
	MsgAppRelease  string = "rla" // release critical section
	MsgAppDied     string = "apd" // app died
	MsgInitialSize string = "siz" // number of lines in the log file
	MsgInitialText string = "txt" // Initial text when the app begins

	// message type to be sent to application
	MsgAppStartSc    string = "ssa" // start critical section
	MsgAppUpdate     string = "upa" // update critical section
	MsgReturnNewText string = "ret" // return the new common text content to the site
)

// message fields
const (
	TypeField               string = "typ" // type of message
	UptField                string = "upt" // content of update for application
	HlgField                string = "hlg" // site clock value
	SiteIdField             string = "sid" // site id of sender
	SiteIdDestField         string = "did" // site id of destination
	VectorialClockField     string = "vcl" // vectorial clock value
	cutNumber               string = "cnb" // number of next cut
	NumberVirtualClockSaved string = "nbv" // number of virtual clock saved
)

type CompareElement struct {
	Clock int
	Id    int
}

type TabElement struct {
	Type  string
	Clock int
}

type Tab []TabElement

var (
	id *int = flag.Int("id", 0, "id of site")
	N  *int = flag.Int("N", 1, "number of sites")
	h  int  = 0
)

var (
	size int    = 0
	text string = ""
	best bool   = false
)

var initializedSites int = 0

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

	var sndmsg string
	var rcvtyp string
	var rcvmsg string
	var vcrcv []int = make([]int, *N) // vectorial clock received
	var hrcv int
	var idrcv int
	var destidrcv int
	var vectorialClock []int = make([]int, *N) // vectorial clock initialized to 0
	var currentAction int = 0

	tab := CreateDefaultTab(*N)
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

		// if there is no "hlg" in the message, hrcv is 0 so new h will be h+1
		// if there is "hlg" in the message, h will be max(h, hrcv) + 1
		s_hrcv := findval(rcvmsg, HlgField, false)
		hrcv, _ = strconv.Atoi(s_hrcv)

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

			// update the clock of the site
			h = resetClock(h, hrcv)

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

		switch rcvtyp {

		case MsgAppRequest:

			tab[*id].Type = MsgRequestSc
			tab[*id].Clock = h
			display_d("Request message received from application")

			sndmsg = msg_format(TypeField, MsgRequestSc) +
				msg_format(HlgField, strconv.Itoa(h)) +
				msg_format(SiteIdField, strconv.Itoa(*id)) +
				msg_format(VectorialClockField, string(jsonVc))
			display_d("Requesting critical section")

		case MsgAppRelease:

			tab[*id].Type = MsgReleaseSc
			tab[*id].Clock = h
			msg := findval(rcvmsg, UptField, true)
			display_d("Release message received from application")

			sndmsg = msg_format(TypeField, MsgReleaseSc) +
				msg_format(HlgField, strconv.Itoa(h)) +
				msg_format(UptField, msg) +
				msg_format(SiteIdField, strconv.Itoa(*id)) +
				msg_format(VectorialClockField, string(jsonVc))
			display_d("Releasing critical section")

		case MsgRequestSc:

			if idrcv != *id {
				tab[idrcv].Type = MsgRequestSc
				tab[idrcv].Clock = hrcv
				display_d("Request message received")

				// forward the message to the next site as id != idrcv
				fmt.Println(rcvmsg)
				display_d("Forwarding request message")

				// send receipt to the sender by the successor (ring topology)
				sndmsg = msg_format(TypeField, MsgReceiptSc) +
					msg_format(HlgField, strconv.Itoa(h)) +
					msg_format(SiteIdField, strconv.Itoa(*id)) +
					msg_format(SiteIdDestField, strconv.Itoa(idrcv)) +
					msg_format(VectorialClockField, string(jsonVc))
				display_d("Sending receipt")

				verifyScApproval(tab)
			}

		case MsgReleaseSc:

			if idrcv != *id {
				tab[idrcv].Type = MsgReleaseSc
				tab[idrcv].Clock = hrcv
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

		case MsgReceiptSc:

			if idrcv != *id {
				if destidrcv == *id {
					if tab[idrcv].Type != MsgRequestSc {
						tab[idrcv].Type = MsgReceiptSc
						tab[idrcv].Clock = hrcv
					}
					display_d("Receipt received")

					verifyScApproval(tab)
				} else {
					// forward the message to the next site as id != destidrcv and id != idrcv
					sndmsg = rcvmsg
					display_d("Forwarding receipt message")
				}
			}

		case MsgInitialSize:

			msg := findval(rcvmsg, UptField, true)
			size, err = strconv.Atoi(msg)
			if err != nil {
				display_e("Error while converting string to int: " + err.Error())
			}

		case MsgInitialText:

			text = findval(rcvmsg, UptField, true)

			// All controllers that are not controller n°0 should inform controller n°0 that they are ready
			if *id != 0 {
				sndmsg = msg_format(TypeField, MsgAcknowledgement) + msg_format(SiteIdDestField, strconv.Itoa(0))
			}

		case MsgAcknowledgement:

			// If the message is not for this site, forward it to the next one
			if destidrcv != *id {
				display_d("Forwarding acknowledgement message")
				fmt.Println(rcvmsg)
				continue
			}

			// This message can only be sent to controler n°0.
			// This controler will then know when every other controler is ready to communicate
			initializedSites++

			// If every other site is initialized we can start comparing sizes
			if initializedSites >= *N - 1 {
				sndmsg = msg_format(TypeField, MsgCompareSize) + msg_format(UptField, strconv.Itoa(size)+"|"+strconv.Itoa(*id))
			}

		case MsgCompareSize:

			msg := findval(rcvmsg, UptField, true)
			parts := strings.SplitN(msg, "|", 2)

			rcvsize, err := strconv.Atoi(parts[0])
			if err != nil {
				display_e("Error while converting string to int")
			}

			if *id != 0 {
				if rcvsize > size {
					sndmsg = msg_format(TypeField, MsgCompareSize) + msg_format(UptField, msg)
				} else {
					sndmsg = msg_format(TypeField, MsgCompareSize) + msg_format(UptField, strconv.Itoa(size)+"|"+strconv.Itoa(*id))
				}
			} else {
				bestid, err := strconv.Atoi(parts[1])
				if err != nil {
					display_e("Error while converting string to int")
				}
				sndmsg = msg_format(TypeField, MsgRequestPropagation) + msg_format(SiteIdDestField, strconv.Itoa(bestid))
			}

		case MsgRequestPropagation:

			// If the message is not for this site, forward it to the next one
			if destidrcv != *id {
				display_d("Forwarding propagation request")
				fmt.Println(rcvmsg)
				continue
			}

			best = true
			sndmsg = msg_format(TypeField, MsgPropagateText) + msg_format(UptField, text)

		case MsgPropagateText:

			if !best {
				text = findval(rcvmsg, UptField, false)
				sndmsg = msg_format(TypeField, MsgReturnNewText) + msg_format(UptField, text)
				fmt.Println(sndmsg)
				sndmsg = msg_format(TypeField, MsgPropagateText) + msg_format(UptField, text)
			}

		case MsgAppDied:

			sndmsg = msg_format(TypeField, MsgAppShallDie)
			fmt.Println(sndmsg)

		case MsgAppShallDie:

			sndmsg = msg_format(TypeField, MsgAppShallDie)
			fmt.Println(sndmsg)
			os.Stdout.Sync()
			return

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
