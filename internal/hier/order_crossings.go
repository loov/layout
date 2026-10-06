package hier

import (
	"cmp"
	"slices"
)

// Crossing counts require up-to-date Node.Pos, see assignPos.
// Each crossing counts the product of the two edge weights.

// crossingsBothWays returns the crossings of u and v with u left of v, and
// with v left of u, visiting each neighbor pair once
func (graph *Graph) crossingsBothWays(u, v *Node) (uv, vu float32) {
	if len(graph.weights) == 0 && len(graph.Ports) == 0 {
		// unweighted fast path
		count := func(uadj, vadj Nodes) {
			for _, w := range uadj {
				for _, z := range vadj {
					if z.Pos < w.Pos {
						uv++
					} else if z.Pos > w.Pos {
						vu++
					}
				}
			}
		}
		count(u.In, v.In)
		count(u.Out, v.Out)
		return uv, vu
	}
	// where the edges end on the neighbors, see Ports
	in := func(w, u *Node) float32 { return float32(w.Pos) + graph.Ports[[2]ID{w.ID, u.ID}][0] }
	out := func(u, w *Node) float32 { return float32(w.Pos) + graph.Ports[[2]ID{u.ID, w.ID}][1] }
	for _, w := range u.In {
		for _, z := range v.In {
			if zx, wx := in(z, v), in(w, u); zx < wx {
				uv += graph.Weight(w, u) * graph.Weight(z, v)
			} else if zx > wx {
				vu += graph.Weight(w, u) * graph.Weight(z, v)
			}
		}
	}
	for _, w := range u.Out {
		for _, z := range v.Out {
			if zx, wx := out(v, z), out(u, w); zx < wx {
				uv += graph.Weight(u, w) * graph.Weight(v, z)
			} else if zx > wx {
				vu += graph.Weight(u, w) * graph.Weight(v, z)
			}
		}
	}
	return uv, vu
}

// assignPos records each node's index within its rank
func (graph *Graph) assignPos() {
	for _, layer := range graph.ByRank {
		layer.assignPos()
	}
}

// assignPos records each node's index within the layer
func (layer Nodes) assignPos() {
	for i, node := range layer {
		node.Pos = i
	}
}

// TotalCrossings counts weighted edge crossings between all adjacent ranks
func (graph *Graph) TotalCrossings() float32 {
	graph.assignPos()
	total := float32(0)
	for r := 1; r < len(graph.ByRank); r++ {
		total += graph.bilayerCrossings(graph.ByRank[r-1], graph.ByRank[r])
	}
	return total
}

// bilayerCrossings counts weighted crossings between two adjacent layers in
// O(E log V) with an accumulator tree over the lower layer positions.
func (graph *Graph) bilayerCrossings(upper, lower Nodes) float32 {
	if len(lower) == 0 {
		return 0
	}
	if len(graph.Ports) > 0 {
		return graph.portCrossings(upper)
	}
	// edges sorted by (upper pos, lower pos)
	type edge struct {
		pos    int
		weight float32
	}
	var edges []edge
	for _, u := range upper {
		start := len(edges)
		for _, v := range u.Out {
			edges = append(edges, edge{v.Pos, graph.Weight(u, v)})
		}
		slices.SortFunc(edges[start:], func(a, b edge) int { return a.pos - b.pos })
	}

	// tree leaves are lower positions; inner nodes hold summed weights
	first := 1
	for first < len(lower) {
		first *= 2
	}
	tree := make([]float32, 2*first)
	total := float32(0)
	for _, e := range edges {
		i := first + e.pos
		tree[i] += e.weight
		for i > 1 {
			if i%2 == 0 {
				total += e.weight * tree[i+1] // edges already seen to the right
			}
			i /= 2
			tree[i] += e.weight
		}
	}
	return total
}

// TotalEdgeLength sums the weighted horizontal distance, in positions,
// between the ends of every edge; a secondary ordering objective.
func (graph *Graph) TotalEdgeLength() float32 {
	graph.assignPos()
	total := float32(0)
	for _, src := range graph.Nodes {
		for _, dst := range src.Out {
			d := float32(src.Pos - dst.Pos)
			if d < 0 {
				d = -d
			}
			total += graph.Weight(src, dst) * d
		}
	}
	return total
}

// edgeLength sums the horizontal distance of node's edges given it sits at pos
func (graph *Graph) edgeLength(node *Node, pos int) float32 {
	total := float32(0)
	for _, src := range node.In {
		d := float32(src.Pos - pos)
		if d < 0 {
			d = -d
		}
		total += graph.Weight(src, node) * d
	}
	for _, dst := range node.Out {
		d := float32(dst.Pos - pos)
		if d < 0 {
			d = -d
		}
		total += graph.Weight(node, dst) * d
	}
	return total
}

// portCrossings counts weighted crossings between the layer upper and the
// one below it as bilayerCrossings does, with the edges ending where
// Ports puts them on their nodes: in the order they leave upper, the
// crossings are the pairs that reach the lower layer the other way round
func (graph *Graph) portCrossings(upper Nodes) float32 {
	type edge struct {
		from, to float32
		weight   float32
	}
	var edges []edge
	for _, u := range upper {
		for _, v := range u.Out {
			p := graph.Ports[[2]ID{u.ID, v.ID}]
			edges = append(edges, edge{float32(u.Pos) + p[0], float32(v.Pos) + p[1], graph.Weight(u, v)})
		}
	}
	slices.SortFunc(edges, func(a, b edge) int {
		if a.from != b.from {
			return cmp.Compare(a.from, b.from)
		}
		return cmp.Compare(a.to, b.to)
	})
	// the leaves of the tree are the places edges reach, in order
	tos := make([]float32, len(edges))
	for i, e := range edges {
		tos[i] = e.to
	}
	slices.Sort(tos)
	tos = slices.Compact(tos)
	first := 1
	for first < len(tos) {
		first *= 2
	}
	tree := make([]float32, 2*first)
	total := float32(0)
	for _, e := range edges {
		at, _ := slices.BinarySearch(tos, e.to)
		i := first + at
		tree[i] += e.weight
		for i > 1 {
			if i%2 == 0 {
				total += e.weight * tree[i+1] // edges already seen to the right
			}
			i /= 2
			tree[i] += e.weight
		}
	}
	return total
}
