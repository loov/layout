// +build ignore

package main

import (
	"os"

	"github.com/loov/layout"
	"github.com/loov/layout/format/svg"
)

func main() {
	graph := layout.NewDigraph()
	graph.Node("A")
	graph.Node("B")
	graph.Node("C")
	graph.Node("D")
	graph.Edge("A", "B")
	graph.Edge("A", "C")
	graph.Edge("B", "D")
	graph.Edge("C", "D")
	graph.Edge("D", "A")

	layout.Hierarchical(graph)

	svg.Write(os.Stdout, graph)
}
