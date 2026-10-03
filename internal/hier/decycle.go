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
				edges = append(edges, rankedEdge{dst.ID, node.ID, graph.MinLen(node, dst)}) // back edge, reverse
			default:
				edges = append(edges, rankedEdge{node.ID, dst.ID, graph.MinLen(node, dst)})
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
	weights := graph.weights
	graph.weights, graph.minlens = nil, nil
	for _, node := range graph.Nodes {
		node.In.Clear()
		node.Out.Clear()
	}
	for _, edge := range edges {
		src, dst := graph.Nodes[edge.src], graph.Nodes[edge.dst]
		minlen := edge.minlen
		if slices.Contains(src.Out, dst) {
			minlen = max(minlen, graph.MinLen(src, dst))
		}
		graph.AddEdge(src, dst)
		graph.SetMinLen(src, dst, minlen)
		if w, ok := weights[[2]ID{src.ID, dst.ID}]; ok {
			graph.SetWeight(src, dst, w)
		} else if w, ok := weights[[2]ID{dst.ID, src.ID}]; ok {
			graph.SetWeight(src, dst, w)
		}
	}
	for _, node := range graph.Nodes {
		node.In.Normalize()
		node.Out.Normalize()
	}
}
