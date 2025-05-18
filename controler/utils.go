package main

import (
	"fmt"
	"strings"
	"math"
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
