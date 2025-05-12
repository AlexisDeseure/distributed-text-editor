package main

import (
	"flag"
	"fmt"
	"strconv"
)

const (
	// message type to be sent/received to/from other sites
	MsgRequestSc   string = "rqs" // request critical section
	MsgReleaseSc   string = "rls" // release critical section
	MsgReceiptSc   string = "rcs" // receipt of critical section
	// message type to be receive from application
	MsgAppRequest  string = "rqa" // request critical section
	MsgAppRelease  string = "rla" // release critical section
	// message type to be sent to application
	MsgAppStartSc  string = "ssa" // start critical section
	MsgAppUpdate   string = "upa" // update critical section
)

const (
	TypeField       string = "typ" // type of message
	UptField        string = "upt" // content of update for application
	HlgField        string = "hlg" // site clock value
	SiteIdField     string = "sid" // site id of sender
	SiteIdDestField string = "did" // site id of destination
)

type CompareElement struct {
	Clock int
	Id    int
}

type TabElement struct {
	Type string
	Clock int
}

type Tab []TabElement

var id *int = flag.Int("id", 0, "id")
var N *int = flag.Int("N", 1, "number of sites")

func main() {

	flag.Parse()

	var sndmsg string
	var rcvtyp string
	var rcvmsg string
	var hrcv int
	var idrcv int
	var destidrcv int
	var h int = 0
	
	tab := CreateDefaultTab(*N)

	for {
		fmt.Scanln(&rcvmsg)
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
		}

		sndmsg = ""

		switch rcvtyp {
		case MsgAppRequest:
			tab[*id].Type = MsgRequestSc
			tab[*id].Clock = h
			display_d("Request message received from application")

			sndmsg = msg_format(TypeField, MsgRequestSc) +
				msg_format(HlgField, strconv.Itoa(h)) +
				msg_format(SiteIdField, strconv.Itoa(*id))
			display_d("Requesting critical section")
			
		case MsgAppRelease:
			tab[*id].Type = MsgReleaseSc
			tab[*id].Clock = h
			msg := findval(rcvmsg, UptField, true)
			display_d("Release message received from application")

			sndmsg = msg_format(TypeField, MsgReleaseSc) +
				msg_format(HlgField, strconv.Itoa(h)) +
				msg_format(UptField, msg) +
				msg_format(SiteIdField, strconv.Itoa(*id))
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
					msg_format(SiteIdDestField, strconv.Itoa(idrcv))
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
		
		// unknown or not handled message type
		// default:
		// 	display_e("Unknown or not handled message type")
		// 	continue		
		}

		// send message to successor
		if sndmsg != "" {
			fmt.Println(sndmsg)
		}
		
	}
}
