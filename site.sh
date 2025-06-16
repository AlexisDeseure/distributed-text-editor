#!/bin/bash

# Default values
TARGET_ADDRESSES=""
APP_DEBUG_VALUE="false" # Changed from DEBUG_MODE="", and default to "false"
FIFO_DIR="/tmp"
PORT=9000
OUTPUTS_DIR="$PWD/output"
# nano timestamp for id 
TIMESTAMP_ID=$(date +%s%N)
ALREADY_BUILT=0
DOCUMENT_NAME="New document - $TIMESTAMP_ID"

# Process IDs for the components
NETWORK_PID=""
CONTROLER_PID=""
APP_PID=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --document|-d)
            DOCUMENT_NAME="$2"
            shift 2
            ;;
        --id)
            TIMESTAMP_ID="$2"
            shift 2
            ;;
        --targets|-t)
            TARGET_ADDRESSES="$2"
            shift 2
            ;;
        --debug)
            APP_DEBUG_VALUE="true" # Set to "true" if --debug is passed
            shift
            ;;
        --fifo-dir)
            FIFO_DIR="$2"
            shift 2
            ;;
        --output-dir)
            OUTPUTS_DIR="$2"
            shift 2
            ;;
        --port)
            PORT="$2"
            shift 2
            ;;
        --already-built)
            ALREADY_BUILT=1
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "  -d, --document NAME     Document name"
            echo "  -t, --targets ADDRS     Target addresses (comma-separated host:port)"
            echo "      --debug             Enable debug mode"
            echo "      --fifo-dir DIR      Directory for FIFOs (default: /tmp)"
            echo "      --output-dir DIR    Directory for outputs (default: ./output)"
            echo "      --port PORT         Port for site (default: 9000)"
            echo "      --already-built     Skip build step (use if already built)"
            echo "  -h, --help              Show this help"
            echo ""
            echo "Example:"
            echo "  $0 --document mydoc --targets localhost:8080,192.168.1.10:9000 --debug"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

FLAG_TARGET_ADDRESSES="-targets"
if [ -z "$TARGET_ADDRESSES" ]; then
    FLAG_TARGET_ADDRESSES=""
fi

# Display configuration
echo "Configuration:"
echo "  Document: $DOCUMENT_NAME"
echo "  Targets: $TARGET_ADDRESSES"
echo "  Debug mode: $APP_DEBUG_VALUE" # Reflecting the new variable's meaning
echo "  FIFO directory: $FIFO_DIR"
echo "  Output directory: $OUTPUTS_DIR"
echo "  Port: $PORT"
echo "  Timestamp ID: $TIMESTAMP_ID"
echo ""


# handle fyne error (UTF-8 locales)
export LANG=en_US.UTF-8


cleanup () {
  
  echo "Cleaning up process group for instance $TIMESTAMP_ID..."
  
  # Kill all processes in the current instance
  if [ ! -z "$NETWORK_PID" ]; then
    kill $NETWORK_PID 2> /dev/null
  fi
  if [ ! -z "$CONTROLER_PID" ]; then
    kill $CONTROLER_PID 2> /dev/null
  fi
  if [ ! -z "$APP_PID" ]; then
    kill $APP_PID 2> /dev/null
  fi
  
  #  Kill all processes with the timestamp ID prefix
  pkill -f "${TIMESTAMP_ID}_" 2> /dev/null
  
 
  # Suppression des tubes nommés
  rm -f $FIFO_DIR/${TIMESTAMP_ID}_in_* $FIFO_DIR/${TIMESTAMP_ID}_out_*

  exit 0
}

trap cleanup SIGINT

if [ "$ALREADY_BUILT" -eq 0 ]; then

    # create outputs folder
    mkdir -p "$OUTPUTS_DIR"
    # create fifo directory if it does not exist
    mkdir -p "$FIFO_DIR"
    go work use
    go build -o $PWD/build/network ./network
    go build -o $PWD/build/controler ./controler
    go build -o $PWD/build/app ./app

else
    echo "Skipping build step as --already-built is set."
fi

# create fifo for app, controler and network
for i in $(seq 1 3); do
    mkfifo "$FIFO_DIR/${TIMESTAMP_ID}_in_$i"
    mkfifo "$FIFO_DIR/${TIMESTAMP_ID}_out_$i"
done

# start local network between app, controler and network
"$PWD/build/network" -id "$TIMESTAMP_ID" -port $PORT "$FLAG_TARGET_ADDRESSES" "$TARGET_ADDRESSES" < "$FIFO_DIR/${TIMESTAMP_ID}_in_1" > "$FIFO_DIR/${TIMESTAMP_ID}_out_1" &
NETWORK_PID=$!
"$PWD/build/controler" -id "$TIMESTAMP_ID" < "$FIFO_DIR/${TIMESTAMP_ID}_in_2" > "$FIFO_DIR/${TIMESTAMP_ID}_out_2" &
CONTROLER_PID=$!
"$PWD/build/app" -id "$TIMESTAMP_ID" -o "$OUTPUTS_DIR" -f "$DOCUMENT_NAME" "-debug=${APP_DEBUG_VALUE}" < "$FIFO_DIR/${TIMESTAMP_ID}_in_3" > "$FIFO_DIR/${TIMESTAMP_ID}_out_3" &
APP_PID=$!

# start tee and cat to redirect outputs
cat "$FIFO_DIR/${TIMESTAMP_ID}_out_1" > "$FIFO_DIR/${TIMESTAMP_ID}_in_2" &
cat "$FIFO_DIR/${TIMESTAMP_ID}_out_3" > "$FIFO_DIR/${TIMESTAMP_ID}_in_2" &
cat "$FIFO_DIR/${TIMESTAMP_ID}_out_2" | tee "$FIFO_DIR/${TIMESTAMP_ID}_in_3" > "$FIFO_DIR/${TIMESTAMP_ID}_in_1" &

wait