package layout

import "sort"

func Decycle(graph *Graph) {
	if !graph.IsCyclic() {
		return
	}

	DecycleDepthFirst(graph)
}

func DecycleOrder(graph *Graph) {
	edges, processed := NewEdgeSet(), NewNodeSet(graph.NodeCount())

	var process func(node *Node)
	process = func(dst *Node) {
		if !processed.Include(dst) {
			return
		}

		for _, src := range dst.In {
			if src == dst {
				continue
			}
			if processed.Contains(src) {
				edges.Include(src, dst)
			} else {
				edges.Include(dst, src)
			}
		}
	}

	for _, node := range graph.Nodes {
		process(node)
	}

	edges.SetTo(graph)
}

func DecycleOutdegree(graph *Graph) {
	edges, processed := NewEdgeSet(), NewNodeSet(graph.NodeCount())

	var process func(node *Node)
	process = func(dst *Node) {
		if !processed.Include(dst) {
			return
		}

		for _, src := range dst.In {
			if src == dst {
				continue
			}
			if processed.Contains(src) {
				edges.Include(src, dst)
			} else {
				edges.Include(dst, src)
			}
		}
	}

	// TODO: after each process, re-sort based on outdegree
	for _, node := range graph.Nodes.Clone().SortDescending() {
		process(node)
	}
	edges.SetTo(graph)
}

func DecycleDepthFirst(graph *Graph) {
	edges, processed := NewEdgeSet(), NewNodeSet(graph.NodeCount())

	indegree := make([]int, graph.NodeCount())
	outdegree := make([]int, graph.NodeCount())
	for _, node := range graph.Nodes {
		indegree[node.ID] = node.InDegree()
		outdegree[node.ID] = node.OutDegree()
	}

	sortByOutdegree := func(nodes Nodes) {
		sort.Slice(nodes, func(i, k int) bool {
			a, b := nodes[i], nodes[k]
			if outdegree[a.ID] == outdegree[b.ID] {
				return indegree[a.ID] < indegree[b.ID]
			}
			return outdegree[a.ID] > outdegree[b.ID]
		})
	}

	var process func(node *Node)
	process = func(dst *Node) {
		if !processed.Include(dst) {
			return
		}

		toRecurse := Nodes{}
		for _, src := range dst.In {
			if src == dst {
				continue
			}

			if processed.Contains(src) {
				edges.Include(src, dst)
			} else {
				indegree[dst.ID]--
				indegree[src.ID]++
				outdegree[src.ID]--
				outdegree[dst.ID]++

				edges.Include(dst, src)
				toRecurse.Append(src)
			}
		}

		for _, src := range toRecurse {
			process(src)
		}
	}

	roots := *graph.Nodes.Clone()
	var root *Node
	sortByOutdegree(roots)
	for len(roots) > 0 {
		root, roots = roots[0], roots[1:]
		if processed.Contains(root) {
			continue
		}

		process(root)
		sortByOutdegree(roots)
	}

	edges.SetTo(graph)
}
