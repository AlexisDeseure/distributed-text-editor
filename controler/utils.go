package main

import (
	"fmt"
	"strings"
	"math"
	"encoding/json"
	"os"
	"io"
)

var fieldsep = "/"
var keyvalsep = "="

func msg_format(key string, val string) string {
	return fieldsep + keyvalsep + key + keyvalsep + val
}

func resetClock(hlg, hrcv int) int {
	if hlg < hrcv {
		return hrcv + 1
	}
	return hlg + 1
}

func findval(msg string, key string, verbose bool) string {

	if len(msg) < 4 {
		if verbose {
			display_e("Message length too short")
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

func updateVectorialClock(oldVectorialClock []int, newVectorialClock []int) []int {
	for i := range oldVectorialClock {
		oldVectorialClock[i] = int(math.Max(float64(oldVectorialClock[i]), float64(newVectorialClock[i])))
	}
	oldVectorialClock[*id]++
	return oldVectorialClock
}

func CreateDefaultTab(n int) Tab {
	arr := make(Tab, n)
	for i := range arr {
		arr[i] = TabElement{Type: MsgReleaseSc, Clock: 0}
	}
	return arr
}

func timestampComparison(a, b CompareElement) bool {
	if a.Clock < b.Clock {
		return true
	} else if a.Clock == b.Clock && a.Id < b.Id {
		return true
	}
	return false
}

func verifyScApproval(tab Tab) {
	var sndmsg string
	if tab[*id].Type == MsgRequestSc {

		site_elem := CompareElement{Clock: tab[*id].Clock, Id: *id}

		for i, el := range tab {
			inter_elem := CompareElement{Clock: el.Clock, Id: i}
			if i != *id && !timestampComparison(site_elem, inter_elem) {
				return
			}
		}

		sndmsg = msg_format(TypeField, MsgAppStartSc)
		fmt.Println(sndmsg)
		display_d("Entering critical section")
	}

}

func saveCutJson(cutNumber string, vectorialClock []int, siteActionNumber string, filePath string) error {
	display_d(cutNumber)
	fichier, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
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

	// json structur : {cutNumber: {siteActionNumber: vectorialClock}}
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
