// Package text draws laid out graphs with Unicode box-drawing characters
// for terminals, optionally colored with ANSI escape codes. Edges are
// rasterized onto a character grid as horizontal and vertical runs, so
// ortho splines look best; diagonal segments become staircases. Lay the
// graph out with layout.Options.ForText, which places everything on the
// character cells that text draws in.
package text

import (
	"cmp"
	"errors"
	"io"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/loov/layout"
)

// Write draws the layout as text. One character cell is
// l.Graph.FontSize*0.55 wide and l.Graph.LineHeight tall.
func Write(w io.Writer, l *layout.Layout) error { return write(w, l, Options{}, false) }

// WriteOptions draws the layout like Write, without colors, with the
// characters that opts allows, see Options.NoLegacyGlyphs.
func WriteOptions(w io.Writer, l *layout.Layout, opts Options) error {
	return write(w, l, opts, false)
}

// WriteColor draws the graph like Write, with the colors set on nodes,
// edges and clusters as ANSI escape codes, see Options. Unset colors are
// left to the terminal, except for lines and text on a fill or
// background, which are black or white, whichever stands out.
func WriteColor(w io.Writer, l *layout.Layout, opts Options) error {
	return write(w, l, opts, true)
}

// errTooLarge reports a graph with more than cells can tell apart
var errTooLarge = errors.New("text: graph too large to draw")

// write draws the graph, with colors when color is set
func write(w io.Writer, l *layout.Layout, opts Options, color bool) error {
	if !fits(l.Graph) {
		return errTooLarge
	}
	c, g := drawn(l, opts.NoLegacyGlyphs)
	g = crop(g)
	colors := &opts
	if !color {
		colors = nil
	}
	_, err := io.WriteString(w, encode(g, c.palette, colors))
	return err
}

// crop returns g without the blank rows above and below the drawing and
// the blank columns left of it; encode leaves out the ones right of it
func crop(g grid) grid {
	blank := func(c cell) bool { return c.r == ' ' && c.bg == 0 && c.style&(underline|strike) == 0 }
	left := math.MaxInt
	top, bottom := len(g), 0
	for y, row := range g {
		if x := slices.IndexFunc(row, func(c cell) bool { return !blank(c) }); x >= 0 {
			left = min(left, x)
			top, bottom = min(top, y), y+1
		}
	}
	if top >= bottom {
		return g[:0]
	}
	g = g[top:bottom]
	for y := range g {
		g[y] = g[y][left:]
	}
	return g
}

// drawn draws the graph as it is laid out and tidies it: edge ends spread
// apart look balanced, but can keep tidying from lining an edge up
// straight, so they stay apart unless that bends edges more, or as much
// on a larger drawing; safe leaves out Symbols for Legacy Computing
func drawn(l *layout.Layout, safe bool) (*canvas, grid) {
	var c *canvas
	var g grid
	for _, spread := range []bool{true, false} {
		d := drawGraph(l, spread, safe)
		if c == nil || better(d.rows, g) {
			c, g = d, d.rows
		}
	}
	g = tidy(g, c.sideways(), c.owners)
	c.frameLabels(g)
	// no margin above
	for len(g) > 1 && !slices.ContainsFunc(g[0], func(c cell) bool { return c.r != ' ' || c.bg != 0 }) {
		g = g[1:]
	}
	return c, g
}

// tidy takes the room out of g that the layout leaves for the runs of
// edges: lines straighten, rows of runs merge, labels hug their lines;
// owners holds the edge of each label id
func tidy(g grid, sideways bool, owners []edgeID) grid {
	var marks rerouteMarks
	for !sideways && unjog(g, &marks) {
	}
	for merged := !sideways; merged; {
		g, merged = mergeRows(g)
	}
	for !sideways && straighten(g) {
	}
	for merged := !sideways; merged; {
		g, merged = mergeRows(g)
	}
	hug(g, owners)
	return g
}

// better reports whether the grid a has fewer runs that edges
// share than b, or as many and fewer bends of edges, or as many in less
// area
func better(a, b grid) bool {
	measure := func(g grid) (shared, bends, area int) {
		width := 0
		for _, row := range g {
			for x, c := range row {
				if c.heavy != 0 {
					shared++
				}
				if !c.solid && roundedCorner(c.r) {
					bends++
				}
				if c.r != ' ' || c.bg != 0 {
					width = max(width, x+1)
				}
			}
		}
		return shared, bends, width * len(g)
	}
	sa, ba, aa := measure(a)
	sb, bb, ab := measure(b)
	return cmp.Or(cmp.Compare(sa, sb), cmp.Compare(ba, bb), cmp.Compare(aa, ab)) < 0
}

// drawGraph draws the graph on a canvas, before tidying; with spread,
// edge ends on a side keep a cell apart where there is room
func drawGraph(l *layout.Layout, spread, safe bool) *canvas {
	graph := l.Graph
	c := newCanvas(l, safe)
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
	c.reserve(graph.Edges, paths)
	for i, edge := range graph.Edges {
		if !edge.Invisible {
			paths[i] = c.route(edge, paths[i])
			c.drawEdge(edge, l.Edges[i].Path, paths[i])
		}
	}
	c.drawLabels(paths)
	return c
}

// fits reports whether the ids of the graph fit the int16 fields of cell:
// edges and clusters share ids, edges merge in at most two groups each,
// an edge has at most three labels, and a cluster label keeps its length; and whether its colors fit the
// palette, at most three an element
func fits(graph *layout.Graph) bool {
	if len(graph.Nodes) >= math.MaxInt16 || 2*len(graph.Edges)+len(graph.Clusters) >= math.MaxInt16 || 3*len(graph.Edges) >= math.MaxInt16 ||
		3*(len(graph.Nodes)+len(graph.Edges)+len(graph.Clusters)) >= math.MaxUint16 {
		return false
	}
	for _, cluster := range graph.Clusters {
		if utf8.RuneCountInString(clusterLabel(cluster))+2 >= math.MaxInt16 {
			return false
		}
	}
	return true
}
