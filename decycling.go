package layout

func NodesByOutdegree(graph *Graph) NodeIDs {
	nodes := make(NodeIDs, 0, len(graph.Nodes))
	for id := range nodes {
		nodes[id] = NodeID(id)
	}
	graph.Sort(nodes, func(a, b *Node) bool {
		if len(a.Out) == len(b.Out) {
			return len(a.In) < len(b.In)
		}
		return len(a.Out) > len(b.Out)
	})
	return nodes
}

func Decycle(graph *Graph) { Decycle_Outdegree(graph) }

func Decycle_Outdegree(graph *Graph) {
	seen := make([]bool, len(graph.Nodes))

	var flipIn func(did NodeID)
	flipIn = func(did NodeID) {
		seen[did] = true
		dst := graph.Nodes[did]
		for _, sid := range dst.In.Copy() {
			if !seen[sid] {
				src := graph.Nodes[sid]

				// flips the edge
				dst.In.Remove(sid)
				dst.Out.Add(sid)
				src.In.Add(did)
				src.Out.Remove(did)
			}
		}
	}

	// sort by outdegree
	nodes := make(NodeIDs, 0, len(graph.Nodes))
	for id := range nodes {
		nodes[id] = NodeID(id)
	}
	graph.Sort(nodes, func(a, b *Node) bool {
		if len(a.Out) == len(b.Out) {
			return len(a.In) < len(b.In)
		}
		return len(a.Out) > len(b.Out)
	})

	// remove cycles
	for _, nid := range NodesByOutdegree(graph) {
		if !seen[nid] {
			flipIn(nid)
		}
	}
}

func Decycle_DepthFirst(graph *Graph) {
	seen := make([]bool, len(graph.Nodes))

	var flipIn func(did NodeID)
	flipIn = func(did NodeID) {
		seen[did] = true
		dst := graph.Nodes[did]
		for _, sid := range dst.In.Copy() {
			if !seen[sid] {
				src := graph.Nodes[sid]

				// flips the edge
				dst.In.Remove(sid)
				dst.Out.Add(sid)
				src.In.Add(did)
				src.Out.Remove(did)

				flipIn(sid)
			}
		}
	}

	// remove cycles
	for _, nid := range NodesByOutdegree(graph) {
		if !seen[nid] {
			flipIn(nid)
		}
	}
}
