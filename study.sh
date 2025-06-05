#!/bin/bash

APP_NAME="study_app"
SRC_DIR="./study"
BUILD_DIR="./build"
FIFO_DIR="/tmp"
OUTPUTS_DIR="$PWD/output"

cleanup () {
  echo "Nettoyage..."
  killall "$APP_NAME" 2> /dev/null
  killall cat 2> /dev/null
  rm -f "$FIFO_DIR/in_A" "$FIFO_DIR/out_A" "$FIFO_DIR/in_B" "$FIFO_DIR/out_B"
  exit 0
}
trap cleanup SIGINT

# Explicit compilation of the file
echo "Compilation de $APP_NAME..."
mkdir -p "$BUILD_DIR"
go build -o "$BUILD_DIR/$APP_NAME" "$SRC_DIR/main.go"
if [ $? -ne 0 ]; then
  echo "Erreur de compilation. Abandon."
  exit 1
fi

# Create FIFOs if necessary
for name in A B; do
  [ -p "$FIFO_DIR/in_$name"  ] || mkfifo "$FIFO_DIR/in_$name"
  [ -p "$FIFO_DIR/out_$name" ] || mkfifo "$FIFO_DIR/out_$name"
done

# Launch instance A (without target)
"$BUILD_DIR/$APP_NAME" -id A < "$FIFO_DIR/in_A" > "$FIFO_DIR/out_A" &

# Launch instance B (with target A)
"$BUILD_DIR/$APP_NAME" -id B -target A < "$FIFO_DIR/in_B" > "$FIFO_DIR/out_B" &

# Connect pipes between A and B
cat "$FIFO_DIR/out_B" > "$FIFO_DIR/in_A" &
cat "$FIFO_DIR/out_A" > "$FIFO_DIR/in_B" &

echo "Instances launched. Ctrl+C to quit and clean up."
wait
