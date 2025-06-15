package main

import (
	"log"
	"os"
)

var (
	cyan   string = "\033[1;36m"
	raz    string = "\033[0;00m"
	rouge  string = "\033[1;31m"
	orange string = "\033[1;33m"
)

var (
	pid    = os.Getpid()
	stderr = log.New(os.Stderr, "", 0)
)

func display_d(what string) {
	stderr.Printf("%s + [%s %d net] %s%s", cyan, *id, pid, what, raz)
}

func display_w(what string) {
	stderr.Printf("%s * [%s %d net] %s%s", orange, *id, pid, what, raz)
}

func display_e(what string) {
	stderr.Printf("%s ! [%s %d net] %s%s", rouge, *id, pid, what, raz)
}
