#!/bin/bash

APP_NAME="study_app"
SRC_DIR="./study"
BUILD_DIR="./build"
OUTPUTS_DIR="$PWD/output"

# UDP ports
PORT_A=8001
PORT_B=8002
PORT_C=8003
PORT_D=8004

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

# Launch instance A 
echo "[SHELL] Launching site A on port $PORT_A..."
"$BUILD_DIR/$APP_NAME" -id A -port $PORT_A &
sleep 4
echo "[SHELL] Launching site B on port $PORT_B, connecting to A on port $PORT_A..."
"$BUILD_DIR/$APP_NAME" -id B -port $PORT_B -target-hosts localhost -target-ports $PORT_A &
sleep 4
echo "[SHELL] Launching site C on port $PORT_C, connecting to A on ports $PORT_A and $PORT_B..."
"$BUILD_DIR/$APP_NAME" -id C -port $PORT_C -target-hosts localhost,localhost -target-ports $PORT_A,$PORT_B &
sleep 4
echo "[SHELL] Launching site D on port $PORT_D, connecting to A on port $PORT_C..."
"$BUILD_DIR/$APP_NAME" -id D -port $PORT_D -target-hosts localhost -target-ports $PORT_C &

echo "[SHELL] Instances launched. Site A on port $PORT_A, Site B on port $PORT_B, Site C on port $PORT_C, Site D on port $PORT_D."
echo "[SHELL] Press Ctrl+C to quit and clean up."
wait