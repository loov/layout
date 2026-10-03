// Package text draws laid out graphs with Unicode box-drawing characters
// for terminals, optionally colored with ANSI escape codes. Edges are
// rasterized onto a character grid as horizontal and vertical runs, so
// ortho splines look best; diagonal segments become staircases. Set
// layout.Graph.ForText before laying out for output with room for the
// runs; the extra room is carved away again when writing.
package text

import (
	"io"

	"github.com/loov/layout"
)

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
	paths := make([][][2]int, len(graph.Edges))
	for i, edge := range graph.Edges {
		paths[i] = c.edgeCells(edge)
	}
	c.spreadSides(graph.Edges, paths)
	for i, edge := range graph.Edges {
		c.drawEdge(edge, paths[i])
	}
	c.drawLabels(graph, paths)

	grid := c.grid()
	grid = carve(grid, " │┃┊┋┆", "▲▼●○", 1)
	grid = transpose(carve(transpose(grid), " ─━┈┉┄", "◀▶●○", 2))
	_, err := io.WriteString(w, encode(grid, opts))
	return err
}
