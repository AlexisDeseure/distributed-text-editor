package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/emicklei/dot"
)

func main() {
	matrice := [][]int{
		{1, 1, 1, 1},
		{1, 1, 1, 1},
		{1, 1, 1, 1},
		{1, 1, 1, 1},
	}

	g := dot.NewGraph(dot.Undirected)
	nodes := make([]dot.Node, len(matrice))
	for i := range matrice {
		nodes[i] = g.Node(fmt.Sprintf("%d", i))
	}

	for i := 0; i < len(matrice); i++ {
		for j := i + 1; j < len(matrice); j++ {
			if matrice[i][j] != 0 {
				g.Edge(nodes[i], nodes[j])
			}
		}
	}

	fmt.Println(g.String())

	// Créer le fichier PNG en utilisant dot
	cmd := exec.Command("dot", "-Tpng", "-o", "graphe.png")
	cmd.Stdin = strings.NewReader(g.String())

	err := cmd.Run()
	if err != nil {
		fmt.Printf("Erreur lors de la création du PNG: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Graphe généré avec succès dans graphe.png")
}
