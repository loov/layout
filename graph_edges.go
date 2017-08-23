package layout

// NewGraphFromEdgeList creates a graph from edge list
//
// Example:
//     graph := NewGraphFromEdgeList([][]int{
//         0: []int{1,2},
//         1: []int{2,0},
//     })
//
//  Creates an graph with edges 0 -> 1, 0 -> 2, 1 -> 2, 1 -> 0.
//
func NewGraphFromEdgeList(edgeList [][]int) *Graph {
	graph := NewGraph()

	for from, out := range edgeList {
		for _, to := range out {
			graph.ensureNode(from)
			graph.ensureNode(to)
			graph.AddEdge(graph.Nodes[from], graph.Nodes[to])
		}
	}

	return graph
}

// ConvertToEdgeList creates edge list
// NewGraphFromEdgeList(edgeList).ConvertToEdgeList() == edgeList
func (graph *Graph) ConvertToEdgeList() [][]int {
	edges := make([][]int, 0, graph.NodeCount())
	for _, node := range graph.Nodes {
		list := make([]int, 0, len(node.Out))
		for _, out := range node.Out {
			list = append(list, int(out.ID))
		}
		edges = append(edges, list)
	}
	return edges
}

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
