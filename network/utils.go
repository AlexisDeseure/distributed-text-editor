package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

var (
	fieldsep  = "~"
	keyvalsep = "`"
)

// msg_format constructs a key-value string using predefined separators
func msg_format(key string, val string) string {
	return fieldsep + keyvalsep + key + keyvalsep + val
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

func prepareWaveMessages(messageID string, color string, senderID *int, receiverID string, msgContent string) string {
	var sndmsg string = msg_format(DiffusionStatusID, messageID) +
		msg_format(ColorDiffusion, color) +
		msg_format(SiteIdField, strconv.Itoa(*senderID)) +
		msg_format(SiteIdDestField, receiverID) + // FIXE ME
		msg_format(MessageContent, msgContent)

	return sndmsg
}

func sendWaveMassages(neighborhoods []string, parent string, sndmsg string, conns map[string]net.Conn) {
	for _, neighbor := range neighborhoods {
		if neighbor != parent {
			conn, ok := conns[neighbor]
			if !ok {
				display_e("No connection found for neighbor: " + neighbor)
				continue
			}
			_, err := conn.Write([]byte(sndmsg))
			if err != nil {
				display_e("Error sending message to " + neighbor + ": " + err.Error())
				continue
			}
		}
	}
}
