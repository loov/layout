// Package text draws laid out graphs with Unicode box-drawing characters
// for terminals, optionally colored with ANSI escape codes. Edges are
// rasterized onto a character grid as horizontal and vertical runs, so
// ortho splines look best; diagonal segments become staircases. Lay the
// graph out with layout.Options.ForText for output with room for the
// runs; the extra room is carved away again when writing.
package text

import (
	"io"

	"github.com/loov/layout"
)

// Write draws the layout as text. One character cell is
// l.Graph.FontSize*0.55 wide and l.Graph.LineHeight tall.
func Write(w io.Writer, l *layout.Layout) error { return write(w, l, nil) }

// WriteColor draws the graph like Write, with the colors set on nodes,
// edges and clusters as ANSI escape codes, see Options. Unset colors are
// left to the terminal, except for lines and text on a fill or
// background, which are black or white, whichever stands out.
func WriteColor(w io.Writer, l *layout.Layout, opts Options) error {
	return write(w, l, &opts)
}

// write draws the graph, colored unless opts is nil
func write(w io.Writer, l *layout.Layout, opts *Options) error {
	graph := l.Graph
	c := newCanvas(l)
	for i, cluster := range graph.Clusters {
		if !cluster.Invisible {
			c.drawCluster(i)
		}
	}
	for _, node := range graph.Nodes {
		c.drawNode(graph, node)
	}
	if graph.MergeEdges {
		c.merged = mergedEdges(l)
	}
	paths := make([][][2]int, len(graph.Edges))
	for i, edge := range graph.Edges {
		if !edge.Invisible {
			paths[i] = c.edgeCells(edge, l.Edges[i].Path)
		}
	}
	c.spreadSides(graph.Edges, paths)
	for i, edge := range graph.Edges {
		if !edge.Invisible {
			c.drawEdge(edge, l.Edges[i].Path, paths[i])
		}
	}
	c.drawLabels(graph, paths)

	_, err := io.WriteString(w, encode(carve(c.rows), opts))
	return err
}

// mergedEdges returns an id for the edges that the layout merged, the
// same for those in a group. An edge merges at its start or its end, not
// both. The ids are negative, apart from those the canvas counts up.
func mergedEdges(l *layout.Layout) map[*layout.Edge]int {
	ids := map[*layout.Edge]int{}
	for i, edge := range l.Graph.Edges {
		if m := l.Edges[i].Merged; m != [2]int{} {
			ids[edge] = -max(m[0], m[1])
		}
	}
	return ids
}
