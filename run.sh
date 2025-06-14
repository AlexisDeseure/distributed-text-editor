#!/bin/bash

# handle fyne error (UTF-8 locales)
export LANG=en_US.UTF-8

# base directory for FIFOs
FIFO_DIR="/tmp"
OUTPUTS_DIR="$PWD/output"

cleanup () {
  # Suppression des processus de l'application app
  killall app 2> /dev/null

  # Suppression des processus de l'application ctl
  killall controler 2> /dev/null

  # Suppression des processus de l'application network
  killall network 2> /dev/null
 
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

if [ -n "$2" ]; then
    output_deletion="$2"
else 
    read -p "Do you want to delete the outputs folder? (1 for yes): " output_deletion
fi

if [ -n "$3" ]; then
    DEBUG_MODE="$3"
else 
    read -p "Do you want to run in debug mode (save button)? (1 for yes): " DEBUG_MODE
fi

if [ "$DEBUG_MODE" -eq 1 ]; then
    DEBUG_MODE="true"
else
    DEBUG_MODE="false"
fi

if [ "$output_deletion" -eq 1 ]; then
    echo "Deleting outputs folder..."
    rm -rf "$OUTPUTS_DIR"
    echo "Folder deletion complete."
fi
# validate input (integer >= 2)
if ! [[ "$N" =~ ^[0-9]+$ ]] || [ "$N" -lt 2 ]; then
    echo "Error: Please enter a valid integer greater than or equal to 2."
    exit 1
fi

# build Go executables
go work use
go build -o build/network ./network
go build -o build/controler ./controler
go build -o build/app ./app

# create FIFOs for each app and controller and network only if they don't already exist
for (( i=0; i< N; i++ )); do
    [ -p "$FIFO_DIR/in_A$i" ] || mkfifo "$FIFO_DIR/in_A$i"
    [ -p "$FIFO_DIR/out_A$i" ] || mkfifo "$FIFO_DIR/out_A$i"
    [ -p "$FIFO_DIR/in_C$i" ] || mkfifo "$FIFO_DIR/in_C$i"
    [ -p "$FIFO_DIR/out_C$i" ] || mkfifo "$FIFO_DIR/out_C$i"
    [ -p "$FIFO_DIR/in_N$i" ] || mkfifo "$FIFO_DIR/in_N$i"
    [ -p "$FIFO_DIR/out_N$i" ] || mkfifo "$FIFO_DIR/out_N$i"
done

# start sites progressively for random topology
# Site A (id=0) starts first
echo "Starting Site A (id=0)..."
"$PWD/build/app" -id "0" -o "$OUTPUTS_DIR" -debug="$DEBUG_MODE" < "$FIFO_DIR/in_A0" > "$FIFO_DIR/out_A0" &
"$PWD/build/controler" -id "0" -N "$N" < "$FIFO_DIR/in_C0" > "$FIFO_DIR/out_C0" &
"$PWD/build/network" -id "0" -N "$N" < "$FIFO_DIR/in_N0" > "$FIFO_DIR/out_N0" &

# wire Site A
cat "$FIFO_DIR/out_A0" > "$FIFO_DIR/in_C0" &
cat "$FIFO_DIR/out_C0" | tee "$FIFO_DIR/in_A0" > "$FIFO_DIR/in_N0" &
cat "$FIFO_DIR/out_N0" > "$FIFO_DIR/in_C0" &

sleep 1

# start remaining sites progressively
for (( i=1; i< N; i++ )); do
    echo "Starting Site $(echo $i | tr '0123456789' 'ABCDEFGHIJ') (id=$i)..."
    
    # launch application, controller, and network with their IDs
    "$PWD/build/app" -id "$i" -o "$OUTPUTS_DIR" -debug="$DEBUG_MODE" < "$FIFO_DIR/in_A$i" > "$FIFO_DIR/out_A$i" &
    "$PWD/build/controler" -id "$i" -N "$N" < "$FIFO_DIR/in_C$i" > "$FIFO_DIR/out_C$i" &
    "$PWD/build/network" -id "$i" -N "$N" < "$FIFO_DIR/in_N$i" > "$FIFO_DIR/out_N$i" &
    
    # wire the new site
    cat "$FIFO_DIR/out_A$i" > "$FIFO_DIR/in_C$i" &
    cat "$FIFO_DIR/out_C$i" | tee "$FIFO_DIR/in_A$i" > "$FIFO_DIR/in_N$i" &
    cat "$FIFO_DIR/out_N$i" > "$FIFO_DIR/in_C$i" &
    
    # wait a bit before starting the next site to allow connections to establish
    sleep 1
done

# wait for all background processes
wait
