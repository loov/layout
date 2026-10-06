package hier

import (
	"slices"
	"testing"
)

// TestOrderFlatEdges checks that a chain of flat edges ends up left to
// right, where moving each target after its source one edge at a time
// undoes an earlier move, and that the other nodes keep their order.
func TestOrderFlatEdges(t *testing.T) {
	graph := NewGraph()
	x, a, b, c := graph.AddNode(), graph.AddNode(), graph.AddNode(), graph.AddNode()
	graph.ByRank = []Nodes{{x, c, b, a}}
	graph.Flat = [][2]*Node{{b, c}, {a, b}}

	orderFlatEdges(graph)
	if want := (Nodes{x, a, b, c}); !slices.Equal(graph.ByRank[0], want) {
		t.Errorf("order = %v, want %v", graph.ByRank[0], want)
	}
}
