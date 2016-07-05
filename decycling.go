package glay

//TODO: test whether this is correct
func DecycleDepthFirst(graph *Graph) {
	seen := make([]bool, graph.nextID) // todo convert to bitvector

	var recurse func(did NodeID)
	recurse = func(did NodeID) {
		seen[did] = true
		dst := graph.Nodes[did]
		for _, sid := range dst.In {
			if !seen[sid] {
				src := graph.Nodes[sid]

				// flips the edge
				dst.In.Remove(sid)
				dst.Out.Add(sid)
				src.In.Add(did)
				src.Out.Remove(did)

				recurse(sid)
			}
		}
	}

	for id := NodeID(0); id < graph.nextID; id++ {
		if !seen[id] {
			recurse(id)
		}
	}
}
