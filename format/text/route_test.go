package text

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
)

// TestRouteSearches checks that wherever route has to take an edge
// around others, search finds a way rather than route keeping the
// edge's cells, which other edges may share: the Graphviz files, laid out
// for text, as drawGraph draws them with and without spread ends.
func TestRouteSearches(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "graphviz", "*.gv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		for suffix, merge := range map[string]bool{"": false, "_merged": true} {
			name := strings.TrimSuffix(filepath.Base(file), ".gv") + suffix
			t.Run(name, func(t *testing.T) {
				graphs, err := dot.ParseFile(file)
				if err != nil {
					t.Fatal(err)
				}
				graphs[0].MergeEdges = merge
				l, err := layout.Hierarchical(graphs[0], layout.Options{ForText: true})
				if err != nil {
					t.Fatal(err)
				}
				var failed []string
				for _, spread := range []bool{true, false} {
					failed = append(failed, searchFailures(l, spread)...)
				}
				if len(failed) > 0 {
					slices.Sort(failed)
					t.Errorf("search found no way for %q", slices.Compact(failed))
				}
			})
		}
	}
}

// searchFailures draws l as drawGraph does and returns the edges route
// had to take around others but search found no way for
func searchFailures(l *layout.Layout, spread bool) []string {
	graph := l.Graph
	c := newCanvas(l, false)
	c.spread = spread
	for i, cluster := range graph.Clusters {
		if !cluster.Invisible {
			c.drawCluster(i)
		}
	}
	for _, node := range graph.Nodes {
		c.drawNode(node)
	}
	paths := make([][]point, len(graph.Edges))
	for i, edge := range graph.Edges {
		if !edge.Invisible {
			paths[i] = c.edgeCells(edge, l.Edges[i].Path)
		}
	}
	c.spreadSides(graph.Edges, paths)
	c.reserve(graph.Edges, paths)
	var failed []string
	for i, edge := range graph.Edges {
		if edge.Invisible {
			continue
		}
		cells := paths[i]
		if len(cells) >= 2 {
			id, merged := c.edgeOwner(edge, c.ids+1)
			last := len(cells) - 1
			if !c.clear(cells, id, merged) || !c.onSide(cells[0], cells[1], edge.From) || !c.onSide(cells[last], cells[last-1], edge.To) {
				starts := c.endCells(cells[0], cells[1], edge.From, edge.FromField)
				goals := c.endCells(cells[last], cells[last-1], edge.To, edge.ToField)
				if c.search(starts, goals, id) == nil {
					failed = append(failed, fmt.Sprintf("%s->%s", edge.From.ID, edge.To.ID))
				}
			}
		}
		paths[i] = c.route(edge, cells)
		c.drawEdge(edge, l.Edges[i].Path, paths[i])
	}
	return failed
}
