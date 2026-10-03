// Package text draws laid out graphs with Unicode box-drawing characters
// for terminals, optionally colored with ANSI escape codes. Edges are
// rasterized onto a character grid as horizontal and vertical runs, so
// ortho splines look best; diagonal segments become staircases. Call
// Prepare before laying out for output with room for the runs; the extra
// room is carved away again when writing.
package text

import (
	"io"
	"math"

	"github.com/loov/layout"
)

// Prepare sets ortho edges and spacing that leaves rows between ranks
// for horizontal runs and arrowheads: more fan-out needs more rows. In
// sideways layouts it also makes nodes with several edges on a side tall
// enough to keep the edges on separate rows.
func Prepare(graph *layout.Graph) {
	graph.Splines = layout.SplinesOrtho
	if graph.LineHeight <= 0 {
		graph.LineHeight = 16
	}
	fan := map[*layout.Node]int{}
	labels := false
	for _, edge := range graph.Edges {
		fan[edge.From]++
		fan[edge.To]++
		labels = labels || edge.Label != ""
	}
	rows := 2.0
	for _, n := range fan {
		rows = max(rows, 2+math.Sqrt(float64(n)))
	}
	if labels {
		rows += 2
	}
	graph.RowPadding = graph.LineHeight * layout.Length(math.Min(rows, 8))
	graph.NodePadding = graph.LineHeight * 2
	graph.EdgePadding = graph.LineHeight

	// sideways, edges leave and enter nodes on their left and right,
	// which are only a few rows tall; make nodes tall enough for a row
	// per edge on the busier side with a row between them
	if graph.RankDir == layout.LeftToRight || graph.RankDir == layout.RightToLeft {
		ins, outs := map[*layout.Node]int{}, map[*layout.Node]int{}
		for _, edge := range graph.Edges {
			if edge.From != edge.To {
				outs[edge.From]++
				ins[edge.To]++
			}
		}
		for _, node := range graph.Nodes {
			if n := max(ins[node], outs[node]); n > 1 {
				node.Radius.Y = max(node.Radius.Y, graph.LineHeight*(layout.Length(n)+0.5))
				if node.Shape == layout.Circle {
					// drawn as a box either way; a circle would widen as much
					node.Shape = layout.Ellipse
				}
			}
		}
	}
}

// Write draws the laid out graph as text. One character cell is
// graph.FontSize*0.55 wide and graph.LineHeight tall.
func Write(w io.Writer, graph *layout.Graph) error { return write(w, graph, nil) }

// WriteColor draws the graph like Write, with the colors set on nodes,
// edges and clusters as ANSI escape codes, see Options. Unset colors are
// left to the terminal, except for lines and text on a fill or
// background, which are black or white, whichever stands out.
func WriteColor(w io.Writer, graph *layout.Graph, opts Options) error {
	return write(w, graph, &opts)
}

// write draws the graph, colored unless opts is nil
func write(w io.Writer, graph *layout.Graph, opts *Options) error {
	c := newCanvas(graph)
	for _, cluster := range graph.Clusters {
		c.drawCluster(cluster)
	}
	for _, node := range graph.Nodes {
		c.drawNode(graph, node)
	}
	for _, edge := range graph.Edges {
		c.drawEdge(edge)
	}
	c.drawLabels(graph)

	grid := c.grid()
	grid = carve(grid, " │┃┊┋┆", "▲▼●○", 1)
	grid = transpose(carve(transpose(grid), " ─━┈┉┄", "◀▶●○", 2))
	_, err := io.WriteString(w, encode(grid, opts))
	return err
}
