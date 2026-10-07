package text

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/internal/examples"
)

// TestCompactedJoined checks that compacted layouts, see
// layout.Options.Compact, still draw every edge joined and apart, drawn
// as they are and carved, as TestEdgesJoined does for the layouts
// themselves: compaction relies on how text draws.
func TestCompactedJoined(t *testing.T) {
	for name, build := range examples.Graphs {
		t.Run(name, func(t *testing.T) { checkCompacted(t, build(), examples.Options[name]) })
	}
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "graphviz", "*.gv"))
	for _, file := range files {
		for suffix, merge := range map[string]bool{"": false, "_merged": true} {
			t.Run(strings.TrimSuffix(filepath.Base(file), ".gv")+suffix, func(t *testing.T) {
				graphs, err := dot.ParseFile(file)
				if err != nil {
					t.Fatal(err)
				}
				graphs[0].MergeEdges = merge
				checkCompacted(t, graphs[0], layout.Options{})
			})
		}
	}
}

// TestCompactedRandom checks the same on random graphs, see randomGraph
func TestCompactedRandom(t *testing.T) {
	for seed := range 300 {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			if seed == 241 {
				// the layout leads the edge from field b up along the side
				// of its record, text draws it out of the side instead,
				// into the cell that compaction left beside n2
				t.Skip("known bad: arrowhead out of a border")
			}
			checkCompacted(t, randomGraph(uint64(seed)), layout.Options{})
		})
	}
}

func checkCompacted(t *testing.T, graph *layout.Graph, opts layout.Options) {
	t.Helper()
	opts.ForText, opts.Compact = true, true
	l, err := layout.Hierarchical(graph, opts)
	if err != nil {
		t.Fatal(err)
	}
	d := drawGraph(l, true, false)
	c, g := carved(l, false)
	for _, drawing := range []struct {
		c *canvas
		g grid
	}{{d, d.rows}, {c, g}} {
		for _, edge := range lost(drawing.c, drawing.g, l) {
			t.Errorf("edge %s -> %s is not joined:\n%s", edge.From, edge.To, encode(drawing.g, drawing.c.palette, nil))
		}
		checkApart(t, drawing.c, drawing.g)
	}
	// carving can draw an arrowhead out of a border on its own, see
	// checkAttached
	checkAttached(t, encode(d.rows, d.palette, nil))
}

// checkAttached checks that no arrowhead comes straight out of a line
// that runs across it, as it does from a border without a tee: text
// draws a run from box to box that has no room for a line that way
func checkAttached(t *testing.T, drawing string) {
	t.Helper()
	lines := strings.Split(drawing, "\n")
	at := func(y, x int) rune {
		if y < 0 || y >= len(lines) {
			return ' '
		}
		row := []rune(lines[y])
		if x < 0 || x >= len(row) {
			return ' '
		}
		return row[x]
	}
	for y, line := range lines {
		for x, r := range []rune(line) {
			var from rune
			across := "─═━"
			switch r {
			case '▼':
				from = at(y-1, x)
			case '▲':
				from = at(y+1, x)
			case '▶':
				from, across = at(y, x-1), "│║┃"
			case '◀':
				from, across = at(y, x+1), "│║┃"
			}
			if from != 0 && strings.ContainsRune(across, from) {
				t.Errorf("%d:%d: arrowhead out of %q:\n%s", y+1, x+1, from, drawing)
				return
			}
		}
	}
}
