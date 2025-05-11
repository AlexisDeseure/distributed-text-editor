package controler

func display_d(what string) {
	stderr.Printf("%s + [%d %d] %s%s", cyan, *id, pid, what, raz)
}

func display_w(what string) {
	stderr.Printf("%s * [%d %d] %s%s", orange, *id, pid, what, raz)
}

func display_e(what string) {
    stderr.Printf("%s ! [%d %d] %s%s", rouge, *id, pid, what, raz)
}