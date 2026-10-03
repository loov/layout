package hier

import (
	"testing"
	"testing/quick"
)

func TestDecycle(t *testing.T) {
	for _, testgraph := range TestGraphs {
		t.Run(testgraph.Name, func(t *testing.T) {
			graph := testgraph.Make()
			beforeCount := graph.CountUndirectedLinks()

			Decycle(graph)

			if err := graph.CheckErrors(); err != nil {
				t.Errorf("got errors: %v\n%v", err, graph.EdgeMatrixString())
			}
			if graph.IsCyclic() {
				t.Errorf("got cycles\n%v", graph.EdgeMatrixString())
			}
			if afterCount := graph.CountUndirectedLinks(); beforeCount != afterCount {
				t.Errorf("too many edges removed %v -> %v", beforeCount, afterCount)
			}
		})
	}
}

func TestDecycleRandom(t *testing.T) {
	err := quick.Check(func(graph *Graph) bool {
		Decycle(graph)
		if err := graph.CheckErrors(); err != nil {
			t.Errorf("invalid %v:\n%v", err, graph.EdgeMatrixString())
			return false
		}
		if graph.IsCyclic() {
			t.Errorf("cyclic:\n%v", graph.EdgeMatrixString())
			return false
		}
		return true
	}, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestDecycleMinimal(t *testing.T) {
	// A->B, A->C, B->D, C->D, D->A: only D->A needs reversing
	graph := NewGraphFromEdgeList([][]int{0: {1, 2}, 1: {3}, 2: {3}, 3: {0}})
	Decycle(graph)
	if got := graph.ConvertToEdgeList(); len(got[3]) != 0 || len(got[0]) != 3 {
		t.Errorf("expected only D->A reversed, got %v", got)
	}
}

func BenchmarkDecycle(b *testing.B) {
	for _, size := range BenchmarkGraphSizes {
		b.Run(size.Name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				graph := GenerateRegularGraph(size.Nodes, size.Connections)
				Decycle(graph)
			}
		})
	}
}

func TestDecyclePreservesAntiparallelMinimumLengths(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		forward, backward, want int32
	}{
		{"longer reverse", 2, 5, 5}, {"longer forward", 5, 2, 5},
		{"default forward", 1, 0, 1}, {"default reverse", 0, 1, 1}, {"zero", 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := NewGraph()
			a, b := graph.AddNode(), graph.AddNode()
			graph.AddEdge(a, b)
			graph.AddEdge(b, a)
			graph.SetMinLen(a, b, tc.forward)
			graph.SetMinLen(b, a, tc.backward)
			Decycle(graph)
			for _, src := range graph.Nodes {
				for _, dst := range src.Out {
					if got := graph.MinLen(src, dst); got != tc.want {
						t.Fatalf("minimum length = %d, want %d", got, tc.want)
					}
				}
			}
			Rank(graph)
			if span := max(a.Rank, b.Rank) - min(a.Rank, b.Rank); span < int(tc.want) {
				t.Fatalf("rank span = %d, want >= %d", span, tc.want)
			}
		})
	}
}
