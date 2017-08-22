package layout

func Decycle(graph *Graph) {
	DecycleDepthFirst(graph)
}

func DecycleOrder(graph *Graph) {
	if !graph.IsCyclic() {
		return
	}

	edges, processed := make(dagEdgeTable), make([]bool, len(graph.Nodes))

	var process func(did NodeID)
	process = func(did NodeID) {
		if processed[did] {
			return
		}
		processed[did] = true

		dst := graph.Nodes[did]
		for _, sid := range dst.In {
			if sid == did {
				continue
			}
			if processed[sid] {
				edges.Include(sid, did)
			} else {
				edges.Include(did, sid)
			}
		}
	}

	for i := range graph.Nodes {
		process(NodeID(i))
	}

	graph.setEdges(edges)
}

func DecycleOutdegree(graph *Graph) {
	if !graph.IsCyclic() {
		return
	}

	edges, processed := make(dagEdgeTable), make([]bool, len(graph.Nodes))

	var process func(did NodeID)
	process = func(did NodeID) {
		if processed[did] {
			return
		}
		processed[did] = true

		dst := graph.Nodes[did]
		for _, sid := range dst.In {
			if sid == did {
				continue
			}
			if processed[sid] {
				edges.Include(sid, did)
			} else {
				edges.Include(did, sid)
			}
		}
	}

	// TODO: after each process, re-sort based on outdegree
	for _, nid := range nodesByOutdegree(graph) {
		process(nid)
	}

	graph.setEdges(edges)
}

func DecycleDepthFirst(graph *Graph) {
	if !graph.IsCyclic() {
		return
	}

	edges, processed := make(dagEdgeTable), make([]bool, len(graph.Nodes))

	var process func(did NodeID)
	process = func(did NodeID) {
		if processed[did] {
			return
		}
		processed[did] = true

		dst := graph.Nodes[did]
		for _, sid := range dst.In {
			if sid == did {
				continue
			}
			if processed[sid] {
				edges.Include(sid, did)
			} else {
				edges.Include(did, sid)
				process(sid)
			}
		}
	}

	// TODO: after each process, re-sort based on outdegree
	for _, node := range graph.Nodes {
		sortNodesByOutdegree(graph, node.In)
	}
	for _, nid := range nodesByOutdegree(graph) {
		process(nid)
	}

	graph.setEdges(edges)
}
