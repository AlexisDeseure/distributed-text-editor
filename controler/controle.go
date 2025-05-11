package controler

import (
	"flag"
	"fmt"
	"log"
	"os"
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
	TypeField   string = "typ" // type of message
	UptField    string = "upt" // content of update for application
	HlgField    string = "hlg" // site clock value
	SiteIdField string = "sid" // site id
)


type TabElement struct {
	Type string
	Clock int
}

var fieldsep = "/"
var keyvalsep = "="

var cyan string = "\033[1;36m"
var raz string = "\033[0;00m"
var rouge string = "\033[1;31m"
var orange string = "\033[1;33m"

var pid = os.Getpid()
var stderr = log.New(os.Stderr, "", 0)

var id *int = flag.Int("id", 1, "id")
var N *int = flag.Int("N", 1, "number of sites")

func CreateDefaultTab(n int) []TabElement {
	arr := make([]TabElement, n)
	for i := range arr {
		arr[i] = TabElement{Type: MsgReleaseSc, Clock: 0}
	}
	return arr
}

func main() {

	flag.Parse()

	var sndmsg string
	var rcvtyp string
	var rcvmsg string
	var hrcv int
	var h int = 0
	
	tab := CreateDefaultTab(*N)

	for {
		fmt.Scanln(&rcvmsg)
		rcvtyp = findval(rcvmsg, TypeField)
		if rcvtyp == "" {
			continue
		}

		s_hrcv := findval(rcvmsg, HlgField)
		hrcv, _ = strconv.Atoi(s_hrcv)
		// if there is no "hlg" in the message, hrcv is 0 so new h will be h+1
		// if there is "hlg" in the message, h will be max(h, hrcv) + 1
		h = recaler(h, hrcv)
		sndmsg = ""
		switch rcvtyp {
		case MsgAppRequest:
			tab[*id].Type = MsgRequestSc
			tab[*id].Clock = h

			sndmsg = msg_format(TypeField, MsgRequestSc) +
				msg_format(HlgField, strconv.Itoa(h)) +
				msg_format(SiteIdField, strconv.Itoa(*id))
			display_d("Requesting critical section")
			
		case MsgAppRelease:
			tab[*id].Type = MsgReleaseSc
			tab[*id].Clock = h
			msg := findval(rcvmsg, UptField)

			sndmsg = msg_format(TypeField, MsgReleaseSc) +
				msg_format(HlgField, strconv.Itoa(h)) +
				msg_format(UptField, msg) +
				msg_format(SiteIdField, strconv.Itoa(*id))
			display_d("Releasing critical section")

		case MsgRequestSc:
			continue
		case MsgReleaseSc:
			continue
		case MsgReceiptSc:
			continue
		
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
