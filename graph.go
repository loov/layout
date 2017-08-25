package layout

type Graph struct {
	ID       string
	Directed bool

	// Defaults
	LineHeight Length
	FontSize   Length
	Shape      Shape

	NodePadding Length
	RowPadding  Length

	NodeByID map[string]*Node
	Nodes    []*Node
	Edges    []*Edge
}

func NewGraph() *Graph {
	graph := &Graph{}

	graph.FontSize = 14 * Point
	graph.LineHeight = 16 * Point
	graph.Shape = Auto

	graph.NodePadding = graph.LineHeight
	graph.RowPadding = graph.LineHeight * 2

	graph.NodeByID = make(map[string]*Node)
	return graph
}

func NewDigraph() *Graph {
	graph := NewGraph()
	graph.Directed = true
	return graph
}

// Node finds or creates node with id
func (graph *Graph) Node(id string) *Node {
	if id == "" {
		panic("invalid node id")
	}

	node, found := graph.NodeByID[id]
	if !found {
		node = NewNode(id)
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

	edge := NewEdge(source, target)
	edge.Directed = graph.Directed
	graph.AddEdge(edge)
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

func minvector(a *Vector, b Vector) {
	if b.X < a.X {
		a.X = b.X
	}
	if b.Y < a.Y {
		a.Y = b.Y
	}
}

func maxvector(a *Vector, b Vector) {
	if b.X > a.X {
		a.X = b.X
	}
	if b.Y > a.Y {
		a.Y = b.Y
	}
}

func (graph *Graph) Bounds() (min, max Vector) {
	for _, node := range graph.Nodes {
		minvector(&min, node.TopLeft())
		maxvector(&max, node.BottomRight())
	}

	for _, edge := range graph.Edges {
		for _, p := range edge.Path {
			minvector(&min, p)
			maxvector(&max, p)
		}
	}

	minvector(&min, max)
	maxvector(&max, min)

	return
}
