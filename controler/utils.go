package controler

import (
	"strings"
)

func msg_format(key string, val string) string {
	return fieldsep + keyvalsep + key + keyvalsep + val
}

func recaler(h, hrcv int) int {
	if h < hrcv {
		return hrcv + 1
	}
	return h + 1
}

func findval(msg string, key string) string {

	if len(msg) < 4 {
		display_e("Message length too short")
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
	display_w("No values found")
	return ""
}
