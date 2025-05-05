package main

import (
	"fmt"
	"strconv"
	"time"
)

var fieldsep = "/"
var keyvalsep = "="

func msg_format(key string, val string) string {
	return fieldsep + keyvalsep + key + keyvalsep + val
}

func main() {
	var nom = "ecrivain"
	var h int = 0
	message := "hello"
	for {
		h = h + 1
		// 	//fmt.Printf(",=sender=%s,=hlg=%d\n", nom, h)

		fmt.Println(msg_format("msg", message) + msg_format("hlg", strconv.Itoa(h)) + msg_format("sender", nom))

		time.Sleep(time.Duration(2) * time.Second)
	}
}
