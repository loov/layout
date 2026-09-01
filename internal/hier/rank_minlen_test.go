package hier

import "testing"

// TestRankMinLen checks that an edge is ranked at least MinLen apart and
// that the ranks it crosses survive compaction, so AddVirtuals can fill them.
func TestRankMinLen(t *testing.T) {
	graph := NewGraph()
	a, b, c := graph.AddNode(), graph.AddNode(), graph.AddNode()
	graph.AddEdge(a, b)
	graph.AddEdge(b, c)
	graph.AddEdge(a, c)
	graph.SetMinLen(a, c, 4)

	Rank(graph)
	if span := c.Rank - a.Rank; span < 4 {
		t.Errorf("a->c spans %d ranks, want at least 4", span)
	}
	if b.Rank <= a.Rank || b.Rank >= c.Rank {
		t.Errorf("b is on rank %d, want between %d and %d", b.Rank, a.Rank, c.Rank)
	}
	if len(graph.ByRank) != c.Rank+1 {
		t.Errorf("got %d ranks, want %d", len(graph.ByRank), c.Rank+1)
	}

	AddVirtuals(graph)
	for _, src := range graph.Nodes {
		for _, dst := range src.Out {
			if dst.Rank-src.Rank != 1 {
				t.Errorf("edge %v->%v spans %d ranks after AddVirtuals", src, dst, dst.Rank-src.Rank)
			}
		}
	}
}
