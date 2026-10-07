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
}
