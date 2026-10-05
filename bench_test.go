package layout_test

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/loov/layout"
)

// randomGraph builds a sparse random digraph with n nodes in about 30
// levels and 2n edges going down one to three levels, like a dependency
// graph. A uniformly random DAG would have O(n) ranks and O(n²) virtual
// nodes, which no layered layout handles well.
func randomGraph(n int, seed int64) *layout.Graph {
	rng := rand.New(rand.NewSource(seed))
	graph := layout.NewDigraph()
	const levels = 30
	perLevel := max(1, n/levels)
	for i := range n {
		graph.Node(fmt.Sprint("n", i))
	}
	for range 2 * n {
		a := rng.Intn(n)
		level := a / perLevel
		b := (level+1+rng.Intn(3))*perLevel + rng.Intn(perLevel)
		if b < n {
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
				if _, err := layout.Hierarchical(graph, layout.Options{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestPerf lays out a random graph of PERF_NODES nodes and prints timings.
func TestPerf(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("PERF_NODES"))
	if n == 0 {
		t.Skip("set PERF_NODES")
	}
	graph := randomGraph(n, 1)
	start := time.Now()
	if _, err := layout.Hierarchical(graph, layout.Options{}); err != nil {
		t.Fatal(err)
	}
	fmt.Println("nodes", n, "edges", len(graph.Edges), "took", time.Since(start))
}
