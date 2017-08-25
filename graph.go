package layout

type Graph struct {
	ID       string
	Directed bool

	NodeByID map[string]*Node
	Nodes    []*Node
	Edges    []*Edge
}

func NewGraph() *Graph {
	graph := &Graph{}
	graph.NodeByID = make(map[string]*Node)
	return graph
}

// Node finds or creates node with id
func (graph *Graph) Node(id string) *Node {
	if id == "" {
		panic("invalid node id")
	}

	node, found := graph.NodeByID[id]
	if !found {
		node = &Node{}
		node.ID = id
		graph.AddNode(node)
	}
	return node
}

// Edge finds or creates new edge based on ids
func (graph *Graph) Edge(from, to string) *Edge {
	source, target := graph.Node(from), graph.Node(to)
	for _, edge := range graph.Edges {
		if edge.From == source && edge.To == target {
			return edge
		}
	}

	edge := &Edge{}
	edge.From = source
	edge.To = target

	return edge
}

// AddNode adds a new node.
//
// When a node with the specified id already it will return false
// and the node is not added.
func (graph *Graph) AddNode(node *Node) bool {
	if node.ID != "" {
		_, found := graph.NodeByID[node.ID]
		if found {
			return false
		}
		graph.NodeByID[node.ID] = node
	}
	graph.Nodes = append(graph.Nodes, node)
	return true
}

func (graph *Graph) AddEdge(edge *Edge) {
	graph.Edges = append(graph.Edges, edge)
}
