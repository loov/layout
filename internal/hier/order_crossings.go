package hier

// Crossing counts require up-to-date Node.Pos, see assignPos.
// Each crossing counts the product of the two edge weights.

// CrossingsUp counts crossings between edges into u and v from the rank above,
// assuming u is placed left of v.
func (graph *Graph) CrossingsUp(u, v *Node) float32 {
	total := float32(0)
	for _, w := range u.In {
		for _, z := range v.In {
			if z.Pos < w.Pos {
				total += graph.Weight(w, u) * graph.Weight(z, v)
			}
		}
	}
	return total
}

// CrossingsDown counts crossings between edges out of u and v to the rank below,
// assuming u is placed left of v.
func (graph *Graph) CrossingsDown(u, v *Node) float32 {
	total := float32(0)
	for _, w := range u.Out {
		for _, z := range v.Out {
			if z.Pos < w.Pos {
				total += graph.Weight(u, w) * graph.Weight(v, z)
			}
		}
	}
	return total
}

// Crossings counts crossings on both sides assuming u is left of v
func (graph *Graph) Crossings(u, v *Node) float32 {
	return graph.CrossingsUp(u, v) + graph.CrossingsDown(u, v)
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
	for _, layer := range graph.ByRank {
		for i, u := range layer {
			for _, v := range layer[i+1:] {
				total += graph.CrossingsUp(u, v)
			}
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
