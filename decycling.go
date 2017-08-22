package layout

import "sort"

func Decycle(graph *Graph) {
	DecycleDepthFirst(graph)
}

func sortNodesByOutdegree(graph *Graph, nodes NodeIDs) {
	sort.Slice(nodes, func(i, k int) bool {
		aid, bid := nodes[i], nodes[k]
		a, b := graph.Nodes[aid], graph.Nodes[bid]

		if len(a.Out) == len(b.Out) {
			return len(a.In) < len(b.In)
		}
		return len(a.Out) > len(b.Out)
	})
}

func nodesByOutdegree(graph *Graph) NodeIDs {
	nodes := make(NodeIDs, 0, len(graph.Nodes))
	for id := range nodes {
		nodes[id] = NodeID(id)
	}

	sortNodesByOutdegree(graph, nodes)

	return nodes
}

func RemoveSelfLoops(graph *Graph) {
	for i, node := range graph.Nodes {
		id := NodeID(i)
		node.In.Remove(id)
		node.Out.Remove(id)
	}
}

func DecycleOrder(graph *Graph) {
	RemoveSelfLoops(graph)

	edges, processed := make(dagEdgeTable), make([]bool, len(graph.Nodes))

	var process func(did NodeID)
	process = func(did NodeID) {
		if processed[did] {
			return
		}
		processed[did] = true

		dst := graph.Nodes[did]
		for _, sid := range dst.In {
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
	RemoveSelfLoops(graph)

	edges, processed := make(dagEdgeTable), make([]bool, len(graph.Nodes))

	var process func(did NodeID)
	process = func(did NodeID) {
		if processed[did] {
			return
		}
		processed[did] = true

		dst := graph.Nodes[did]
		for _, sid := range dst.In {
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
	RemoveSelfLoops(graph)

	edges, processed := make(dagEdgeTable), make([]bool, len(graph.Nodes))

	var process func(did NodeID)
	process = func(did NodeID) {
		if processed[did] {
			return
		}
		processed[did] = true

		dst := graph.Nodes[did]
		for _, sid := range dst.In {
			if processed[sid] {
				edges.Include(sid, did)
			} else {
				edges.Include(did, sid)
				process(sid)
			}
		}
	}

	// TODO: after each process, re-sort based on outdegree
	for _, nid := range nodesByOutdegree(graph) {
		process(nid)
	}

	graph.setEdges(edges)
}

type dagEdgeTable map[[2]NodeID]struct{}

func (et dagEdgeTable) Include(src, dst NodeID) {
	if _, exists := et[[2]NodeID{dst, src}]; exists {
		return
	}
	et[[2]NodeID{src, dst}] = struct{}{}
}

func (graph *Graph) setEdges(edges dagEdgeTable) {
	// recreate inbound links from outbound
	for _, node := range graph.Nodes {
		node.In.Clear()
		node.Out.Clear()
	}

	for edge := range edges {
		graph.Edge(edge[0], edge[1])
	}
}
