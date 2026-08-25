package layout

// Graph is a set of nodes and edges together with the default styling
// used for nodes that don't specify their own.
type Graph struct {
	ID       string
	Directed bool

	// Defaults for nodes and edges that leave the value unset
	LineHeight Length
	FontSize   Length
	Shape      Shape

	// NodePadding is the horizontal space reserved around each node,
	// RowPadding the vertical space between ranks, and EdgePadding the
	// space reserved for an edge passing between nodes.
	NodePadding Length
	RowPadding  Length
	EdgePadding Length

	NodeByID map[string]*Node
	Nodes    []*Node
	Edges    []*Edge
	// SameRank groups nodes that must be placed on the same rank
	SameRank [][]*Node
}

// NewGraph creates an empty undirected graph with default styling.
func NewGraph() *Graph {
	graph := &Graph{}

	graph.LineHeight = 16 * Point
	graph.Shape = Auto

	graph.NodeByID = make(map[string]*Node)
	return graph
}

// NewDigraph creates an empty directed graph with default styling.
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
// When a node with the same id already exists it returns false
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

// AddEdge adds an edge without checking for duplicates.
func (graph *Graph) AddEdge(edge *Edge) {
	graph.Edges = append(graph.Edges, edge)
}

// minvector sets a to the component-wise minimum of a and b
func minvector(a *Vector, b Vector) {
	if b.X < a.X {
		a.X = b.X
	}
	if b.Y < a.Y {
		a.Y = b.Y
	}
}

// maxvector sets a to the component-wise maximum of a and b
func maxvector(a *Vector, b Vector) {
	if b.X > a.X {
		a.X = b.X
	}
	if b.Y > a.Y {
		a.Y = b.Y
	}
}

// Bounds returns the bounding box of all nodes and edge paths.
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
