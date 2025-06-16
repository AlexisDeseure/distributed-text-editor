package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

var (
	fieldsep  = "~"
	keyvalsep = "`"
)

func addWaitingSiteMap(siteID string, conn *net.Conn, addr string) {
	waitingConnections[siteID] = &WaitingObject{
		Conn: conn,
		Addr: addr,
	}
}

// func getLocalIP() string {
// 	addrs, err := net.InterfaceAddrs()
// 	if err != nil {
// 		return "127.0.0.1" // fallback localhost
// 	}
// 	for _, addr := range addrs {
// 		// Check if the address is IP address and not loopback
// 		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
// 			if ip4 := ipnet.IP.To4(); ip4 != nil {
// 				return ip4.String()
// 			}
// 		}
// 	}
// 	return "127.0.0.1" // fallback localhost
// }

func registerConn(addr string, conn net.Conn, connectionsMap *map[string]*net.Conn) {
	if _, exists := (*connectionsMap)[addr]; !exists { // the adress is the time ID
		(*connectionsMap)[addr] = &conn
	}
}
func getAndRemoveConn(addr string, connectionsMap *map[string]*net.Conn) *net.Conn {
	if conn, exists := (*connectionsMap)[addr]; exists {
		delete(*connectionsMap, addr)
		return conn
	}
	return nil
}

func unregisterAllConns(connectionsMap *map[string]*net.Conn) {
	for addr, conn := range *connectionsMap {
		(*conn).Close()
		delete(*connectionsMap, addr)
	}
}
func addKnownSite(id string) {
	if !isKnownSite(id) {
		knownSites = append(knownSites, id)
	}
}

func isKnownSite(id string) bool {
	for _, site := range knownSites {
		if site == id {
			return true
		}
	}
	return false
}

func isConnected(addr string) bool {
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

func prepareWaveMessages(messageID string, color string, msgContent string) string {

	formatedMessageContent, _ := msgToJSON(msgContent, true)
	var sndmsg string = msg_format(TypeField, DiffusionMessage) +
		msg_format(DiffusionStatusID, messageID) +
		msg_format(ColorDiffusion, color) +
		msg_format(MessageContent, formatedMessageContent) +
		msg_format(SiteIdField, *id)

	return sndmsg
}

func sendWaveMessages(neighborhoods map[string]*net.Conn, senderID string, sndmsg string) {
	for timerID, conn := range neighborhoods {
		if conn == nil || *conn == nil {
			display_e("Error sending message to " + timerID + " : connection is nil")
			continue
		}
		if timerID != *id && timerID != senderID {
			_, err := writeToConn(*conn, sndmsg)
			if err != nil {
				display_e("Error sending message to " + timerID + ": " + err.Error())
				continue
			}
		}
	}
}

func processTargetFlags(targetAddrs string) []string {

	var finalAddrs []string
	if targetAddrs == "" {
		return nil
	}

	targetAddrsList := strings.Split(targetAddrs, ",")

	uniqueTargets := make(map[string]struct{}) // To store "host:port" strings for deduplication

	for _, addr := range targetAddrsList {
		_, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			display_e(fmt.Sprintf("Resolving target address %s failed: %v. Skipping this target.", addr, err))
			continue
		}
		uniqueTargets[addr] = struct{}{}
	}

	if len(uniqueTargets) == 0 {
		display_w("No valid target addresses provided. Using local IP as default.")
		return nil
	}
	for addr := range uniqueTargets {
		finalAddrs = append(finalAddrs, addr)
	}

	return finalAddrs
}

func printConnectedSites() {
	mutex.Lock()
	defer mutex.Unlock()

	display_e(fmt.Sprintf("Connected sites (%d total):", len(connectedSites)))
	for addr, conn := range connectedSites {
		if conn != nil && *conn != nil {
			display_e(fmt.Sprintf("  - %s (active)", addr))
		} else {
			display_e(fmt.Sprintf("  - %s (nil connection)", addr))
		}
	}
}

// msgToJSON converts a formatted message to JSON format
func msgToJSON(msg string, verbose bool) (string, error) {
	if len(msg) < 4 {
		if verbose {
			display_e("Message length too short: " + msg)
		}
		return "", fmt.Errorf("message too short")
	}

	sep := msg[0:1]
	tab_allkeyvals := strings.Split(msg[1:], sep)

	keyValueMap := make(map[string]string)

	for _, keyval := range tab_allkeyvals {
		if len(keyval) < 4 {
			continue
		}

		equ := keyval[0:1]
		tabkeyval := strings.Split(keyval[1:], equ)
		if len(tabkeyval) >= 2 {
			keyValueMap[tabkeyval[0]] = tabkeyval[1]
		}
	}

	jsonBytes, err := json.Marshal(keyValueMap)
	if err != nil {
		if verbose {
			display_e("Error marshaling to JSON: " + err.Error())
		}
		return "", err
	}

	return string(jsonBytes), nil
}

// jsonToMsg converts JSON to the standard msg_format used in the system
func jsonToMsg(jsonStr string, verbose bool) (string, error) {
	var keyValueMap map[string]string

	err := json.Unmarshal([]byte(jsonStr), &keyValueMap)
	if err != nil {
		if verbose {
			display_e("Error unmarshaling JSON: " + err.Error())
		}
		return "", err
	}

	var result string
	for key, value := range keyValueMap {
		result += msg_format(key, value)
	}

	return result, nil
}