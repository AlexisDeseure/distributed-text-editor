#!/bin/bash

# Handle fyne error (UTF-8 locales)
export LANG=en_US.UTF-8

# Default values
NUM_SITES=""
DEBUG_MODE="false"
FIFO_DIR="/tmp"
OUTPUTS_DIR="$PWD/output"
BASE_PORT=9000
MAX_TARGETS=3
CLEAN_OUTPUT=0

# Array to store site PIDs and timestamps
declare -a SITE_PIDS
declare -a SITE_IDS
declare -a SITE_PORTS

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--num-sites)
            NUM_SITES="$2"
            shift 2
            ;;
        --debug)
            DEBUG_MODE="true"
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
        --base-port)
            BASE_PORT="$2"
            shift 2
            ;;
        --max-targets)
            MAX_TARGETS="$2"
            shift 2
            ;;
        --clean-output)
            CLEAN_OUTPUT=1
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "  -n, --num-sites NUM     Number of sites to create"
            echo "      --debug             Enable debug mode for all sites"
            echo "      --fifo-dir DIR      Directory for FIFOs (default: /tmp)"
            echo "      --output-dir DIR    Directory for outputs (default: ./output)"
            echo "      --base-port PORT    Base port number (default: 9000)"
            echo "      --max-targets NUM   Maximum number of targets per site (default: 3)"
            echo "      --clean-output      Clean output directory before starting"
            echo "  -h, --help              Show this help"
            echo ""
            echo "Example:"
            echo "  $0 --num-sites 5 --debug --max-targets 2"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Get number of sites if not provided
if [ -z "$NUM_SITES" ]; then
    read -p "How many sites do you want to create? " NUM_SITES
fi

# Validate number of sites
if ! [[ "$NUM_SITES" =~ ^[0-9]+$ ]] || [ "$NUM_SITES" -lt 2 ]; then
    echo "Error: Number of sites must be an integer >= 2"
    exit 1
fi

# Display configuration
echo "Configuration:"
echo "  Number of sites: $NUM_SITES"
echo "  Debug mode: $DEBUG_MODE"
echo "  FIFO directory: $FIFO_DIR"
echo "  Output directory: $OUTPUTS_DIR"
echo "  Base port: $BASE_PORT"
echo "  Max targets per site: $MAX_TARGETS"
echo "  Clean output: $CLEAN_OUTPUT"
echo ""

# Clean output directory if requested
if [ "$CLEAN_OUTPUT" -eq 1 ]; then
    echo "Cleaning output directory..."
    rm -rf "$OUTPUTS_DIR"
fi

# Create directories
mkdir -p "$OUTPUTS_DIR"
mkdir -p "$FIFO_DIR"

echo "Building executables..."
go work use
go build -o "$PWD/build/network" ./network
go build -o "$PWD/build/controler" ./controler
go build -o "$PWD/build/app" ./app

if [ $? -ne 0 ]; then
    echo "Error: Failed to build executables"
    exit 1
fi

echo "Build completed successfully."

# Generate site IDs and ports
echo "Generating site configurations..."
for ((i=0; i<NUM_SITES; i++)); do
    SITE_IDS[i]=$(date +%s%N)
    SITE_PORTS[i]=$((BASE_PORT + i))
    # Small delay to ensure unique timestamps
    sleep 0.001
done

# Initialize adjacency matrix
declare -A adjacency_matrix
for ((i=0; i<NUM_SITES; i++)); do
    for ((j=0; j<NUM_SITES; j++)); do
        adjacency_matrix[$i,$j]=0
    done
done

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    for pid in "${SITE_PIDS[@]}"; do
        if [ ! -z "$pid" ]; then
            kill $pid 2>/dev/null
        fi
    done
    
    # Kill any remaining site.sh processes
    pkill -f "site.sh" 2>/dev/null
    
    echo "Cleanup completed."
    exit 0
}

trap cleanup SIGINT SIGTERM

# Launch sites sequentially with realistic topology
echo "Launching sites sequentially with dynamic topology..."

for ((i=0; i<NUM_SITES; i++)); do
    site_id="${SITE_IDS[i]}"
    port="${SITE_PORTS[i]}"
    
    echo "=== Launching Site $i (ID: $site_id, Port: $port) ==="
    
    # Determine targets for this site
    targets=""
    target_count=0
    
    if [ $i -eq 0 ]; then
        # First site (site 0) has no targets - it's the initial node
        echo "  Site 0 is the initial node - no targets"
    else
        # For sites 1 and above, choose random targets from already launched sites (0 to i-1)
        available_sites=()
        for ((j=0; j<i; j++)); do
            available_sites+=($j)
        done
        
        # Choose number of targets (1 to min(MAX_TARGETS, number of available sites))
        max_possible_targets=${#available_sites[@]}
        if [ $MAX_TARGETS -lt $max_possible_targets ]; then
            max_possible_targets=$MAX_TARGETS
        fi
        
        num_targets=$((1 + RANDOM % max_possible_targets))
        echo "  Choosing $num_targets targets from ${#available_sites[@]} available sites: [${available_sites[*]}]"
        
        # Randomly select targets
        selected_targets=()
        for ((k=0; k<num_targets; k++)); do
            if [ ${#available_sites[@]} -eq 0 ]; then
                break
            fi
            
            # Pick random available site
            rand_idx=$((RANDOM % ${#available_sites[@]}))
            target_site=${available_sites[rand_idx]}
            selected_targets+=($target_site)
            
            # Remove from available sites to avoid duplicates
            unset available_sites[rand_idx]
            available_sites=(${available_sites[@]})
        done
        
        # Build targets string and update adjacency matrix
        for target_site in "${selected_targets[@]}"; do
            if [ -z "$targets" ]; then
                targets="localhost:${SITE_PORTS[target_site]}"
            else
                targets="$targets,localhost:${SITE_PORTS[target_site]}"
            fi
            
            # Update adjacency matrix
            adjacency_matrix[$i,$target_site]=1
            adjacency_matrix[$target_site,$i]=1  # Bidirectional for graph visualization
            ((target_count++))
        done
        
        echo "  Selected targets: [${selected_targets[*]}] -> $targets"
    fi
    
    # Build and execute site.sh command
    site_cmd="./site.sh --id \"$site_id\" --document \"$site_id\" --port $port --fifo-dir \"$FIFO_DIR\" --output-dir \"$OUTPUTS_DIR\" --already-built"
    
    if [ ! -z "$targets" ]; then
        site_cmd="$site_cmd --targets \"$targets\""
    fi
    
    if [ "$DEBUG_MODE" = "true" ]; then
        site_cmd="$site_cmd --debug"
    fi
    
    echo "  Command: $site_cmd"
    
    # Start the site in background
    eval "$site_cmd" &
    SITE_PIDS[i]=$!
    
    echo "  Site $i started with PID ${SITE_PIDS[i]}"
    echo ""
    
    # Small delay before starting next site
    sleep 1
done

echo "All sites launched. Final network topology:"
echo "Sites: $NUM_SITES"
echo "Connections (outgoing targets):"
for ((i=0; i<NUM_SITES; i++)); do
    connections=""
    connection_count=0
    
    if [ $i -eq 0 ]; then
        echo "  Site $i -> [No targets - initial node]"
    else
        for ((j=0; j<NUM_SITES; j++)); do
            if [ $i -ne $j ] && [ ${adjacency_matrix[$i,$j]} -eq 1 ]; then
                if [ -z "$connections" ]; then
                    connections="$j"
                else
                    connections="$connections, $j"
                fi
                ((connection_count++))
            fi
        done
        
        if [ $connection_count -eq 0 ]; then
            echo "  Site $i -> [No connections]"
        else
            echo "  Site $i -> [$connections]"
        fi
    fi
done

echo ""
echo "Generating network topology graph..."
cat > "$OUTPUTS_DIR/generate_graph.go" << 'EOF'
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/emicklei/dot"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run generate_graph.go <output_file> [matrix_data]")
		os.Exit(1)
	}
	
	outputFile := os.Args[1]
	
	var matrixData string
	if len(os.Args) >= 3 {
		matrixData = os.Args[2]
	} else {
		// Read from stdin
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Printf("Error reading from stdin: %v\n", err)
			os.Exit(1)
		}
		matrixData = string(data)
	}
	
	// Parse matrix data
	lines := strings.Split(strings.TrimSpace(matrixData), "\n")
	n := len(lines)
	
	matrix := make([][]int, n)
	for i := range matrix {
		matrix[i] = make([]int, n)
		values := strings.Fields(lines[i])
		for j, val := range values {
			if val == "1" {
				matrix[i][j] = 1
			}
		}
	}
	
	g := dot.NewGraph(dot.Undirected)
	nodes := make([]dot.Node, n)
	
	// Get site IDs from environment variable
	siteIdsStr := os.Getenv("SITE_IDS_FOR_GRAPH")
	fmt.Printf("DEBUG: SITE_IDS_FOR_GRAPH env var: '%s'\n", siteIdsStr)
	
	var siteIds []string
	if siteIdsStr != "" {
		siteIds = strings.Split(siteIdsStr, ",")
		fmt.Printf("DEBUG: Parsed %d site IDs\n", len(siteIds))
		for idx, id := range siteIds {
			fmt.Printf("DEBUG: Site %d ID: '%s'\n", idx, id)
		}
	}
	
	for i := range matrix {
		var nodeName string
		if i < len(siteIds) && siteIds[i] != "" {
			nodeName = siteIds[i]
			fmt.Printf("DEBUG: Using site ID '%s' for node %d\n", nodeName, i)
		} else {
			nodeName = fmt.Sprintf("Site_%d", i)
			fmt.Printf("DEBUG: Using default name '%s' for node %d\n", nodeName, i)
		}
		nodes[i] = g.Node(nodeName)
	}

	for i := 0; i < len(matrix); i++ {
		for j := i + 1; j < len(matrix); j++ {
			if matrix[i][j] != 0 {
				g.Edge(nodes[i], nodes[j])
			}
		}
	}

	// Create the PNG file using dot
	cmd := exec.Command("dot", "-Tpng", "-o", outputFile)
	cmd.Stdin = strings.NewReader(g.String())

	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error creating PNG: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Graph generated successfully in %s\n", outputFile)
}
EOF

# Prepare matrix data for graph generation
matrix_data=""
for ((i=0; i<NUM_SITES; i++)); do
    row=""
    for ((j=0; j<NUM_SITES; j++)); do
        row="$row ${adjacency_matrix[$i,$j]}"
    done
    matrix_data="$matrix_data$row"$'\n'
done

# Export site IDs for graph generation (join with commas for better parsing)
site_ids_joined=""
for ((i=0; i<NUM_SITES; i++)); do
    if [ $i -eq 0 ]; then
        site_ids_joined="${SITE_IDS[i]}"
    else
        site_ids_joined="$site_ids_joined,${SITE_IDS[i]}"
    fi
done
export SITE_IDS_FOR_GRAPH="$site_ids_joined"
echo "DEBUG: Exporting SITE_IDS_FOR_GRAPH: ${SITE_IDS_FOR_GRAPH}"

# Generate the graph
cd "$OUTPUTS_DIR"
go mod init graph_generator 2>/dev/null || true
go get github.com/emicklei/dot 2>/dev/null || true

# Pass matrix data via stdin instead of command line argument
echo "$matrix_data" | SITE_IDS_FOR_GRAPH="${SITE_IDS_FOR_GRAPH}" go run generate_graph.go "network_topology.png"

# Clean up generated Go files
rm -f generate_graph.go go.mod go.sum

cd - > /dev/null

echo "Network topology graph created: $OUTPUTS_DIR/network_topology.png"

echo ""
echo "Press Ctrl+C to stop all sites and cleanup."

# Wait for all background processes
wait
