package hier

// Crossing counts require up-to-date Node.Pos, see assignPos.

// CrossingsUp counts crossings between edges into u and v from the rank above,
// assuming u is placed left of v.
func (graph *Graph) CrossingsUp(u, v *Node) int { return crossings(u.In, v.In) }

// CrossingsDown counts crossings between edges out of u and v to the rank below,
// assuming u is placed left of v.
func (graph *Graph) CrossingsDown(u, v *Node) int { return crossings(u.Out, v.Out) }

// Crossings counts crossings on both sides assuming u is left of v
func (graph *Graph) Crossings(u, v *Node) int {
	return crossings(u.In, v.In) + crossings(u.Out, v.Out)
}

// crossings counts pairs (w in uadj, z in vadj) with z left of w
func crossings(uadj, vadj Nodes) int {
	count := 0
	for _, w := range uadj {
		for _, z := range vadj {
			if z.Pos < w.Pos {
				count++
			}
		}
	}
	return count
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

// TotalCrossings counts edge crossings between all adjacent ranks
func (graph *Graph) TotalCrossings() int {
	graph.assignPos()
	total := 0
	for _, layer := range graph.ByRank {
		for i, u := range layer {
			for _, v := range layer[i+1:] {
				total += graph.CrossingsUp(u, v)
			}
		}
	}
	return total
}
