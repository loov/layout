package hier

import (
	"cmp"
	"slices"
)

// Decycle makes the graph acyclic by reversing the back edges of a depth
// first search. Self-loops are removed and duplicate edges merged; edge
// weights and minimum lengths follow the reversed edges.
//
// The search starts from nodes with many outgoing and few incoming edges,
// so that the reversed edges tend to be the ones pointing "up" in the
// natural hierarchy.
func Decycle(graph *Graph) {
	if !graph.IsCyclic() {
		return
	}

	order := slices.Clone(graph.Nodes)
	slices.SortStableFunc(order, func(a, b *Node) int {
		return cmp.Compare(b.OutDegree()-b.InDegree(), a.OutDegree()-a.InDegree())
	})

	const (
		unseen = iota
		active
		done
	)
	state := make([]int, graph.NodeCount())
	type rankedEdge struct {
		src, dst ID
		minlen   int32
		weight   float32
	}
	var edges []rankedEdge

	var visit func(node *Node)
	visit = func(node *Node) {
		state[node.ID] = active
		for _, dst := range node.Out {
			switch {
			case dst == node:
				// drop self-loop
			case state[dst.ID] == active:
				edges = append(edges, rankedEdge{dst.ID, node.ID, graph.MinLen(node, dst), graph.Weight(node, dst)}) // back edge, reverse
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
