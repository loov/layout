package layout_test

import (
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/internal/examples"
)

func TestHierarchicalOptions(t *testing.T) {
	// more iterations and no balancing must still give a valid layout
	graph := examples.Graphs["complex"]()
	l, err := layout.Hierarchical(graph, layout.Options{OrderIterations: 100, NoRankBalance: true})
	if err != nil {
		t.Fatal(err)
	}
	for i, edge := range graph.Edges {
		if len(l.Edges[i].Path) < 2 {
			t.Errorf("edge %v has no path", edge)
		}
	}
}
