package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func msg_format(key string, val string) string {
	var fieldsep = "/"
	var keyvalsep = "="

	return fieldsep + keyvalsep + key + keyvalsep + val
}

func findval(msg string, key string) string {

	sep := msg[0:1]
	tab_allkeyvals := strings.Split(msg[1:], sep)

	if len(msg) < 4 {
		return ""
	}
	for _, keyval := range tab_allkeyvals {

		equ := keyval[0:1]

		tabkeyval := strings.Split(keyval[1:], equ)
		if tabkeyval[0] == key {

			return tabkeyval[1]
		}
	}
	return ""
}

func recaler(x, y int) int {
	if x < y {
		return y + 1
	}
	return x + 1
}

func main() {
	var nom = "controler"
	var h int = 0

	var rcvmsg string
	var message string

	for i := 0; i < 10; i++ {

		// lecture
		fmt.Scanln(&rcvmsg)

		s_hrcv := findval(rcvmsg, "hlg")
		if s_hrcv != "" {
			hrcv, _ := strconv.Atoi(s_hrcv)
			h = recaler(h, hrcv)
		} else {
			h = h + 1
		}
		tab_allkeyvals := strings.Split(rcvmsg[1:], rcvmsg[0:1])
		fmt.Printf("%q\n", tab_allkeyvals)
		fmt.Println("--------------------------------")

		// separate key and values
		for _, keyval := range tab_allkeyvals {
			tab_keyval := strings.Split(keyval[1:], keyval[0:1])
			fmt.Printf("  %q\n", tab_keyval)
			fmt.Printf("  key : %s  val : %s\n", tab_keyval[0], tab_keyval[1])
		}

		fmt.Println("nom reçu = ", findval(rcvmsg, "sender"))
		fmt.Println("horloge reçue = ", findval(rcvmsg, "hlg"))
		fmt.Println("-------------------------------")

		sndmsg := findval(rcvmsg, "msg")
		if sndmsg == "" {
			fmt.Println(msg_format("msg", rcvmsg) + msg_format("hlg", strconv.Itoa(h)))
		} else {
			fmt.Println(sndmsg)
		}
	}
}
