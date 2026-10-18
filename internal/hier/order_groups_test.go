package hier

import (
	"math/rand/v2"
	"slices"
	"testing"
)

// TestOrderRanksGroupsDelta checks that counting only the change of a
// move takes the moves that counting all again does, on random graphs
// with weights, ports and flat edges.
func TestOrderRanksGroupsDelta(t *testing.T) {
	defer func(ratio int) { deltaRatio = ratio }(deltaRatio)
	for seed := range uint64(1000) {
		var orders [2][]Nodes
		for i, ratio := range []int{0, 1 << 30} {
			deltaRatio = ratio
			graph := randomLayers(rand.New(rand.NewPCG(seed, 1)), seed%2 == 0, seed%3 == 0)
			OrderRanksGroups(graph)
			orders[i] = saveOrder(graph)
		}
		if !slices.EqualFunc(orders[0], orders[1], func(a, b Nodes) bool {
			return slices.EqualFunc(a, b, func(x, y *Node) bool { return x.ID == y.ID })
		}) {
			t.Fatalf("seed %d: orders differ\ndelta:   %v\nrecount: %v", seed, orders[0], orders[1])
		}
	}
}
