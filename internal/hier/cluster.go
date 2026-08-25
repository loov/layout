package hier

import (
	"math"
	"sort"
)

// Cluster is a group of nodes drawn inside a common box.
type Cluster struct {
	Members Nodes
	// Left and Right are virtual border nodes, one per rank the cluster
	// spans starting at MinRank; ordering keeps all members between them
	// and positioning aligns them vertically.
	Left, Right      Nodes
	MinRank, MaxRank int
}

// AddClusterBorders creates border nodes for every cluster and assigns
// cluster membership to virtual nodes of edges inside a cluster. Must run
// after AddVirtuals and before ordering.
func AddClusterBorders(graph *Graph) {
	if len(graph.Clusters) == 0 {
		return
	}

	for _, cluster := range graph.Clusters {
		member := NewNodeSet(graph.NodeCount())
		for _, node := range cluster.Members {
			member.Add(node)
			node.Cluster = cluster
		}
		// virtual chains between members belong to the cluster
		for _, src := range cluster.Members {
			for _, out := range src.Out {
				var chain Nodes
				dst := out
				for dst.Virtual {
					chain.Append(dst)
					dst = dst.Out[0]
				}
				if member.Contains(dst) {
					for _, v := range chain {
						v.Cluster = cluster
						cluster.Members.Append(v)
					}
				}
			}
		}

		cluster.MinRank, cluster.MaxRank = math.MaxInt, -1
		for _, node := range cluster.Members {
			cluster.MinRank = min(cluster.MinRank, node.Rank)
			cluster.MaxRank = max(cluster.MaxRank, node.Rank)
		}

		var prevLeft, prevRight *Node
		for rank := cluster.MinRank; rank <= cluster.MaxRank; rank++ {
			left, right := graph.AddNode(), graph.AddNode()
			for _, border := range []*Node{left, right} {
				border.Rank = rank
				border.Virtual = true
				border.Cluster = cluster
				graph.ByRank[rank].Append(border)
			}
			left.BorderLeft, right.BorderRight = true, true
			cluster.Left.Append(left)
			cluster.Right.Append(right)
			// chain borders so positioning keeps them vertical
			if prevLeft != nil {
				graph.AddWeightedEdge(prevLeft, left, borderWeight)
				graph.AddWeightedEdge(prevRight, right, borderWeight)
			}
			prevLeft, prevRight = left, right
		}
	}
}

// borderWeight makes border chains expensive to cross and keeps them straight
const borderWeight = 8

// clusterCoef gives all nodes of a cluster in the layer the mean of their
// Coef values so that the median sweep moves the cluster as a block
func clusterCoef(layer Nodes) {
	sum := map[*Cluster]float32{}
	count := map[*Cluster]int{}
	for _, node := range layer {
		if node.Cluster != nil {
			sum[node.Cluster] += node.Coef
			count[node.Cluster]++
		}
	}
	for _, node := range layer {
		if node.Cluster != nil {
			node.Coef = sum[node.Cluster] / float32(count[node.Cluster])
		}
	}
}

// borderSide orders a cluster's left border first and right border last
func borderSide(node *Node) int {
	switch {
	case node.BorderLeft:
		return -1
	case node.BorderRight:
		return 1
	}
	return 0
}

// orderClusters makes every cluster contiguous in every rank, with its left
// border first and right border last, keeping the relative order otherwise.
func orderClusters(graph *Graph) {
	if len(graph.Clusters) == 0 {
		return
	}
	graph.assignPos()
	for _, layer := range graph.ByRank {
		// cluster key: mean position of its nodes in this layer
		sum := map[*Cluster]float64{}
		count := map[*Cluster]int{}
		for _, node := range layer {
			if node.Cluster != nil {
				sum[node.Cluster] += float64(node.Pos)
				count[node.Cluster]++
			}
		}
		key := func(node *Node) float64 {
			if node.Cluster != nil {
				return sum[node.Cluster] / float64(count[node.Cluster])
			}
			return float64(node.Pos)
		}
		sort.SliceStable(layer, func(i, k int) bool {
			a, b := layer[i], layer[k]
			if ka, kb := key(a), key(b); ka != kb {
				return ka < kb
			}
			return borderSide(a) < borderSide(b)
		})
		layer.assignPos()
	}
}

// AlignClusterBorders shifts nodes outward so that every cluster's left
// borders share one x coordinate and so do its right borders. Must run
// after positioning.
func AlignClusterBorders(graph *Graph) {
	if len(graph.Clusters) == 0 {
		return
	}
	graph.assignPos()
	for range 10 {
		moved := false
		for _, cluster := range graph.Clusters {
			minLeft, maxRight := float32(math.Inf(1)), float32(math.Inf(-1))
			for i := range cluster.Left {
				minLeft = min(minLeft, cluster.Left[i].Center.X)
				maxRight = max(maxRight, cluster.Right[i].Center.X)
			}
			for i := range cluster.Left {
				left, right := cluster.Left[i], cluster.Right[i]
				layer := graph.ByRank[left.Rank]
				if d := left.Center.X - minLeft; d > 0 {
					for _, node := range layer[:left.Pos+1] {
						node.Center.X -= d
					}
					moved = true
				}
				if d := maxRight - right.Center.X; d > 0 {
					for _, node := range layer[right.Pos:] {
						node.Center.X += d
					}
					moved = true
				}
			}
		}
		if !moved {
			break
		}
	}
	flushLeft(graph)
}
