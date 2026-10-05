package hier

import (
	"slices"
	"testing"
)

// TestOrderRanksChains checks that a long edge beside a path moves to the
// other side of the path, out of the way of an edge leaving the path,
// which no single node move fixes.
func TestOrderRanksChains(t *testing.T) {
	graph := NewGraph()
	a0, a1, a2, a3, b3 := graph.AddNode(), graph.AddNode(), graph.AddNode(), graph.AddNode(), graph.AddNode()
	graph.AddEdge(a0, a1)
	graph.AddEdge(a1, a2)
	graph.AddEdge(a2, a3)
	graph.AddEdge(a0, a3)
	graph.AddEdge(a1, b3)
	graph.SetMinLen(a1, b3, 2)
	Rank(graph)
	AddVirtuals(graph)

	// the long edges a0->a3 and a1->b3 go right of a1 and a2
	v1, v2 := a0.Out[1], a0.Out[1].Out[0]
	w := a1.Out[1]
	graph.ByRank[1] = Nodes{a1, v1}
	graph.ByRank[2] = Nodes{a2, v2, w}
	graph.ByRank[3] = Nodes{a3, b3}
	if got := graph.TotalCrossings(); got != 1 {
		t.Fatalf("crossings before = %v, want 1", got)
	}

	OrderRanksChains(graph)
	if got := graph.TotalCrossings(); got != 0 {
		t.Errorf("crossings after = %v, want 0", got)
	}
	if !slices.Equal(graph.ByRank[1], Nodes{v1, a1}) || !slices.Equal(graph.ByRank[2], Nodes{v2, a2, w}) {
		t.Errorf("order = %v %v, want the chain left of a1 and a2", graph.ByRank[1], graph.ByRank[2])
	}
}
