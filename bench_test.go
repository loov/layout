package layout_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/loov/layout"
)

// randomGraph builds a sparse random digraph with n nodes and about 2n edges
func randomGraph(n int, seed int64) *layout.Graph {
	rng := rand.New(rand.NewSource(seed))
	graph := layout.NewDigraph()
	for i := range n {
		graph.Node(fmt.Sprint("n", i))
	}
	for range 2 * n {
		a, b := rng.Intn(n), rng.Intn(n)
		if a != b {
			graph.Edge(fmt.Sprint("n", a), fmt.Sprint("n", b))
		}
	}
	return graph
}

func BenchmarkHierarchical(b *testing.B) {
	for _, n := range []int{20, 100, 500} {
		b.Run(fmt.Sprint("nodes=", n), func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				graph := randomGraph(n, 1)
				b.StartTimer()
				if err := layout.Hierarchical(graph); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
