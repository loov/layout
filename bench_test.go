package layout_test

import (
	"fmt"
	"maps"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/internal/examples"
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

// BenchmarkText lays out all the fixtures, and random graphs, for text.
func BenchmarkText(b *testing.B) {
	var graphs []func() *layout.Graph
	var options []layout.Options
	for _, name := range slices.Sorted(maps.Keys(examples.Graphs)) {
		graphs = append(graphs, examples.Graphs[name])
		options = append(options, examples.Options[name])
	}
	files, err := filepath.Glob(filepath.Join("testdata", "graphviz", "*.gv"))
	if err != nil {
		b.Fatal(err)
	}
	for _, file := range files {
		parsed, err := dot.ParseFile(file)
		if err != nil {
			b.Fatal(err)
		}
		graphs = append(graphs, func() *layout.Graph { return parsed[0] })
		options = append(options, layout.Options{})
	}

	b.Run("fixtures", func(b *testing.B) {
		for b.Loop() {
			for i, build := range graphs {
				opts := options[i]
				opts.ForText = true
				if _, err := layout.Hierarchical(build(), opts); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	for _, n := range []int{100, 500} {
		b.Run(fmt.Sprint("nodes=", n), func(b *testing.B) {
			graph := randomGraph(n, 1)
			for b.Loop() {
				if _, err := layout.Hierarchical(graph, layout.Options{ForText: true}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
