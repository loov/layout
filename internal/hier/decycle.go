package hier

import (
	"cmp"
	"slices"
)

// Decycle makes the graph acyclic by putting the nodes in a sequence and
// reversing the edges that point back in it. Self-loops are removed and
// duplicate edges merged; edge weights and minimum lengths follow the
// reversed edges.
//
// It starts from the reverse postorders of two depth first searches: from
// nodes with many outgoing and few incoming edges, so that the reversed
// edges tend to be the ones pointing "up" in the natural hierarchy, and
// like Graphviz, from the nodes in input order and through the last edges
// first, which finds the root of a graph whose root has many edges back to
// it. Each is improved by moving single nodes to where the fewest of their
// edges point back, and the one that reverses the least weight is kept,
// the first on a tie.
func Decycle(graph *Graph) {
	if !graph.IsCyclic() {
		return
	}

	n := graph.NodeCount()

	// dfs returns the reverse postorder of a depth first search, in which
	// only the back edges of the search point back
	dfs := func(order Nodes, lastFirst bool) Nodes {
		seen := make([]bool, n)
		seq := make(Nodes, n)
		next := n - 1
		var visit func(node *Node)
		visit = func(node *Node) {
			seen[node.ID] = true
			for i := range node.Out {
				dst := node.Out[i]
				if lastFirst {
					dst = node.Out[len(node.Out)-1-i]
				}
				if !seen[dst.ID] {
					visit(dst)
				}
			}
			seq[next] = node
			next--
		}
		for _, node := range order {
			if !seen[node.ID] {
				visit(node)
			}
		}
		return seq
	}

	// sift moves each node to the place in seq where the least weight of
	// its edges points back, until no move helps, and returns the weight
	// that points back
	pos := make([]int, n)
	type near struct {
		at    int     // position of a neighbor
		delta float32 // w(node->u) - w(u->node)
	}
	var nears []near
	sift := func(seq Nodes) (_ Nodes, reversed float32) {
		for i, node := range seq {
			pos[node.ID] = i
		}
		// every move saves weight, the cap only guards against rounding
		for pass, improved := 0, true; improved && pass < 100; pass++ {
			improved = false
			for _, node := range slices.Clone(seq) {
				at := pos[node.ID]
				nears = nears[:0]
				total := float32(0)
				for _, dst := range node.Out {
					if dst != node {
						nears = append(nears, near{pos[dst.ID], graph.Weight(node, dst)})
						total += graph.Weight(node, dst)
					}
				}
				for _, src := range node.In {
					if src != node {
						nears = append(nears, near{pos[src.ID], -graph.Weight(src, node)})
						total += graph.Weight(src, node)
					}
				}
				slices.SortFunc(nears, func(a, b near) int { return cmp.Compare(a.at, b.at) })
				// moving node past u changes the weight pointing back by
				// its delta, so the best places are next to neighbors; a
				// move has to save more than rounding errors
				best, bestAt := -max(1e-3, 1e-5*total), at
				split, _ := slices.BinarySearchFunc(nears, at, func(a near, at int) int { return cmp.Compare(a.at, at) })
				var cost float32
				for i := split - 1; i >= 0; i-- {
					cost -= nears[i].delta
					if (i == 0 || nears[i-1].at != nears[i].at) && cost < best {
						best, bestAt = cost, nears[i].at
					}
				}
				cost = 0
				for i := split; i < len(nears); i++ {
					cost += nears[i].delta
					if (i == len(nears)-1 || nears[i+1].at != nears[i].at) && cost < best {
						best, bestAt = cost, nears[i].at
					}
				}
				if bestAt == at {
					continue
				}
				improved = true
				seq = slices.Insert(slices.Delete(seq, at, at+1), bestAt, node)
				for i := min(at, bestAt); i <= max(at, bestAt); i++ {
					pos[seq[i].ID] = i
				}
			}
		}
		for _, node := range graph.Nodes {
			for _, dst := range node.Out {
				if pos[dst.ID] < pos[node.ID] {
					reversed += graph.Weight(node, dst)
				}
			}
		}
		return seq, reversed
	}

	order := slices.Clone(graph.Nodes)
	slices.SortStableFunc(order, func(a, b *Node) int {
		return cmp.Compare(b.OutDegree()-b.InDegree(), a.OutDegree()-a.InDegree())
	})
	var best Nodes
	var bestReversed float32
	for _, seq := range []Nodes{dfs(order, false), dfs(graph.Nodes, true)} {
		if seq, reversed := sift(seq); best == nil || reversed < bestReversed {
			best, bestReversed = seq, reversed
		}
	}
	for i, node := range best {
		pos[node.ID] = i
	}

	// rebuild adjacency, weights and minimum lengths from the sequence
	type rankedEdge struct {
		src, dst *Node
		minlen   int32
		weight   float32
	}
	var edges []rankedEdge
	for _, node := range graph.Nodes {
		for _, dst := range node.Out {
			switch {
			case dst == node:
				// drop self-loop
			case pos[node.ID] < pos[dst.ID]:
				edges = append(edges, rankedEdge{node, dst, graph.MinLen(node, dst), graph.Weight(node, dst)})
			default:
				edges = append(edges, rankedEdge{dst, node, graph.MinLen(node, dst), graph.Weight(node, dst)})
			}
		}
	}
	graph.weights = nil
	graph.minlens = nil
	for _, node := range graph.Nodes {
		node.In.Clear()
		node.Out.Clear()
	}
	for _, edge := range edges {
		src, dst := edge.src, edge.dst
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
