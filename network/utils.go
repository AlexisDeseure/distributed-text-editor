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

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1" // fallback localhost
	}
	for _, addr := range addrs {
		// Check if the address is IP address and not loopback
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				return ip4.String()
			}
		}
	}
	return "127.0.0.1" // fallback localhost
}

func registerConn(addr string, conn net.Conn) {
	mutex.Lock()
	defer mutex.Unlock()
	if _, exists := connectedSites[addr]; !exists {
		connectedSites[addr] = &conn
	}
}

func isConnected(addr string) bool {
	mutex.Lock()
	defer mutex.Unlock()
	_, exists := connectedSites[addr]
	return exists
}

func writeToConn(conn net.Conn, msg string) (int, error) {
	return conn.Write([]byte(msg + "\n"))
}

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

func prepareWaveMessages(messageID string, color string, senderID int, receiverID string, msgContent string) string {
	var sndmsg string = msg_format(DiffusionStatusID, messageID) +
		msg_format(ColorDiffusion, color) +
		msg_format(SiteIdField, strconv.Itoa(senderID)) +
		msg_format(SiteIdDestField, receiverID) + // FIXE ME
		msg_format(MessageContent, msgContent)

	return sndmsg
}

func sendWaveMassages(neighborhoods map[string]*net.Conn, parent string, sndmsg string) {
	for timerID, conn := range neighborhoods {
		if conn != nil && *conn != nil {
			if timerID != parent {
				_, err := writeToConn(*conn, sndmsg)
				if err != nil {
					display_e("Error sending message to " + timerID + ": " + err.Error())
					continue
				}
			}
		}
	}
}
