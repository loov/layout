package layout

import (
	"sort"
)

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

func sortNodesAscending(nodes NodeIDs) {
	sort.Slice(nodes, func(i, k int) bool {
		return nodes[i] < nodes[k]
	})
}

func nodesByOutdegree(graph *Graph) NodeIDs {
	nodes := make(NodeIDs, len(graph.Nodes))
	for id := range nodes {
		nodes[id] = NodeID(id)
	}

	sortNodesByOutdegree(graph, nodes)
	return nodes
}
