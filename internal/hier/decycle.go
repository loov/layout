package hier

import (
	"cmp"
	"slices"
)

// Decycle makes the graph acyclic by reversing the back edges of a depth
// first search. Self-loops are removed and duplicate edges merged; edge
// weights and minimum lengths follow the reversed edges.
//
// It searches twice and keeps the search that reverses less weight, the
// first on a tie: once from nodes with many outgoing and few incoming
// edges, so that the reversed edges tend to be the ones pointing "up" in
// the natural hierarchy, and once like Graphviz, from the nodes in input
// order and through the last edges first, which finds the root of a graph
// whose root has many edges back to it.
func Decycle(graph *Graph) {
	if !graph.IsCyclic() {
		return
	}

	const (
		unseen = iota
		active
		done
	)
	type rankedEdge struct {
		src, dst ID
		minlen   int32
		weight   float32
	}
	search := func(order Nodes, lastFirst bool) (edges []rankedEdge, reversed float32) {
		state := make([]int, graph.NodeCount())
		var visit func(node *Node)
		visit = func(node *Node) {
			state[node.ID] = active
			for i := range node.Out {
				dst := node.Out[i]
				if lastFirst {
					dst = node.Out[len(node.Out)-1-i]
				}
				switch {
				case dst == node:
					// drop self-loop
				case state[dst.ID] == active:
					edges = append(edges, rankedEdge{dst.ID, node.ID, graph.MinLen(node, dst), graph.Weight(node, dst)}) // back edge, reverse
					reversed += graph.Weight(node, dst)
				default:
					edges = append(edges, rankedEdge{node.ID, dst.ID, graph.MinLen(node, dst), graph.Weight(node, dst)})
					if state[dst.ID] == unseen {
						visit(dst)
					}
				}
			}
			state[node.ID] = done
		}
		for _, node := range order {
			if state[node.ID] == unseen {
				visit(node)
			}
		}
		return edges, reversed
	}

	order := slices.Clone(graph.Nodes)
	slices.SortStableFunc(order, func(a, b *Node) int {
		return cmp.Compare(b.OutDegree()-b.InDegree(), a.OutDegree()-a.InDegree())
	})
	edges, reversed := search(order, false)
	if inputEdges, inputReversed := search(graph.Nodes, true); inputReversed < reversed {
		edges = inputEdges
	}

	// rebuild adjacency, weights and minimum lengths from the edge list
	graph.weights, graph.minlens = nil, nil
	for _, node := range graph.Nodes {
		node.In.Clear()
		node.Out.Clear()
	}
	for _, edge := range edges {
		src, dst := graph.Nodes[edge.src], graph.Nodes[edge.dst]
		if slices.Contains(src.Out, dst) {
			// edges that end up the same way merge, with their weights
			graph.SetMinLen(src, dst, max(edge.minlen, graph.MinLen(src, dst)))
			graph.SetWeight(src, dst, graph.Weight(src, dst)+edge.weight)
			continue
		}
		graph.AddEdge(src, dst)
		graph.SetMinLen(src, dst, edge.minlen)
		graph.SetWeight(src, dst, edge.weight)
	}
	for _, node := range graph.Nodes {
		node.In.Normalize()
		node.Out.Normalize()
	}
}
