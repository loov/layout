package hier

import (
	"math/rand/v2"
	"slices"
	"testing"
)

// TestOrderRanksBlocks checks that a column, n with its source and its
// link, moves past the fan of another link that a hub also feeds, which
// moving any one node doesn't do: alone, n or its link crosses the fan,
// and the hub, held right by its own source and a third link, crosses
// the fan wherever it goes.
func TestOrderRanksBlocks(t *testing.T) {
	graph := NewGraph()
	node := func() *Node { return graph.AddNode() }
	s, u1, u2, u3, h0, x1, x2, x3 := node(), node(), node(), node(), node(), node(), node(), node()
	n, t1, t2, t3, hub, y1, y2, y3 := node(), node(), node(), node(), node(), node(), node(), node()
	link2, link1, link3 := node(), node(), node()
	for _, e := range [][2]*Node{
		{s, n},
		{n, link2},
		{u1, t1},
		{u2, t2},
		{u3, t3},
		{t1, link1},
		{t2, link1},
		{t3, link1},
		{h0, hub},
		{hub, link1},
		{hub, link2},
		{hub, link3},
		{x1, y1},
		{x2, y2},
		{x3, y3},
		{y1, link3},
		{y2, link3},
		{y3, link3},
	} {
		graph.AddEdge(e[0], e[1])
	}

	Rank(graph)
	graph.ByRank = []Nodes{{s, u1, u2, u3, h0, x1, x2, x3}, {n, t1, t2, t3, hub, y1, y2, y3}, {link2, link1, link3}}
	before := graph.TotalCrossings()

	start := saveOrder(graph)
	OrderRanksSift(graph)
	OrderRanksTranspose(graph)
	if got := graph.TotalCrossings(); got != before {
		t.Fatalf("sift and transpose crossings = %v, want them stuck at %v", got, before)
	}
	graph.ByRank = start

	OrderRanksBlocks(graph)
	if got := graph.TotalCrossings(); got >= before {
		t.Errorf("crossings after = %v, want fewer than %v", got, before)
	}
	if !slices.Equal(graph.ByRank[2], Nodes{link1, link2, link3}) {
		t.Errorf("order = %v, want link2 right of link1", graph.ByRank)
	}
}

// TestPairCosts checks that pairCosts, which moves c from one slot to the
// next, counts the crossings of b->c as blockEdgeCosts does for each slot
// alone, on random layered graphs with weights.
func TestPairCosts(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	for range 500 {
		graph := randomLayers(rng, true, false)
		m := &blockMover{graph: graph}
		for r := 1; r < len(graph.ByRank); r++ {
			upper, lower := graph.ByRank[r-1], graph.ByRank[r]
			b, c := upper[rng.IntN(len(upper))], lower[rng.IntN(len(lower))]
			var ps, qs []int
			for p := range len(upper) {
				if rng.IntN(2) == 0 {
					ps = append(ps, p)
				}
			}
			for q := range len(lower) {
				if rng.IntN(2) == 0 {
					qs = append(qs, q)
				}
			}
			if len(ps) == 0 || len(qs) == 0 {
				continue
			}
			m.blockEdges(b, c)
			got := map[int][]float32{}
			m.pairCosts(ps, qs, func(q int, pair []float32) { got[q] = slices.Clone(pair) })
			for _, q := range qs {
				full := m.blockEdgeCosts(q)
				for k, p := range ps {
					if got[q][k] != full[p] {
						t.Fatalf("%v->%v at %d, %d: %v, want %v", b, c, p, q, got[q][k], full[p])
					}
				}
			}
		}
	}
}
