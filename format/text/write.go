// Package text draws laid out graphs with Unicode box-drawing characters
// for terminals, optionally colored with ANSI escape codes. Edges are
// rasterized onto a character grid as horizontal and vertical runs, so
// ortho splines look best; diagonal segments become staircases. Lay the
// graph out with layout.Options.ForText for output with room for the
// runs; the extra room is carved away again when writing.
package text

import (
	"io"
	"strings"

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
	_, g := carved(l)
	_, err := io.WriteString(w, encode(g, opts))
	return err
}

// carved draws the graph on a canvas and carves it into the grid that
// write encodes
func carved(l *layout.Layout) (*canvas, grid) {
	// edge ends spread apart look balanced, but can keep carving from
	// lining an edge up straight: they stay apart unless that bends
	// edges more, or as much on a larger drawing
	var c *canvas
	var g grid
	for _, spread := range []bool{true, false} {
		d := drawGraph(l, spread)
		carved := carve(d.rows, d.sideways())
		if c == nil || better(carved, g) {
			c, g = d, carved
		}
	}
	c.frameLabels(g)
	return c, g
}

// better reports whether the carved grid a has fewer bends of edges than
// b, or as many in less area
func better(a, b grid) bool {
	measure := func(g grid) (bends, area int) {
		width := 0
		for _, row := range g {
			for x, c := range row {
				if !c.solid && strings.ContainsRune("╭╮╰╯", c.r) {
					bends++
				}
				if c.r != ' ' || c.bg != 0 {
					width = max(width, x+1)
				}
			}
		}
		return bends, width * len(g)
	}
	ba, aa := measure(a)
	bb, ab := measure(b)
	return ba < bb || ba == bb && aa < ab
}

// drawGraph draws the graph on a canvas, before carving; with spread,
// edge ends on a side keep a cell apart where there is room
func drawGraph(l *layout.Layout, spread bool) *canvas {
	graph := l.Graph
	c := newCanvas(l)
	c.spread = spread
	for i, cluster := range graph.Clusters {
		if !cluster.Invisible {
			c.drawCluster(i)
		}
	}
	for _, node := range graph.Nodes {
		c.drawNode(node)
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
	c.drawLabels(paths)
	return c
}
