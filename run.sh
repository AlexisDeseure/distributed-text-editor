#!/bin/bash

# handle fyne error (UTF-8 locales)
export LANG=en_US.UTF-8

# base directory for FIFOs
FIFO_DIR="/tmp"

cleanup () {
  # Suppression des processus de l'application app
  killall app 2> /dev/null

  # Suppression des processus de l'application ctl
  killall controler 2> /dev/null
 
  # Suppression des processus tee et cat
  killall tee 2> /dev/null
  killall cat 2> /dev/null
 
  # Suppression des tubes nommés
  \rm -f $FIFO_DIR/in_* $FIFO_DIR/out_*
  exit 0
}

trap cleanup SIGINT

# determine number of sites (N)
if [ -n "$1" ]; then
    N="$1"
else
    read -p "How many instances do you want to run? " N
fi

# validate input (integer >= 2)
if ! [[ "$N" =~ ^[0-9]+$ ]] || [ "$N" -lt 2 ]; then
    echo "Error: Please enter a valid integer greater than or equal to 2."
    exit 1
fi

# build Go executables
go work use
go build -o build/controler ./controler
go build -o build/app ./app

# create FIFOs for each app and controller only if they don't already exist
for (( i=0; i< N; i++ )); do
    [ -p "$FIFO_DIR/in_A$i" ] || mkfifo "$FIFO_DIR/in_A$i"
    [ -p "$FIFO_DIR/out_A$i" ] || mkfifo "$FIFO_DIR/out_A$i"
    [ -p "$FIFO_DIR/in_C$i" ] || mkfifo "$FIFO_DIR/in_C$i"
    [ -p "$FIFO_DIR/out_C$i" ] || mkfifo "$FIFO_DIR/out_C$i"
done

# start all apps and controllers
for (( i=0; i< N; i++ )); do
    # launch application with its ID
    "$PWD/build/app" -id "$i" < "$FIFO_DIR/in_A$i" > "$FIFO_DIR/out_A$i" &
    # launch controller with its ID and total N
    "$PWD/build/controler" -id "$i" -N "$N" < "$FIFO_DIR/in_C$i" > "$FIFO_DIR/out_C$i" &
done

# wire the ring: each app output feeds its controller; each controller output tees to its app and to next controller
for (( i=0; i< N; i++ )); do
    next=$(( (i + 1) % N ))
    # app -> its controller
    cat "$FIFO_DIR/out_A$i" > "$FIFO_DIR/in_C$i" &
    # controller -> its app and next controller
    cat "$FIFO_DIR/out_C$i" | tee "$FIFO_DIR/in_A$i" > "$FIFO_DIR/in_C$next" &
done

# wait for all background processes
wait
