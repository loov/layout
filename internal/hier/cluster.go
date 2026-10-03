package hier

import (
	"math"
	"slices"
	"sort"
)

// Cluster is a group of nodes drawn inside a common box.
type Cluster struct {
	Members Nodes
	// Parent is the enclosing cluster, if any
	Parent *Cluster
	// Left and Right are virtual border nodes, one per rank the cluster
	// spans starting at MinRank; ordering keeps all members between them
	// and positioning aligns them vertically.
	Left, Right      Nodes
	MinRank, MaxRank int
	// MinWidth is the least width of the box around the borders, such as
	// for a label
	MinWidth float32
}

// AddClusterBorders creates border nodes for every cluster and assigns
// cluster membership to virtual nodes of edges inside a cluster. Must run
// after AddVirtuals and before ordering.
func AddClusterBorders(graph *Graph) {
	if len(graph.Clusters) == 0 {
		return
	}

	// nodes belong to their innermost cluster
	for _, cluster := range graph.Clusters {
		for _, node := range cluster.Members {
			if node.Cluster == nil || node.Cluster.depth() < cluster.depth() {
				node.Cluster = cluster
			}
		}
	}
	// rank spans from the real members
	rankSpan := func(cluster *Cluster) {
		cluster.MinRank, cluster.MaxRank = math.MaxInt, -1
		for _, node := range cluster.Members {
			cluster.MinRank = min(cluster.MinRank, node.Rank)
			cluster.MaxRank = max(cluster.MaxRank, node.Rank)
		}
	}
	for _, cluster := range graph.Clusters {
		rankSpan(cluster)
	}
	// a virtual node belongs to the innermost cluster of either end whose
	// ranks it is on: a chain is inside the cluster it leaves until the
	// cluster's last rank, and inside the one it enters from its first
	for _, src := range graph.Nodes {
		if src.Virtual {
			continue
		}
		for _, out := range src.Out {
			var chain Nodes
			dst := out
			for dst.Virtual {
				chain.Append(dst)
				dst = dst.Out[0]
			}
			for _, v := range chain {
				var owner *Cluster
				for c := src.Cluster; c != nil; c = c.Parent {
					if v.Rank <= c.MaxRank && (owner == nil || c.depth() > owner.depth()) {
						owner = c
					}
				}
				for c := dst.Cluster; c != nil; c = c.Parent {
					if v.Rank >= c.MinRank && (owner == nil || c.depth() > owner.depth()) {
						owner = c
					}
				}
				if owner != nil {
					v.Cluster = owner
					owner.Members.Append(v)
				}
			}
		}
	}

	for _, cluster := range graph.Clusters {
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

func (cluster *Cluster) depth() int {
	d := 0
	for c := cluster; c != nil; c = c.Parent {
		d++
	}
	return d
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
	clusterIndex := make(map[*Cluster]int, len(graph.Clusters))
	for i, c := range graph.Clusters {
		clusterIndex[c] = i
	}
	// bary is the mean position of a node's neighbors in the ranks above
	// and below, which decides on which side of a cluster a node at the
	// cluster's mean position goes
	bary := func(node *Node) float64 {
		sum, n := 0.0, 0
		for _, adj := range [2]Nodes{node.In, node.Out} {
			for _, other := range adj {
				sum += float64(other.Pos)
				n++
			}
		}
		if n == 0 {
			return float64(node.Pos)
		}
		return sum / float64(n)
	}
	for _, layer := range graph.ByRank {
		// cluster key: mean position and neighbor position of its nodes in
		// this layer, for the cluster and each enclosing cluster; borders
		// only count for the position
		sum, near := map[*Cluster]float64{}, map[*Cluster]float64{}
		count, nearCount := map[*Cluster]int{}, map[*Cluster]int{}
		for _, node := range layer {
			for c := node.Cluster; c != nil; c = c.Parent {
				sum[c] += float64(node.Pos)
				count[c]++
				if borderSide(node) == 0 {
					near[c] += bary(node)
					nearCount[c]++
				}
			}
		}
		// node key: means from the outermost cluster inward, each followed
		// by the cluster's index so that siblings with equal means don't
		// mix, then the node's own position; borders sort to the ends of
		// their cluster
		keys := make([][]float64, len(layer))
		for i, node := range layer {
			var key []float64
			for c := node.Cluster; c != nil; c = c.Parent {
				key = append(key, float64(clusterIndex[c]), near[c]/float64(max(nearCount[c], 1)), sum[c]/float64(count[c]))
			}
			slices.Reverse(key)
			last := float64(node.Pos)
			if side := borderSide(node); side != 0 {
				last = math.Inf(side)
			}
			keys[i] = append(key, last, bary(node))
		}
		index := make(map[*Node]int, len(layer))
		for i, node := range layer {
			index[node] = i
		}
		// a node outside a cluster whose position ties with the cluster's
		// mean goes to the side of its neighbors, and with those tied too
		// before all of the cluster, never among its nodes; equal keys
		// keep the current order
		sort.SliceStable(layer, func(i, k int) bool {
			a, b := index[layer[i]], index[layer[k]]
			if c := slices.Compare(keys[a], keys[b]); c != 0 {
				return c < 0
			}
			if sa, sb := borderSide(layer[i]), borderSide(layer[k]); sa != sb {
				return sa < sb
			}
			return a < b
		})
		layer.assignPos()
	}
}

// AlignClusterBorders moves every cluster's left borders to the leftmost
// of them and its right borders to the rightmost, widened on both sides to
// MinWidth, so that each side shares one x coordinate. Nodes in the way are
// pushed aside: one pass pushes right and one pushes left, and nodes end
// up at the mean of both, which keeps the borders aligned and the nodes
// apart as both passes do. Must run after positioning.
func AlignClusterBorders(graph *Graph) {
	if len(graph.Clusters) == 0 {
		return
	}
	type gap struct {
		to  ID
		min float32
	}
	n := graph.NodeCount()
	// class merges the borders on one side of a cluster into one position,
	// which wants to be at target and stays gap.min left of gap.to
	class := make([]ID, n)
	target := make([]float32, n)
	// clusters whose order contradicts another's across ranks can't line
	// up, they stay as they are
	stuck := map[*Cluster]bool{}
	for {
		for _, node := range graph.Nodes {
			class[node.ID], target[node.ID] = node.ID, node.Center.X
		}
		gaps := make([][]gap, n)
		for _, cluster := range graph.Clusters {
			if stuck[cluster] || len(cluster.Left) == 0 {
				continue
			}
			left, right := cluster.Left[0], cluster.Right[0]
			minLeft, maxRight := float32(math.Inf(1)), float32(math.Inf(-1))
			for i := range cluster.Left {
				class[cluster.Left[i].ID], class[cluster.Right[i].ID] = left.ID, right.ID
				minLeft = min(minLeft, cluster.Left[i].Center.X)
				maxRight = max(maxRight, cluster.Right[i].Center.X)
			}
			// widen to the least width by moving both borders out, so that
			// the members stay centered
			grow := max(0, cluster.MinWidth-(maxRight+right.Radius.X-minLeft+left.Radius.X))
			target[left.ID], target[right.ID] = minLeft-grow/2, maxRight+grow/2
			if cluster.MinWidth > 0 {
				gaps[left.ID] = append(gaps[left.ID], gap{right.ID, cluster.MinWidth - left.Radius.X - right.Radius.X})
			}
		}
		for _, layer := range graph.ByRank {
			for i := 1; i < len(layer); i++ {
				a, b := layer[i-1], layer[i]
				gaps[class[a.ID]] = append(gaps[class[a.ID]], gap{class[b.ID], a.Radius.X + b.Radius.X})
			}
		}

		// topological order of the classes
		indeg := make([]int, n)
		for _, out := range gaps {
			for _, g := range out {
				indeg[g.to]++
			}
		}
		var order []ID
		for _, node := range graph.Nodes {
			if class[node.ID] == node.ID && indeg[node.ID] == 0 {
				order = append(order, node.ID)
			}
		}
		for i := 0; i < len(order); i++ {
			for _, g := range gaps[order[i]] {
				if indeg[g.to]--; indeg[g.to] == 0 {
					order = append(order, g.to)
				}
			}
		}
		cyclic := false
		for _, cluster := range graph.Clusters {
			if !stuck[cluster] && len(cluster.Left) > 0 && (indeg[cluster.Left[0].ID] > 0 || indeg[cluster.Right[0].ID] > 0) {
				stuck[cluster], cyclic = true, true
			}
		}
		if cyclic {
			continue
		}

		right, left := slices.Clone(target), slices.Clone(target)
		for _, v := range order {
			for _, g := range gaps[v] {
				right[g.to] = max(right[g.to], right[v]+g.min)
			}
		}
		for _, v := range slices.Backward(order) {
			for _, g := range gaps[v] {
				left[v] = min(left[v], left[g.to]-g.min)
			}
		}
		for _, node := range graph.Nodes {
			c := class[node.ID]
			node.Center.X = (left[c] + right[c]) / 2
		}
		break
	}
	flushLeft(graph)
}
