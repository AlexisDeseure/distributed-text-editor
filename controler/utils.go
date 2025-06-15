package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
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

// updateVectorialClock merges two vector clocks and increments the local entry
//
//	func updateVectorialClock(oldVectorialClock []int, newVectorialClock []int) []int {
//		for i := range oldVectorialClock {
//			oldVectorialClock[i] = int(math.Max(float64(oldVectorialClock[i]), float64(newVectorialClock[i])))
//		}
//		oldVectorialClock[*id]++
//		return oldVectorialClock
//	}
func updateVectorialClock(localClock map[string]int, receivedClock map[string]int, mySiteID string) map[string]int {
	for siteID, receivedValue := range receivedClock {
		if localValue, exists := localClock[siteID]; !exists || receivedValue > localValue {
			localClock[siteID] = receivedValue
		}
	}
	// Incrément du compteur local
	localClock[mySiteID]++
	return localClock
}

// CreateDefaultTab initializes a Tab of length n with default type and zero clock
func CreateDefaultTab(siteID string) StateMap {
	// arr := make(Tab, n)
	// for i := range arr {
	// 	arr[i] = TabElement{Type: MsgReleaseSc, Clock: 0}
	// }
	// return arr
	stateMap := make(map[string]*StateObject)
	stateMap[siteID] = &StateObject{Type: MsgReleaseSc, Clock: 0}
	return stateMap
}

func AddSiteToStateMap(stateMap StateMap, siteID string) {
	if _, exists := stateMap[siteID]; !exists {
		stateMap[siteID] = &StateObject{
			Type:  MsgReleaseSc,
			Clock: 0,
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

// verifyIfMaxNbLinesSite checks if this site has the maximum lines and constructs propagation message
func verifyIfMaxNbLinesSite(arr map[string]int, tab StateMap, myID string, currentText string) string {
	if len(arr) < len(tab) {
		return ""
	}

	var id_max string = ""
	var v_max int = -1
	for id, v := range arr {
		if v > v_max {
			v_max = v
			id_max = id
		}
	}

	if myID == id_max {
		display_d("Sending the text from app because it has the maximum number of lines")
		return msg_format(TypeField, MsgPropagateText) +
			msg_format(SiteIdField, myID) +
			msg_format(UptField, currentText)
	} else {
		return ""
	}
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

// saveCutJson records a vectorial clock under a given cut and action in a JSON file
func saveCutJson(cutNumber string, vectorialClock map[string]int, siteActionNumber string, filePath string) error {
	fichier, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("error opening/creating file: %w", err)
	}
	defer fichier.Close()

	contenu, err := io.ReadAll(fichier)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	var data map[string]interface{}
	if len(contenu) == 0 {
		data = make(map[string]interface{})
	} else {
		err = json.Unmarshal(contenu, &data)
		if err != nil {
			return fmt.Errorf("error while parsing JSON: %w", err)
		}
	}

	// json structure: {cutNumber: {siteActionNumber: vectorialClock}}
	innerMap, ok := data[cutNumber].(map[string]interface{})
	if !ok {
		innerMap = make(map[string]interface{})
		data[cutNumber] = innerMap
	}

	innerMap[siteActionNumber] = vectorialClock

	_, err = fichier.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("error seeking file start: %w", err)
	}

	err = fichier.Truncate(0)
	if err != nil {
		return fmt.Errorf("error truncating file: %w", err)
	}

	modifiedContent, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling JSON: %w", err)
	}

	_, err = fichier.Write(modifiedContent)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	return nil
}
