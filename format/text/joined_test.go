package text

import (
	"math/bits"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/internal/examples"
)

// notJoined lists the Graphviz files, with _merged for merged edges,
// whose drawings leave edges unjoined, with which; they are skipped until
// the bug is fixed. Remove an entry with its fix.
var notJoined = map[string]string{
	"arrows":                      "12 edges, such as _box -> lbox",
	"arrows_merged":               "12 edges, such as _box -> lbox",
	"hashtable":                   "4 edges, such as node0 -> node1",
	"hashtable_merged":            "node0 -> node5",
	"jcctree":                     "8 edges, such as SPEC -> DEF2",
	"jcctree_merged":              "6 edges, such as SPEC -> DEF2",
	"ldbxtried":                   "n0 -> n448, n448 -> n449",
	"ldbxtried_merged":            "n0 -> n448, n448 -> n449",
	"Linux_kernel_diagram":        "14 edges, such as system -> system_",
	"Linux_kernel_diagram_merged": "14 edges, such as system -> system_",
	"NaN_merged":                  "Target -> TargetF",
	"pgram":                       "the three Parallelogram -> Octagon edges",
	"pgram_merged":                "the three Parallelogram -> Octagon edges",
	"sdh":                         "8 edges, such as prTTP_4_2 -> prTTP_5_1",
	"sdh_merged":                  "8 edges, such as prTTP_4_2 -> prTTP_5_1",
	"shells":                      "6 edges, such as 1976 -> 1978",
	"shells_merged":               "6 edges, such as 1976 -> 1978",
	"table":                       "struct1 -> struct2",
	"table_merged":                "struct1 -> struct2",
	"UML_Class_diagram":           "5 edges, such as Interface1 -> Class1",
	"UML_Class_diagram_merged":    "Class1 -> System_1, System_1 -> Subsystem_3",
}

// TestEdgesJoined checks that every drawing joins the two nodes of every
// edge with its line, after carving and straightening have reworked the
// cells: the examples and the Graphviz files, laid out for text as their
// .txt files in testdata are, with merged edges too.
func TestEdgesJoined(t *testing.T) {
	check := func(t *testing.T, graph *layout.Graph, opts layout.Options) {
		opts.ForText = true
		l, err := layout.Hierarchical(graph, opts)
		if err != nil {
			t.Fatal(err)
		}
		c, g := carved(l)
		for _, edge := range lost(c, g, l) {
			t.Errorf("edge %s -> %s is not joined:\n%s", edge.From, edge.To, encode(g, nil))
		}
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

// lost returns the visible edges whose line in grid doesn't join their
// nodes: no run of cells joined by the edge's arms touches both, right
// beside them or past an arrowhead or marker beside them
func lost(c *canvas, g grid, l *layout.Layout) []*layout.Edge {
	steps := map[uint8][2]int{up: {-1, 0}, down: {1, 0}, left: {0, -1}, right: {0, 1}}
	back := map[uint8]uint8{up: down, down: up, left: right, right: left}
	// owns reports whether the edge id draws the arm of the cell
	owns := func(p *cell, arm uint8, id int32) bool {
		return p != nil && p.lines&arm != 0 && p.owner[bits.TrailingZeros(uint(arm))] == id
	}
	// touches reports whether the cell is beside the box of the node, or
	// beside a marker that is
	touches := func(r, x int, node int32) bool {
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
func ownsAny(p *cell, id int32) bool {
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
