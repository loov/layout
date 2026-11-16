package hier

import (
	"math/rand"
	"testing"
)

// TestOrderRanksSiftClusters checks on random clustered graphs that sift
// leaves every node inside its own clusters and outside all others, so
// that orderClusters has nothing to undo.
func TestOrderRanksSiftClusters(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	moved := 0
	for i := range 200 {
		graph := GenerateRandomGraph(8+i%15, 0.2, rng)
		perm := rng.Perm(len(graph.Nodes))
		outer := &Cluster{}
		for _, id := range perm[:4] {
			outer.Members.Append(graph.Nodes[id])
		}
		inner := &Cluster{Parent: outer, Members: Nodes{graph.Nodes[perm[0]], graph.Nodes[perm[1]]}}
		sibling := &Cluster{Members: Nodes{graph.Nodes[perm[4]], graph.Nodes[perm[5]]}}

		Decycle(graph)
		Rank(graph)
		AddVirtuals(graph)
		graph.Clusters = []*Cluster{outer, inner, sibling}
		AddClusterBorders(graph)
		OrderRanksDepthFirst(graph)
		orderClusters(graph)
		moved += OrderRanksSift(graph)

		for r, layer := range graph.ByRank {
			var inside []*Cluster
			for _, node := range layer {
				switch {
				case node.BorderLeft:
					inside = append(inside, node.Cluster)
					continue
				case node.BorderRight:
					inside = inside[:len(inside)-1]
					continue
				}
				var around *Cluster
				if len(inside) > 0 {
					around = inside[len(inside)-1]
				}
				if around != node.Cluster {
					t.Fatalf("graph %d rank %d: %v in cluster %p, want %p", i, r, node, around, node.Cluster)
				}
			}
		}
	}
	if moved == 0 {
		t.Error("sift moved nothing")
	}
}
