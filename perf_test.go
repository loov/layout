package layout_test

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/loov/layout"
)

// TestPerf lays out a random graph of PERF_NODES nodes and prints timings.
func TestPerf(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("PERF_NODES"))
	if n == 0 {
		t.Skip("set PERF_NODES")
	}
	graph := randomGraph(n, 1)
	start := time.Now()
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	fmt.Println("nodes", n, "edges", len(graph.Edges), "took", time.Since(start))
}
