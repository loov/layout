package hier

import (
	"math"
	"slices"
)

// Position assigns node centers: rows by rank height, columns by Brandes-Köpf.
func Position(graph *Graph) {
	PositionInitial(graph)
	if len(graph.Nodes) == 0 {
		return
	}

	// four alignments: up/down x left/right
	var xs [4][]float32
	for i := range xs {
		up, left := i < 2, i%2 == 0
		xs[i] = brandesKoepf(graph, up, left)
	}

	// align layouts to the narrowest one, then take the average of the two medians
	width := func(x []float32) float32 { return slices.Max(x) - slices.Min(x) }
	narrow := 0
	for i := range xs {
		if width(xs[i]) < width(xs[narrow]) {
			narrow = i
		}
	}
	for i := range xs {
		var shift float32
		if i%2 == 0 {
			shift = slices.Min(xs[narrow]) - slices.Min(xs[i])
		} else {
			shift = slices.Max(xs[narrow]) - slices.Max(xs[i])
		}
		for k := range xs[i] {
			xs[i][k] += shift
		}
	}

	for _, node := range graph.Nodes {
		v := []float32{xs[0][node.ID], xs[1][node.ID], xs[2][node.ID], xs[3][node.ID]}
		slices.Sort(v)
		node.Center.X = (v[1] + v[2]) / 2
	}
	StraightenChains(graph)
	flushLeft(graph)
	AlignClusterBorders(graph)
}

// StraightenChains moves virtual nodes towards the midpoint of their
// neighbors along the edge, as far as the room between the nodes beside
// them allows, so that long edges run diagonally instead of hooking at
// the end. Order within the ranks is kept, so crossings don't change.
func StraightenChains(graph *Graph) {
	graph.assignPos()
	for range 10 {
		moved := false
		for _, layer := range graph.ByRank {
			for i, node := range layer {
				if !node.Virtual || node.BorderLeft || node.BorderRight || len(node.In) != 1 || len(node.Out) != 1 {
					continue
				}
				want := (node.In[0].Center.X + node.Out[0].Center.X) / 2
				if i > 0 {
					left := layer[i-1]
					want = max(want, left.Center.X+left.Radius.X+node.Radius.X)
				}
				if i+1 < len(layer) {
					right := layer[i+1]
					want = min(want, right.Center.X-right.Radius.X-node.Radius.X)
				}
				if d := want - node.Center.X; d > 0.5 || d < -0.5 {
					node.Center.X = want
					moved = true
				}
			}
		}
		if !moved {
			break
		}
	}
}

// PositionInitial assigns rows and packs nodes left to right
func PositionInitial(graph *Graph) {
	top := float32(0)
	for _, nodes := range graph.ByRank {
		left := float32(0)

		halfrow := float32(0)
		for _, node := range nodes {
			halfrow = max(halfrow, node.Radius.Y)
		}

		top += halfrow
		for _, node := range nodes {
			node.Center.X = left + node.Radius.X
			node.Center.Y = top
			left = node.Center.X + node.Radius.X
		}
		top += halfrow
	}
}

// brandesKoepf computes x coordinates for one of the four alignment directions.
// up: align to neighbors in the rank above (otherwise below);
// left: compact towards the left (otherwise right).
func brandesKoepf(graph *Graph, up, left bool) []float32 {
	n := graph.NodeCount()

	// orient the problem so that we always align "up" and compact "left"
	layers := make([]Nodes, len(graph.ByRank))
	for i, layer := range graph.ByRank {
		layers[i] = slices.Clone(layer)
		if !left {
			slices.Reverse(layers[i])
		}
	}
	if !up {
		slices.Reverse(layers)
	}
	upper := func(node *Node) Nodes {
		if up {
			return node.In
		}
		return node.Out
	}

	pos := make([]int, n)
	layerOf := make([]Nodes, n)
	for _, layer := range layers {
		for i, node := range layer {
			pos[node.ID] = i
			layerOf[node.ID] = layer
		}
	}
	// upper neighbors sorted by position
	nbrs := make([]Nodes, n)
	for _, node := range graph.Nodes {
		nbrs[node.ID] = slices.Clone(upper(node))
		slices.SortFunc(nbrs[node.ID], func(a, b *Node) int { return pos[a.ID] - pos[b.ID] })
	}

	// type 1 conflicts: non-inner segments crossing inner (virtual-virtual) segments
	conflict := map[[2]ID]bool{}
	inner := func(v *Node) *Node {
		if v.Virtual && len(nbrs[v.ID]) == 1 && nbrs[v.ID][0].Virtual {
			return nbrs[v.ID][0]
		}
		return nil
	}
	for i := 1; i < len(layers); i++ {
		layer, above := layers[i], layers[i-1]
		k0, l := 0, 0
		for l1, v := range layer {
			u := inner(v)
			if l1 != len(layer)-1 && u == nil {
				continue
			}
			k1 := len(above) - 1
			if u != nil {
				k1 = pos[u.ID]
			}
			for ; l <= l1; l++ {
				for _, w := range nbrs[layer[l].ID] {
					if k := pos[w.ID]; k < k0 || k > k1 {
						conflict[[2]ID{w.ID, layer[l].ID}] = true
					}
				}
			}
			k0 = k1
		}
	}

	// vertical alignment
	root := make([]*Node, n)
	align := make([]*Node, n)
	for _, node := range graph.Nodes {
		root[node.ID], align[node.ID] = node, node
	}
	for i := 1; i < len(layers); i++ {
		r := -1
		for _, v := range layers[i] {
			d := len(nbrs[v.ID])
			if d == 0 {
				continue
			}
			for m := (d - 1) / 2; m <= d/2; m++ {
				if align[v.ID] != v {
					break
				}
				u := nbrs[v.ID][m]
				if !conflict[[2]ID{u.ID, v.ID}] && r < pos[u.ID] {
					align[u.ID] = v
					root[v.ID] = root[u.ID]
					align[v.ID] = root[v.ID]
					r = pos[u.ID]
				}
			}
		}
	}

	// horizontal compaction: longest path over the block graph
	// (blocks = roots, edges between horizontally adjacent blocks)
	x := make([]float32, n)
	placed := make([]bool, n)
	var place func(v *Node)
	place = func(v *Node) {
		if placed[v.ID] {
			return
		}
		placed[v.ID] = true
		for w := v; ; w = align[w.ID] {
			if p := pos[w.ID]; p > 0 {
				prev := layerOf[w.ID][p-1]
				u := root[prev.ID]
				place(u)
				x[v.ID] = max(x[v.ID], x[u.ID]+prev.Radius.X+w.Radius.X)
			}
			if align[w.ID] == v {
				break
			}
		}
	}
	for _, node := range graph.Nodes {
		place(root[node.ID])
	}
	final := make([]float32, n)
	for _, node := range graph.Nodes {
		final[node.ID] = x[root[node.ID].ID]
		if !left {
			final[node.ID] = -final[node.ID]
		}
	}
	return final
}

// flushLeft moves the graph so that the leftmost node touches x = 0
func flushLeft(graph *Graph) {
	if len(graph.Nodes) == 0 {
		return
	}
	minleft := float32(math.Inf(1))
	for _, node := range graph.Nodes {
		minleft = min(minleft, node.Center.X-node.Radius.X)
	}
	for _, node := range graph.Nodes {
		node.Center.X -= minleft
	}
}
