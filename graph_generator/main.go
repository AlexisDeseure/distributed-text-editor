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
	} // Check if dot command is available
	_, err := exec.LookPath("dot")
	if err != nil {
		// ANSI color codes
		red := "\033[31m"
		reset := "\033[0m"
		bold := "\033[1m"

		fmt.Printf("%s%sError: 'dot' command not found!%s\n", red, bold, reset)
		fmt.Println("")
		fmt.Printf("%sGraphviz is required to generate the network topology graph.%s\n", red, reset)
		fmt.Printf("%sPlease install Graphviz:%s\n", red, reset)
		fmt.Println("")
		fmt.Println("  On Ubuntu/Debian: sudo apt-get install graphviz")
		fmt.Println("  On CentOS/RHEL:   sudo yum install graphviz")
		fmt.Println("  On macOS:         brew install graphviz")
		fmt.Println("")
		fmt.Printf("%sAfter installation, make sure 'dot' is in your PATH.%s\n", red, reset)
		os.Exit(1)
	}

	// Create the PNG file using dot
	cmd := exec.Command("dot", "-Tpng", "-o", outputFile)
	cmd.Stdin = strings.NewReader(g.String())

	err = cmd.Run()
	if err != nil {
		fmt.Printf("Error creating PNG: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Graph generated successfully in %s\n", outputFile)
}
