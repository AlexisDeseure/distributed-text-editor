package main

import (
	"fmt"
	"os"
	"strings"
)

var (
	fieldsep  = "~"
	keyvalsep = "`"
)

func msg_format(key string, val string) string {
	return fieldsep + keyvalsep + key + keyvalsep + val
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

func getCurrentTextContentFormated() string {
	content, err := os.ReadFile(localSaveFilePath)
	if err != nil {
		display_e(fmt.Sprintf("Failed to read file %s: %v", localSaveFilePath, err))
		return ""
	}
	// "\n" cannot be sent to the standard output without being misinterpreted
	formatted := strings.ReplaceAll(string(content), "\n", "↩")
	return formatted
}
