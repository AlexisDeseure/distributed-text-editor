package main

import (
	"fmt"
	"strings"
)

var (
	fieldsep  = "~"
	keyvalsep = "`"
)

type CompareElement struct {
	Clock int
	Id    string
}

type StateObject struct {
	Type  string
	Clock int
}

type StateMap map[string]*StateObject

// msg_format constructs a key-value string using predefined separators
func msg_format(key string, val string) string {
	return fieldsep + keyvalsep + key + keyvalsep + val
}

// resetStamp returns the next logical timestamp, ensuring monotonicity
func resetStamp(stamp, stamprcv int) int {
	if stamp < stamprcv {
		return stamprcv + 1
	}
	return stamp + 1
}

// findval searches a formatted message for a given key and returns its value
func findval(msg string, key string, verbose bool) string {
	if len(msg) < 4 {
		if verbose {
			display_e("Message length too short: " + msg)
		}
		return ""
	}

	sep := msg[0:1]
	tab_allkeyvals := strings.Split(msg[1:], sep)

	for _, keyval := range tab_allkeyvals {

		if len(keyval) < 4 {
			continue
		}

		equ := keyval[0:1]
		tabkeyval := strings.Split(keyval[1:], equ)
		if tabkeyval[0] == key {
			return tabkeyval[1]
		}
	}
	if verbose {
		err_msg := fmt.Sprintf("Key %s not found in message", key)
		display_w(err_msg)
	}
	return ""
}

// CreateDefaultTab initializes a Map with the current site ID and a default state object
func CreateDefaultStateMap(siteID string) StateMap {

	stateMap := make(map[string]*StateObject)
	stateMap[siteID] = &StateObject{Type: MsgReleaseSc, Clock: 0}
	return stateMap
}

func AddSiteToStateMap(stateMap *StateMap, siteID string) {
	if _, exists := (*stateMap)[siteID]; !exists {
		(*stateMap)[siteID] = &StateObject{
			Type:  MsgReleaseSc,
			Clock: -1, // Initialize with -1 to indicate not set : to be sure we won't get the priority
		}
	}
}

// CreateTabInit returns an integer slice of length n filled with -1
func CreateTabInit() map[string]int {
	return make(map[string]int)
}

// timestampComparison returns true if element a precedes b by clock, then id
func timestampComparison(a, b CompareElement) bool {
	if a.Clock < b.Clock {
		return true
	} else if a.Clock == b.Clock && a.Id < b.Id {
		return true
	}
	return false
}

// verifyScApproval checks if the local site can enter the critical section and signals approval
func verifyScApproval(tab StateMap, myID string) {
	var sndmsg string
	if tab[myID].Type == MsgRequestSc {

		site_elem := CompareElement{Clock: tab[myID].Clock, Id: myID}

		for i, el := range tab {
			inter_elem := CompareElement{Clock: el.Clock, Id: i}
			if i != myID && !timestampComparison(site_elem, inter_elem) {
				return
			}
		}

		sndmsg = msg_format(TypeField, MsgAppStartSc)
		fmt.Println(sndmsg)
		display_d("Entering critical section")
	}
}
