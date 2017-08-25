// +build ignore

package main

import (
	"os"

	"github.com/loov/layout"
	"github.com/loov/layout/format/svg"
)

func main() {
	graph := layout.NewDigraph()
	graph.Edge("A", "B")
	graph.Edge("A", "C")
	graph.Edge("B", "D")
	graph.Edge("C", "D")

	layout.Hierarchical(graph)

	svg.Write(os.Stdout, graph)
}
