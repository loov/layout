package layout

import "math"

// Graph is a set of nodes and edges together with the default styling
// used for nodes that don't specify their own.
type Graph struct {
	ID       string
	Directed bool

	// RankDir is the direction ranks are laid out in, see RankDir constants.
	RankDir RankDir
	// Splines is how edges are routed and drawn, see Splines constants.
	Splines Splines
	// PackEdgeEnds spreads the ends of ortho edges on a side of a node
	// from its start, the left or the top when sideways, an edge padding
	// apart, instead of around the center, and lines nodes up at their
	// first end: the main path of a sideways layout then runs along the
	// top of its nodes. text.Prepare sets it for sideways layouts.
	PackEdgeEnds bool

	// MeasureText returns the half size of a single line of text in the
	// given font. When nil, a built-in approximation is used.
	MeasureText func(line string, fontName string, fontSize Length) Vector

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

	// Pinned means every node already has a Center (for example from a
	// dot "pos" attribute); layouting then keeps them and only computes
	// missing edge paths, like dot -n.
	Pinned bool

	// NodeByID indexes Nodes by ID. Node and AddNode keep the two in
	// sync; a node appended to Nodes directly is missing from NodeByID,
	// and Node then creates a second node with the same ID.
	NodeByID map[string]*Node
	Nodes    []*Node
	Edges    []*Edge
	// SameRank groups nodes that must be placed on the same rank
	SameRank [][]*Node
	// MinRank and MaxRank hold nodes pinned to the first and last rank
	MinRank, MaxRank []*Node
	// Clusters are groups of nodes drawn inside a box
	Clusters []*Cluster
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

// AddNode adds a new node to Nodes and NodeByID; a node without an ID
// is only added to Nodes.
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
	inf := Length(math.Inf(1))
	min, max = Vector{inf, inf}, Vector{-inf, -inf}
	for _, node := range graph.Nodes {
		minvector(&min, node.TopLeft())
		maxvector(&max, node.BottomRight())
	}

	for _, cluster := range graph.Clusters {
		minvector(&min, cluster.TopLeft)
		maxvector(&max, cluster.BottomRight)
	}
	for _, edge := range graph.Edges {
		for _, p := range edge.Path {
			minvector(&min, p)
			maxvector(&max, p)
		}
		if edge.Label != "" {
			minvector(&min, Vector{edge.LabelPos.X - edge.LabelRadius.X, edge.LabelPos.Y - edge.LabelRadius.Y})
			maxvector(&max, Vector{edge.LabelPos.X + edge.LabelRadius.X, edge.LabelPos.Y + edge.LabelRadius.Y})
		}
	}

	if min.X > max.X { // nothing to bound
		return Vector{}, Vector{}
	}
	return
}
