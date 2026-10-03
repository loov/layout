package hier

import (
	"math"
	"math/rand"
	"slices"
	"testing"
)

// TestAlignClusterBorders checks on random clustered graphs that every
// cluster's borders line up, that nodes in a rank don't overlap and that
// clusters are no wider than their outermost borders before aligning, or
// their least width, plus the room their nested clusters take.
func TestAlignClusterBorders(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := range 200 {
		graph := GenerateRandomGraph(6+i%15, 0.2, rng)
		perm := rng.Perm(len(graph.Nodes))
		// two sibling clusters, the second one sometimes with a nested one
		var clusters []*Cluster
		for k, members := range [][]int{perm[:3], perm[3:6]} {
			cluster := &Cluster{}
			if k == 0 && rng.Intn(3) == 0 {
				cluster.MinWidth = 300
			}
			for _, id := range members {
				cluster.Members.Append(graph.Nodes[id])
			}
			clusters = append(clusters, cluster)
		}
		if rng.Intn(2) == 0 {
			clusters = append(clusters, &Cluster{Parent: clusters[1], Members: Nodes{graph.Nodes[perm[3]], graph.Nodes[perm[4]]}})
		}

		Decycle(graph)
		Rank(graph)
		AddVirtuals(graph)
		graph.Clusters = clusters
		AddClusterBorders(graph)
		OrderRanks(graph)
		for _, node := range graph.Nodes {
			node.Radius = Vector{X: 20, Y: 5}
			if node.BorderLeft || node.BorderRight {
				node.Radius.X = 4
			}
		}
		// Position, recording how wide each cluster is before aligning
		PositionInitial(graph)
		positionBalanced(graph)
		StraightenChains(graph)
		flushLeft(graph)
		need := make([]float32, len(graph.Clusters))
		for c, cluster := range graph.Clusters {
			lo, hi := float32(math.Inf(1)), float32(math.Inf(-1))
			for k := range cluster.Left {
				lo = min(lo, cluster.Left[k].Center.X)
				hi = max(hi, cluster.Right[k].Center.X)
			}
			need[c] = max(hi-lo, cluster.MinWidth-cluster.Left[0].Radius.X-cluster.Right[0].Radius.X)
		}
		// a nested cluster lining up can take room from its parent's
		// other nodes in a rank, which then move aside
		for c, cluster := range graph.Clusters {
			if cluster.Parent != nil {
				need[slices.Index(graph.Clusters, cluster.Parent)] += need[c]
			}
		}
		AlignClusterBorders(graph)

		for _, layer := range graph.ByRank {
			for k := 1; k < len(layer); k++ {
				a, b := layer[k-1], layer[k]
				if a.Center.X+a.Radius.X > b.Center.X-b.Radius.X+1e-3 {
					t.Errorf("graph %d: overlap %v(%.1f) %v(%.1f)", i, a, a.Center.X, b, b.Center.X)
				}
			}
		}
		for c, cluster := range graph.Clusters {
			left, right := cluster.Left[0].Center.X, cluster.Right[0].Center.X
			for k := range cluster.Left {
				if l, r := cluster.Left[k].Center.X, cluster.Right[k].Center.X; abs(l-left) > 1e-3 || abs(r-right) > 1e-3 {
					t.Errorf("graph %d cluster %d: rank %d borders %.1f-%.1f, want %.1f-%.1f", i, c, cluster.MinRank+k, l, r, left, right)
				}
			}
			if width := right - left; width > need[c]+1e-3 {
				t.Errorf("graph %d cluster %d: %.1f wide, want at most %.1f", i, c, width, need[c])
			}
			if width := right + cluster.Right[0].Radius.X - left + cluster.Left[0].Radius.X; width < cluster.MinWidth-1e-3 {
				t.Errorf("graph %d cluster %d: box %.1f wide, want at least %.1f", i, c, width, cluster.MinWidth)
			}
		}
	}
}

func abs(v float32) float32 { return max(v, -v) }
