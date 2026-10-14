package hier

import "slices"

// AddVirtuals creates nodes for edges spanning multiple ranks
//
//	Rank  input    output
//	 0      A        A
//	       /|       / \
//	 1    B |  =>  B   V
//	       \|       \ /
//	 2      C        C
func AddVirtuals(graph *Graph) {
	if len(graph.ByRank) == 0 {
		return
	}

	for _, src := range graph.Nodes {
		for di, dst := range src.Out {
			if dst.Rank-src.Rank <= 1 {
				continue
			}

			src.Out[di] = nil
			dst.In.Remove(src)
			weight := graph.Weight(src, dst)
			// parallel edges share the weight, keep it for the next one
			if !slices.Contains(src.Out[di+1:], dst) {
				delete(graph.weights, [2]ID{src.ID, dst.ID})
				delete(graph.minlens, [2]ID{src.ID, dst.ID})
			}

			for rank := dst.Rank - 1; rank > src.Rank; rank-- {
				node := graph.AddNode()
				node.Rank = rank
				node.Virtual = true
				graph.ByRank[node.Rank].Append(node)
				graph.AddWeightedEdge(node, dst, weight)
				dst = node
			}

			src.Out[di] = dst
			dst.In.Append(src)
			graph.SetWeight(src, dst, weight)
		}
	}
}

// Edge is an edge of the graph as ranking and AddVirtuals leave it: from
// its upper end to its lower, through the virtual nodes of Chain, or
// along a rank when Flat, see TraceEdges
type Edge struct {
	From, To *Node
	Chain    Nodes
	Flat     bool
}

// TraceEdges records the edges between the nodes that aren't virtual, as
// they run after AddVirtuals, for EdgeBetween, EdgesBetween and Edges;
// edges that merged run as one, and of two that join the same nodes the
// way they run, as at different ports, the last counts
func TraceEdges(graph *Graph) {
	graph.edges = map[[2]ID][]*Edge{}
	var traced []*Edge
	add := func(e *Edge) {
		k := [2]ID{e.From.ID, e.To.ID}
		graph.edges[k] = append(graph.edges[k], e)
		traced = append(traced, e)
	}
	for _, src := range graph.Nodes {
		if src.Virtual {
			continue
		}
		for _, out := range src.Out {
			e := &Edge{From: src}
			if e.To = out.ChainEnd(); e.To.Virtual {
				continue // a chain that doesn't end at a node
			}
			e.Chain = slices.Collect(out.Chain())
			add(e)
		}
	}

	for _, flat := range graph.Flat {
		add(&Edge{From: flat[0], To: flat[1], Flat: true})
	}
	graph.traced = traced[:0]
	for _, e := range traced {
		if es := graph.edges[[2]ID{e.From.ID, e.To.ID}]; es[len(es)-1] == e {
			graph.traced = append(graph.traced, e)
		}
	}
}

// Edges returns the edges TraceEdges recorded, in the order of their
// sources and then of Flat
func (graph *Graph) Edges() []*Edge { return graph.traced }

// EdgeBetween returns the edge between a and b, see TraceEdges, and
// whether it runs from b to a; nil when there is none
func (graph *Graph) EdgeBetween(a, b *Node) (edge *Edge, reversed bool) {
	if es := graph.edges[[2]ID{a.ID, b.ID}]; len(es) > 0 {
		return es[len(es)-1], false
	}
	if es := graph.edges[[2]ID{b.ID, a.ID}]; len(es) > 0 {
		return es[len(es)-1], true
	}
	return nil, false
}

// EdgesBetween returns every edge that TraceEdges recorded between a and
// b, either way, the last counting ones included
func (graph *Graph) EdgesBetween(a, b *Node) []*Edge {
	return slices.Concat(graph.edges[[2]ID{a.ID, b.ID}], graph.edges[[2]ID{b.ID, a.ID}])
}
