package main

import (
	"log"
	"os"
)

var cyan string = "\033[1;36m"
var raz string = "\033[0;00m"
var rouge string = "\033[1;31m"
var orange string = "\033[1;33m"

var pid = os.Getpid()
var stderr = log.New(os.Stderr, "", 0)

func display_d(what string) {
	stderr.Printf("%s + [%d %d ctl] (%d) %s%s", cyan, *id, pid, h, what, raz)
}

func display_w(what string) {
	stderr.Printf("%s * [%d %d ctl] (%d) %s%s", orange, *id, pid, h, what, raz)
}

func display_e(what string) {
	stderr.Printf("%s ! [%d %d ctl] (%d) %s%s", rouge, *id, pid, h, what, raz)
}
