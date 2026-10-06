package text

import (
	"fmt"
	"math/bits"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/internal/examples"
)

// notJoined lists the Graphviz files, with _merged for merged edges,
// whose drawings leave edges unjoined, with which; they are skipped until
// the bug is fixed. Remove an entry with its fix.
var notJoined = map[string]string{}

// TestEdgesJoined checks that every drawing joins the two nodes of every
// edge with its line, and runs no two edges along the same cells, after
// carving and straightening have reworked the cells: the examples and the
// Graphviz files, laid out for text as their .txt files in testdata are,
// with merged edges too.
func TestEdgesJoined(t *testing.T) {
	check := func(t *testing.T, graph *layout.Graph, opts layout.Options) {
		opts.ForText = true
		l, err := layout.Hierarchical(graph, opts)
		if err != nil {
			t.Fatal(err)
		}
		c, g := carved(l, false)
		for _, edge := range lost(c, g, l) {
			t.Errorf("edge %s -> %s is not joined:\n%s", edge.From, edge.To, encode(g, c.palette, nil))
		}
		// a run that edges share can't tell them apart, nor which
		// arrowhead is whose; merged edges are one edge
		checkApart(t, c, g)
	}
	for name, build := range examples.Graphs {
		t.Run(name, func(t *testing.T) { check(t, build(), examples.Options[name]) })
	}
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "graphviz", "*.gv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		for suffix, merge := range map[string]bool{"": false, "_merged": true} {
			name := strings.TrimSuffix(filepath.Base(file), ".gv") + suffix
			t.Run(name, func(t *testing.T) {
				if reason, ok := notJoined[name]; ok {
					t.Skip("known bad: " + reason)
				}
				graphs, err := dot.ParseFile(file)
				if err != nil {
					t.Fatal(err)
				}
				graphs[0].MergeEdges = merge
				check(t, graphs[0], layout.Options{})
			})
		}
	}
}

// checkApart fails t when edges in the grid share a run, which can't
// tell them apart, nor which arrowhead is whose; merged edges are one
// edge
func checkApart(t *testing.T, c *canvas, g grid) {
	t.Helper()
	var shared []string
	for r, row := range g {
		for x, p := range row {
			if p.heavy != 0 {
				shared = append(shared, fmt.Sprintf("%d:%d", r+1, x+1))
			}
		}
	}
	if len(shared) > 0 {
		t.Errorf("edges share cells at %s:\n%s", strings.Join(shared, " "), encode(g, c.palette, nil))
	}
}

// TestEdgesApartRandom checks that random graphs, with ports, fields,
// loops, labels and merged edges, in every direction, draw no two edges
// along the same cells
func TestEdgesApartRandom(t *testing.T) {
	seeds := 300
	if testing.Short() {
		seeds = 30
	}
	for seed := range seeds {
		rng := rand.New(rand.NewPCG(uint64(seed), 0))
		graph := layout.NewDigraph()
		graph.RankDir = []layout.RankDir{layout.TopToBottom, layout.LeftToRight, layout.BottomToTop, layout.RightToLeft}[rng.IntN(4)]
		graph.MergeEdges = rng.IntN(3) == 0
		n := 3 + rng.IntN(8)
		for i := range n {
			node := graph.Node(fmt.Sprint("n", i))
			if rng.IntN(4) == 0 {
				node.Shape = layout.Record
				node.Label = "<a> a|<b> b|<c> c"
			}
		}
		ports := []layout.Compass{layout.CompassAuto, layout.North, layout.South, layout.East, layout.West}
		for range n + rng.IntN(2*n) {
			edge := graph.Edge(fmt.Sprint("n", rng.IntN(n)), fmt.Sprint("n", rng.IntN(n)))
			if rng.IntN(5) == 0 {
				edge.Label = "label"
			}
			if rng.IntN(5) == 0 && edge.From.Shape == layout.Record {
				edge.FromField = "abc"[rng.IntN(3):][:1]
			}
			if rng.IntN(4) == 0 {
				edge.FromPort, edge.ToPort = ports[rng.IntN(len(ports))], ports[rng.IntN(len(ports))]
			}
		}
		l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
		if err != nil {
			t.Fatal(err)
		}
		c, g := carved(l, false)
		t.Run(fmt.Sprint(seed), func(t *testing.T) { checkApart(t, c, g) })
	}
}

// lost returns the visible edges whose line in grid doesn't join their
// nodes: no run of cells joined by the edge's arms touches both, right
// beside them or past an arrowhead or marker beside them
func lost(c *canvas, g grid, l *layout.Layout) []*layout.Edge {
	steps := map[uint8][2]int{up: {-1, 0}, down: {1, 0}, left: {0, -1}, right: {0, 1}}
	back := map[uint8]uint8{up: down, down: up, left: right, right: left}
	// owns reports whether the edge id draws the arm of the cell
	owns := func(p *cell, arm uint8, id edgeID) bool {
		return p != nil && p.lines&arm != 0 && p.owner[bits.TrailingZeros(uint(arm))] == id
	}
	// touches reports whether the cell is beside the box of the node, or
	// beside a marker that is
	touches := func(r, x int, node nodeID) bool {
		for _, s := range steps {
			q := g.at(r+s[0], x+s[1])
			if q == nil || !q.solid {
				continue
			}
			if q.node == node {
				return true
			}
			if q.node == 0 {
				for _, t := range steps {
					if n := g.at(r+s[0]+t[0], x+s[1]+t[1]); n != nil && n.node == node {
						return true
					}
				}
			}
		}
		return false
	}
	var out []*layout.Edge
	for _, edge := range l.Graph.Edges {
		id, ok := c.drawn[edge]
		if !ok || edge.Invisible {
			continue
		}
		from, to := c.nodes[edge.From], c.nodes[edge.To]
		seen := map[[2]int]bool{}
		joined := false
		for r := range g {
			for x := range g[r] {
				if seen[[2]int{r, x}] || !ownsAny(g.at(r, x), id) {
					continue
				}
				// the run of the edge's cells joined to this one
				a, b := false, false
				stack := [][2]int{{r, x}}
				seen[[2]int{r, x}] = true
				for len(stack) > 0 {
					p := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					a = a || touches(p[0], p[1], from)
					b = b || touches(p[0], p[1], to)
					for arm, s := range steps {
						n := [2]int{p[0] + s[0], p[1] + s[1]}
						if !seen[n] && owns(g.at(p[0], p[1]), arm, id) && owns(g.at(n[0], n[1]), back[arm], id) {
							seen[n] = true
							stack = append(stack, n)
						}
					}
				}
				joined = joined || a && b
			}
		}
		// an edge too short for a line has only its marker between them
		for r := range g {
			for x := range g[r] {
				joined = joined || g[r][x].node == from && touches(r, x, to)
			}
		}
		if !joined {
			out = append(out, edge)
		}
	}
	return out
}

// ownsAny reports whether the edge id draws any arm of the cell
func ownsAny(p *cell, id edgeID) bool {
	if p == nil {
		return false
	}
	for arm := range 4 {
		if p.lines&(1<<arm) != 0 && p.owner[arm] == id {
			return true
		}
	}
	return false
}

// TestNoLegacyGlyphs checks that the Graphviz files with diamonds, drawn
// with Options.NoLegacyGlyphs, use no characters of Symbols for Legacy
// Computing, and join every edge without sharing runs.
func TestNoLegacyGlyphs(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "graphviz", "*.gv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		graphs, err := dot.ParseFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.ContainsFunc(graphs[0].Nodes, func(n *layout.Node) bool { return n.Shape == layout.Diamond }) {
			continue
		}
		for suffix, merge := range map[string]bool{"": false, "_merged": true} {
			t.Run(strings.TrimSuffix(filepath.Base(file), ".gv")+suffix, func(t *testing.T) {
				graph := graphs[0]
				graph.MergeEdges = merge
				l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
				if err != nil {
					t.Fatal(err)
				}
				c, g := carved(l, true)
				for _, edge := range lost(c, g, l) {
					t.Errorf("edge %s -> %s is not joined:\n%s", edge.From, edge.To, encode(g, c.palette, nil))
				}
				checkApart(t, c, g)
				var buf strings.Builder
				if err := WriteOptions(&buf, l, Options{NoLegacyGlyphs: true}); err != nil {
					t.Fatal(err)
				}
				if i := strings.IndexFunc(buf.String(), func(r rune) bool { return r >= 0x1FB00 && r <= 0x1FBFF }); i >= 0 {
					t.Errorf("draws %q:\n%s", []rune(buf.String()[i:])[0], buf.String())
				}
			})
		}
	}
}
