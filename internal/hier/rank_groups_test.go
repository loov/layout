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

// TestRankSameRankCycleKeepsPins checks that a cycle made by contracting a
// same-rank group doesn't break the ranking: r -> p contradicts the group
// {q, r} and is dropped, while every other edge and the MaxRank pin hold.
func TestRankSameRankCycleKeepsPins(t *testing.T) {
	graph := NewGraph()
	p, q, r, s, m := graph.AddNode(), graph.AddNode(), graph.AddNode(), graph.AddNode(), graph.AddNode()
	graph.AddEdge(p, q)
	graph.AddEdge(p, s)
	graph.AddEdge(r, p)
	graph.AddEdge(r, m)
	graph.SameRank = []Nodes{{q, r}}
	graph.MaxRank = Nodes{m}

	RankNetworkSimplex(graph)
	if q.Rank != r.Rank {
		t.Errorf("q on rank %d, r on rank %d, want the same", q.Rank, r.Rank)
	}
	for _, node := range graph.Nodes {
		if node != m && node.Rank >= m.Rank {
			t.Errorf("%v on rank %d, m on rank %d, want m last", node, node.Rank, m.Rank)
		}
	}
	for _, edge := range [][2]*Node{{p, q}, {p, s}, {r, m}} {
		if edge[1].Rank <= edge[0].Rank {
			t.Errorf("edge %v->%v ranks %d->%d", edge[0], edge[1], edge[0].Rank, edge[1].Rank)
		}
	}
}

// TestRankSameRankFlatCycle checks that a cycle of edges allowed to be flat,
// made by contracting same-rank groups, keeps every edge's minimum length.
func TestRankSameRankFlatCycle(t *testing.T) {
	graph := NewGraph()
	a, b, c, d, p, q := graph.AddNode(), graph.AddNode(), graph.AddNode(), graph.AddNode(), graph.AddNode(), graph.AddNode()
	graph.AddEdge(a, b)
	graph.SetMinLen(a, b, 0)
	graph.AddEdge(c, d)
	graph.SetMinLen(c, d, 0)
	graph.AddEdge(p, a)
	graph.AddEdge(p, q)
	graph.SetMinLen(p, q, 2)
	graph.AddWeightedEdge(q, c, 10)
	graph.SetMinLen(q, c, 0)
	graph.SameRank = []Nodes{{b, c}, {d, a}}

	RankNetworkSimplex(graph)
	for _, src := range graph.Nodes {
		for _, dst := range src.Out {
			if span := int32(dst.Rank - src.Rank); span < graph.MinLen(src, dst) {
				t.Errorf("edge %v->%v spans %d ranks, want at least %d", src, dst, span, graph.MinLen(src, dst))
			}
		}
	}
	if a.Rank != b.Rank || b.Rank != c.Rank || c.Rank != d.Rank {
		t.Errorf("ranks a=%d b=%d c=%d d=%d, want the same", a.Rank, b.Rank, c.Rank, d.Rank)
	}
}
