package layout_test

import (
	"strings"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
)

// TestMinLen checks that an edge with MinLen spans that many ranks.
func TestMinLen(t *testing.T) {
	span := func(minlen int) layout.Length {
		graph := layout.NewDigraph()
		edge := graph.Edge("A", "B")
		edge.MinLen = minlen
		if err := layout.Hierarchical(graph); err != nil {
			t.Fatal(err)
		}
		return graph.NodeByID["B"].Center.Y - graph.NodeByID["A"].Center.Y
	}

	one := span(0) // default
	if got := span(1); got != one {
		t.Errorf("minlen=1 spans %v, want the default %v", got, one)
	}
	if got := span(3); got <= one {
		t.Errorf("minlen=3 spans %v, want more than the %v of minlen=1", got, one)
	}
}

// TestMinLenDot checks that the dot parser picks up minlen.
func TestMinLenDot(t *testing.T) {
	graphs, err := dot.Parse(strings.NewReader(`digraph { A -> B [minlen=4]; }`))
	if err != nil {
		t.Fatal(err)
	}
	if got := graphs[0].Edges[0].MinLen; got != 4 {
		t.Errorf("minlen is %v, want 4", got)
	}
}
