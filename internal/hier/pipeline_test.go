package hier

import (
	"math/rand"
	"testing"
)

func checkPipeline(t *testing.T, name string, graph *Graph) {
	t.Helper()
	Decycle(graph)
	if graph.IsCyclic() {
		t.Errorf("%s: still cyclic after decycle", name)
	}
	Rank(graph)
	for _, src := range graph.Nodes {
		for _, dst := range src.Out {
			if dst.Rank <= src.Rank {
				t.Errorf("%s: edge %v->%v ranks %d->%d", name, src, dst, src.Rank, dst.Rank)
			}
		}
	}
	for i, layer := range graph.ByRank {
		if len(layer) == 0 {
			t.Errorf("%s: empty rank %d", name, i)
		}
	}
	AddVirtuals(graph)
	for _, src := range graph.Nodes {
		for _, dst := range src.Out {
			if dst.Rank-src.Rank != 1 {
				t.Errorf("%s: edge %v->%v spans %d", name, src, dst, dst.Rank-src.Rank)
			}
			if !dst.In.Contains(src) {
				t.Errorf("%s: edge %v->%v missing from In", name, src, dst)
			}
		}
	}
	OrderRanksDepthFirst(graph)
	before := graph.TotalCrossings()
	OrderRanks(graph)
	after := graph.TotalCrossings()
	if after > before {
		t.Errorf("%s: ordering made crossings worse %v -> %v", name, before, after)
	}
	for _, node := range graph.Nodes {
		node.Radius = Vector{X: 20, Y: 5}
	}
	Position(graph, true)
	for _, layer := range graph.ByRank {
		for i := 1; i < len(layer); i++ {
			a, b := layer[i-1], layer[i]
			if a.Center.X+a.Radius.X > b.Center.X-b.Radius.X+1e-3 {
				t.Errorf("%s: overlap %v(%.1f) %v(%.1f)", name, a, a.Center.X, b, b.Center.X)
			}
		}
	}
}

func TestPipeline(t *testing.T) {
	for _, tg := range TestGraphs {
		checkPipeline(t, tg.Name, tg.Make())
	}
	rng := rand.New(rand.NewSource(1))
	for i := range 30 {
		checkPipeline(t, "random", GenerateRandomGraph(5+i, 0.2, rng))
	}
}
