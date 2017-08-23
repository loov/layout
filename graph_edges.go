package layout

type Edge [2]*Node
type EdgeSet map[Edge]struct{}

func NewEdgeSet() EdgeSet { return make(EdgeSet) }

func (edges EdgeSet) Include(src, dst *Node) {
	if _, exists := edges[Edge{dst, src}]; exists {
		return
	}

	edges[Edge{src, dst}] = struct{}{}
}

func (edges EdgeSet) SetTo(graph *Graph) {
	// recreate inbound links from outbound
	for _, node := range graph.Nodes {
		node.In.Clear()
		node.Out.Clear()
	}

	for edge := range edges {
		graph.AddEdge(edge[0], edge[1])
	}
}
