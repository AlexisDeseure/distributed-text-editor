#!/bin/bash

APP_NAME="study_app"
SRC_DIR="./study"
BUILD_DIR="./build"
OUTPUTS_DIR="$PWD/output"

# UDP ports
PORT_A=8001
PORT_B=8002

cleanup () {
  echo "[SHELL] Cleanup..."
  killall "$APP_NAME" 2> /dev/null
  exit 0
}
trap cleanup SIGINT

# Explicit compilation of the file
echo "[SHELL] Compiling $APP_NAME..."
mkdir -p "$BUILD_DIR"
go build -o "$BUILD_DIR/$APP_NAME" "$SRC_DIR/main.go"
if [ $? -ne 0 ]; then
  echo "[SHELL] Compilation error. Aborting."
  exit 1
fi

# Launch instance A (listening on port 8001)
echo "[SHELL] Launching site A on port $PORT_A..."
"$BUILD_DIR/$APP_NAME" -id A -port $PORT_A &

# Wait a moment for A to start
sleep 1

# Launch instance B (listening on port 8002, connecting to A on port 8001)
echo "[SHELL] Launching site B on port $PORT_B, connecting to A on port $PORT_A..."
"$BUILD_DIR/$APP_NAME" -id B -port $PORT_B -target-host localhost -target-port $PORT_A &

echo "[SHELL] Instances launched. Site A on port $PORT_A, Site B on port $PORT_B."
echo "[SHELL] Press Ctrl+C to quit and clean up."
wait