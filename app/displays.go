package main

import (
	"log"
	"os"
)

var (
	cyan   string = "\033[1;36m"
	raz    string = "\033[0;00m"
	red    string = "\033[1;31m"
	orange string = "\033[1;33m"
)

var (
	pid    = os.Getpid()
	stderr = log.New(os.Stderr, "", 0)
)

func display_d(what string) {
	stderr.Printf("%s + [%s %d app] %s%s", cyan, *id, pid, what, raz)
}

func display_w(what string) {
	stderr.Printf("%s * [%s %d app] %s%s", orange, *id, pid, what, raz)
}

func display_e(what string) {
	stderr.Printf("%s ! [%s %d app] %s%s", red, *id, pid, what, raz)
}
