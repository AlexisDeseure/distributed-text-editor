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
	//message type to be received form network
	KnownSiteListMessage   string = "mks" // Messages list of sites to add to estampille tab
	InitializationMessage  string = "ini" // Initialization message to set the initial state
	GetSharedText          string = "gst" // Get the shared text from the controller
	AddSiteCriticalSection string = "asl" // add site to critical section

	// message type to be sent/received to/from other sites
	MsgRequestSc       string = "rqs" // request critical section
	MsgReleaseSc       string = "rls" // release critical section
	MsgReceiptSc       string = "rcs" // receipt of critical section
	MsgCut             string = "cut" // give the vectorial clock value
	MsgAppShallDie     string = "shd" // indicates that app shall die
	MsgAcknowledgement string = "ack" // tell the site's controler the number of lines of log file of sender id
	MsgPropagateText   string = "prp" // give the correct initial text to the next controller

	// message type to be receive from application
	MsgAppRequest string = "rqa" // request critical section
	MsgAppRelease string = "rla" // release critical section
	MsgAppDied    string = "apd" // indicates that app died
	// MsgInitialSize string = "siz" // give the number of lines in the log file
	// MsgInitialText string = "txt" // give the initial local text of app when the app begins

	// message type to be sent to application
	MsgAppStartSc        string = "ssa"  // start critical section
	MsgAppUpdate         string = "upa"  // update critical section
	MsgReturnInitialText string = "ret"  // give the initial common text content to the site
	MsgReturnText        string = "ret2" // give the current text content to the site
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
	KnownSiteList           string = "ksl" // list of sites to add to estampille tab
	SitesToAdd              string = "sta" // list of sites to add to the next release message
)

var (
	// id *int = flag.Int("id", 0, "id of site")
	id *string = flag.String("id", "0", "unique id of site (timestamp)") // get the timestamp id from site.sh
	s  int     = 0
)

// var text string = ""

var (
	outputDir        *string = flag.String("o", "./output", "output directory")
	localCutFilePath         = fmt.Sprintf("%s/cut.json", *outputDir)
)

func main() {
	flag.Parse()

	var sndmsg string // message to be sent
	var rcvtyp string // type of the received message
	var rcvmsg string // received message
	// var vcrcv []int = make([]int, *N)
	var vcrcv map[string]int = make(map[string]int)          // received vectorial clock
	var stamprcv int                                         // received stamp
	var idrcv string                                         // id of the controller who sent the received message
	var vectorialClock map[string]int = make(map[string]int) // vectorial clock initialized to 0
	vectorialClock[*id] = 0
	var currentAction int = 0       // action counter
	var idToAddNetworkNextRelease []string // id of the site to add to the next release message

	tab := CreateDefaultStateMap(*id) //not a table but a StateMap : make(map[string]*StateObject)
	// tabinit := CreateTabInit()
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
		stamprcv, err = strconv.Atoi(s_hrcv)
		if err != nil {
			stamprcv = 0
		}

		idrcv = findval(rcvmsg, SiteIdField, false)
		// idrcv, _ = strconv.Atoi(s_id)
		// if idrcv == *id {
		// 	display_e("Invalid site id received")
		// 	continue
		// }

		s_destid := findval(rcvmsg, SiteIdDestField, false)
		// destidrcv, _ = strconv.Atoi(s_destid)

		// if the message is a Receipt and is not for this site, ignore it
		if rcvtyp != MsgReceiptSc || s_destid == *id {

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
				vectorialClock = updateVectorialClock(vectorialClock, vcrcv, *id)
				jsonVc, err = json.Marshal(vectorialClock)
				if err != nil {
					display_e("JSON encoding error: " + err.Error())
				}
			}
		}

		sndmsg = ""

		// each message type will be processed differently
		switch rcvtyp {
		case KnownSiteListMessage:
			var knownSitesReceived []string
			knonwSite := findval(rcvmsg, KnownSiteList, false)

			err := json.Unmarshal([]byte(knonwSite), &knownSitesReceived)
			if err != nil {
				display_e("Erreur de décodage JSON :" + err.Error())
				return
			}

			for _, site := range knownSitesReceived {
				AddSiteToStateMap(&tab, site)
			}

		case GetSharedText:
			sndmsg = msg_format(TypeField, MsgReturnText) +
				msg_format(SiteIdField, idrcv)

			display_d("Getting text from application")

		case MsgReturnText:
			// This message is received from the application
			text := findval(rcvmsg, UptField, true)
			if idrcv == "-1" { // if idrcv is -1, it means that we need to share the return text to multiple sites : it is due to release of critical section
				if len(idToAddNetworkNextRelease) > 0 { // if there are sites to add to the next release message
					display_d("Returning text to network for one or more sites due to access to critical section")
					idToAddNetworkNextReleaseJson, err := json.Marshal(idToAddNetworkNextRelease)
					if err != nil {
						display_e("JSON encoding error for idToAddNetworkNextRelease: " + err.Error())
						continue
					}
					sndmsg = msg_format(TypeField, GetSharedText) +
						msg_format(SitesToAdd, string(idToAddNetworkNextReleaseJson)) +
						msg_format(UptField, text)
				}
			} else { // if idrcv is not -1, it means that the site wanting to join network is already known
				display_d("Returning text to network for a single site")
				singleSiteTab := []string{idrcv}
				singleSiteTabJson, err := json.Marshal(singleSiteTab)
				if err != nil {
					display_e("JSON encoding error for singleSiteTab: " + err.Error())
					continue
				}

				sndmsg = msg_format(TypeField, GetSharedText) +
					msg_format(SitesToAdd, string(singleSiteTabJson)) +
					msg_format(UptField, text)
			}
			
		
		case AddSiteCriticalSection:
			display_d("Add site to critical section message received : site will be added to the next release message")
			idToAddNetworkNextRelease = append(idToAddNetworkNextRelease, idrcv)
			if tab[*id].Type != MsgRequestSc {
				tab[*id].Type = MsgRequestSc
				tab[*id].Clock = s
				
				sndmsg = msg_format(TypeField, MsgRequestSc) +
					msg_format(StampField, strconv.Itoa(s)) +
					msg_format(SiteIdField, *id) +
					msg_format(VectorialClockField, string(jsonVc))
				display_d("Requesting critical section (to at least add site to network)")
			}

		// This message is sent by the site to request access to the critical section
		// so that other sites cannot access it
		case MsgAppRequest:
			display_d("Request message received from application")
			if tab[*id].Type != MsgRequestSc {
				tab[*id].Type = MsgRequestSc
				tab[*id].Clock = s

				sndmsg = msg_format(TypeField, MsgRequestSc) +
					msg_format(StampField, strconv.Itoa(s)) +
					msg_format(SiteIdField, *id) +
					msg_format(VectorialClockField, string(jsonVc))
				display_d("Requesting critical section (to at least send modification in shared text)")
			}

		// This message is sent by the site to ask the release of the critical section
		// so that other sites can access it again
		case MsgAppRelease:

			tab[*id].Type = MsgReleaseSc
			tab[*id].Clock = s
			msg := findval(rcvmsg, UptField, true)
			display_d("Release message received from application")

			jsonIdToAdd, err := json.Marshal(idToAddNetworkNextRelease)
			if err != nil {
				display_e("JSON encoding error for idToAddNetworkNextRelease: " + err.Error())
			}

			sndmsg = msg_format(TypeField, MsgReleaseSc) +
				msg_format(StampField, strconv.Itoa(s)) +
				msg_format(UptField, msg) +
				msg_format(SiteIdField, *id) +
				msg_format(VectorialClockField, string(jsonVc)) +
				msg_format(SitesToAdd, string(jsonIdToAdd))
			display_d("Releasing critical section")
			idToAddNetworkNextRelease = idToAddNetworkNextRelease[:0] // reset the list after use

		// This message is sent by another controller to announce that the critical section is temporarily locked
		case MsgRequestSc:

			if idrcv != *id {
				tab[idrcv].Type = MsgRequestSc
				tab[idrcv].Clock = stamprcv
				display_d("Request message received")

				// send receipt to the sender by the successor (ring topology)
				sndmsg = msg_format(TypeField, MsgReceiptSc) +
					msg_format(StampField, strconv.Itoa(s)) +
					msg_format(SiteIdField, *id) +
					msg_format(SiteIdDestField, idrcv) +
					msg_format(VectorialClockField, string(jsonVc))
				display_d("Sending receipt")

			}
			verifyScApproval(tab, *id) // outside the if to work when the site is alone in the network

		// This message is sent by another controller to announce that the critical section has been released
		case MsgReleaseSc:

			if idrcv != *id {
				tab[idrcv].Type = MsgReleaseSc
				tab[idrcv].Clock = stamprcv
				display_d("Release message received")

				// send the updated message to the application
				sndmsg = msg_format(TypeField, MsgAppUpdate) +
					msg_format(UptField, findval(rcvmsg, UptField, true))
				display_d("Sending update message to application")

				verifyScApproval(tab, *id)
			}

		// This message is sent by another controller to give a receipt after receiving a previous message
		case MsgReceiptSc:
			if idrcv != *id {
				if s_destid == *id {
					if tab[idrcv].Type != MsgRequestSc {
						tab[idrcv].Type = MsgReceiptSc
						tab[idrcv].Clock = stamprcv
					}
					display_d("Receipt received")

					verifyScApproval(tab, *id)
				}
			}

		case InitializationMessage:
			// 		msg_format(UptField, originalText)
			var knownSitesReceived []string
			knownSite := findval(rcvmsg, KnownSiteList, false)
			if knownSite != "" { // if the site enter in a network
				display_d("Controller initialization message received as a secondary site")
				err := json.Unmarshal([]byte(knownSite), &knownSitesReceived)
				if err != nil {
					display_e("Erreur de décodage JSON :" + err.Error())
					return
				}

				for _, site := range knownSitesReceived {
					AddSiteToStateMap(&tab, site)
				}

				text := findval(rcvmsg, UptField, true)
				sndmsg = msg_format(TypeField, MsgReturnInitialText) +
					msg_format(SiteIdField, idrcv) +
					msg_format(UptField, text)
			} else { // if the site is the first one to enter in the network : primary site
				display_d("Controller initialization message received as a primary site")
				sndmsg = msg_format(TypeField, MsgReturnInitialText) +
					msg_format(SiteIdField, idrcv)
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
			nbcut := findval(rcvmsg, cutNumber, false)
			nbvls, err := strconv.Atoi(findval(rcvmsg, NumberVirtualClockSaved, false))
			if err != nil {
				display_e("Error : " + err.Error())
			}
			if nbvls < len(tab) {
				if nbvls == 0 {
					display_d("Cut message received from application")
				} else {
					display_d("Cut message received from a controler")
				}

				siteActionNumber := fmt.Sprintf("site_%s_action_%d", *id, currentAction+1)

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
