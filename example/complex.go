// +build ignore

package main

import (
	"os"

	"github.com/loov/layout"
	"github.com/loov/layout/format/svg"
)

func main() {
	graph := layout.NewDigraph()
	graph.RowPadding = 30 * layout.Point

	a := graph.Node("A")
	a.Shape = layout.Box
	a.Label = "Lorem\nIpsum\nDolorem"
	a.FillColor = layout.RGB{0xFF, 0xA0, 0x20}

	b := graph.Node("B")
	b.Shape = layout.Ellipse
	b.Label = "Ignitus"
	b.FillColor = layout.HSL{0, 0.7, 0.7}

	c := graph.Node("C")
	c.Shape = layout.Square
	c.FontSize = 12 * layout.Point
	c.FontColor = layout.RGB{0x20, 0x20, 0x20}

	graph.Node("D")

	ab := graph.Edge("A", "B")
	ab.LineWidth = 4 * layout.Point

	ac := graph.Edge("A", "C")
	ac.LineWidth = 4 * layout.Point
	if col, ok := layout.ColorByName("blue"); ok {
		ac.LineColor = col
	}

	bd := graph.Edge("B", "D")
	bd.LineColor = layout.RGB{0xA0, 0xFF, 0xA0}

	graph.Edge("C", "D")
	graph.Edge("D", "A")

	layout.Hierarchical(graph)

	svg.Write(os.Stdout, graph)
}
