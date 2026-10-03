package hier

import "testing"

func TestRankOverlappingSameRankGroupsRemainTogether(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		graph := NewGraph()
		a, b, c, d := graph.AddNode(), graph.AddNode(), graph.AddNode(), graph.AddNode()
		graph.AddEdge(a, d)
		graph.AddEdge(d, c)
		graph.SameRank = []Nodes{{a, b}, {c, b}}
		if reverse {
			graph.SameRank[0], graph.SameRank[1] = graph.SameRank[1], graph.SameRank[0]
		}
		Rank(graph)
		if a.Rank != b.Rank || b.Rank != c.Rank {
			t.Fatalf("reverse=%v: ranks a=%d b=%d c=%d", reverse, a.Rank, b.Rank, c.Rank)
		}
	}
}
