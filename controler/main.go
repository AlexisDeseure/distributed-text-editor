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

const (
	// message type to be sent/received to/from other sites
	MsgRequestSc   string = "rqs" // request critical section
	MsgReleaseSc   string = "rls" // release critical section
	MsgReceiptSc   string = "rcs" // receipt of critical section
	MsgCut         string = "cut" // save the vectorial clock value
	MsgAppShallDie string = "shd" // app shall die
	// message type to be receive from application
	MsgAppRequest string = "rqa" // request critical section
	MsgAppRelease string = "rla" // release critical section
	MsgAppDied    string = "apd" // app died
	// message type to be sent to application
	MsgAppStartSc string = "ssa" // start critical section
	MsgAppUpdate  string = "upa" // update critical section
)

const (
	TypeField           string = "typ" // type of message
	UptField            string = "upt" // content of update for application
	HlgField            string = "hlg" // site clock value
	SiteIdField         string = "sid" // site id of sender
	SiteIdDestField     string = "did" // site id of destination
	VectorialClockField string = "vcl" // vectorial clock value
	ActionNumberField   string = "act" // number of the site's current action
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

var id *int = flag.Int("id", 0, "id of site")
var N *int = flag.Int("N", 1, "number of sites")
var h int = 0

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
		if idrcv > *N {
			display_e("Invalid site id received")
			continue
		}

		s_destid := findval(rcvmsg, SiteIdDestField, false)
		destidrcv, _ = strconv.Atoi(s_destid)

		// if the message is not for this site, ignore it
		if s_destid == "" || destidrcv == *id {
			// update the clock of the site
			h = resetClock(h, hrcv)
			currentAction++

			if rcvtyp != MsgAppRequest && rcvtyp != MsgAppRelease && rcvtyp != MsgAppStartSc && rcvtyp != MsgAppUpdate {
				// update the vectorial clock if the message is not from the application
				var err error
				// get the vectorial clock from the message
				tmp_vcrc := findval(rcvmsg, VectorialClockField, false)
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

		case MsgAppDied:
			sndmsg = msg_format(TypeField, MsgAppShallDie)
			fmt.Println(sndmsg)

		case MsgAppShallDie:
			sndmsg = msg_format(TypeField, MsgAppShallDie)
			fmt.Println(sndmsg)
			display_w("Controller died")
			os.Stdout.Sync()
			return

			// unknown or not handled message type
			// default:
			// 	display_e("Unknown or not handled message type")
			// 	continue
		}

		// send message to successor
		if sndmsg != "" {
			currentAction++
			fmt.Println(sndmsg)
		}

	}
}
