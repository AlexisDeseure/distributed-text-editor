package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

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

func main() {
	// var rcvmsg string

	// for {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		rcvmsg := scanner.Text()

		// fmt.Scanln(&rcvmsg)
		fmt.Printf("message reçu : %s \n", rcvmsg)

		// find all keyvals in message
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
	}

}
