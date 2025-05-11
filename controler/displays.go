package controler

var cyan string = "\033[1;36m"
var raz string = "\033[0;00m"
var rouge string = "\033[1;31m"
var orange string = "\033[1;33m"

func display_d(what string) {
	stderr.Printf("%s + [%d %d] %s%s", cyan, *id, pid, what, raz)
}

func display_w(what string) {
	stderr.Printf("%s * [%d %d] %s%s", orange, *id, pid, what, raz)
}

func display_e(what string) {
    stderr.Printf("%s ! [%d %d] %s%s", rouge, *id, pid, what, raz)
}