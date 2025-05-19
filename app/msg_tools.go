package main

import (
	"fmt"
	"strings"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strconv"
)

var fieldsep = "/"
var keyvalsep = "="

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

func GetNextCutNumber(filePath string) (string, error) {
	data, err := ioutil.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return "cut_number_1", fmt.Errorf("failed to read file: %w", err)
	}

	var cuts map[string]interface{} // Use interface{} as values might not strictly be strings
	err = json.Unmarshal(data, &cuts)
	if err != nil {
		return "cut_number_1", fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	maxCut := -1
	for key := range cuts {
		if !strings.HasPrefix(key, "cut_number_") {
			continue
		}

		parts := strings.Split(key, "_")
		if len(parts) != 3 {
			continue
		}

		numStr := parts[2]
		num, err := strconv.Atoi(numStr)
		if err != nil {
			// Ignore keys where the number part is not a valid integer
			continue
		}

		if num > maxCut {
			maxCut = num
		}
	}

	nextCut := maxCut + 1
	return fmt.Sprintf("cut_number_%d", nextCut), nil
}
