package layout

// Graph is the basic graph
type Graph struct {
	Nodes Nodes
	// Ranking
	ByRank []Nodes
}

// ID is an unique identifier to a Node
type ID int

// Node is the basic information about a node
type Node struct {
	ID ID

	Virtual bool

	In  Nodes
	Out Nodes

	Label string

	// Rank info
	Rank int

	// Ordering info
	Coef  float32
	GridX float32

	// Visuals
	Position Vector
	Radius   Vector
}

// String returns node label
func (node *Node) String() string { return node.Label }

// InDegree returns count of inbound edges
func (node *Node) InDegree() int { return len(node.In) }

// OutDegree returns count of outbound edges
func (node *Node) OutDegree() int { return len(node.Out) }

// Vector represents a 2D vector
type Vector struct {
	X, Y float32
}

// NewGraph creates an empty graph
func NewGraph() *Graph { return &Graph{} }

// ensureNode adds nodes until we have reached id
func (graph *Graph) ensureNode(id int) {
	for id >= len(graph.Nodes) {
		graph.AddNode()
	}
}

// NodeCount returns count of nodes
func (graph *Graph) NodeCount() int { return len(graph.Nodes) }

// AddNode adds a new node and returns it's ID
func (graph *Graph) AddNode() *Node {
	node := &Node{ID: ID(len(graph.Nodes))}
	graph.Nodes = append(graph.Nodes, node)
	return node
}

// AddEdge adds a new edge to the node
func (graph *Graph) AddEdge(src, dst *Node) {
	src.Out.Append(dst)
	dst.In.Append(src)
}

// Roots returns nodes without any incoming edges
func (graph *Graph) Roots() Nodes {
	nodes := Nodes{}
	for _, node := range graph.Nodes {
		if node.InDegree() == 0 {
			nodes.Append(node)
		}
	}
	return nodes
}

// CountUndirectedLinks counts unique edges in the graph excluding loops
func (graph *Graph) CountUndirectedLinks() int {
	counted := map[[2]ID]struct{}{}

	for _, src := range graph.Nodes {
		for _, dst := range src.Out {
			if src == dst {
				continue
			}

			a, b := src.ID, dst.ID
			if a > b {
				a, b = b, a
			}

			counted[[2]ID{a, b}] = struct{}{}
		}
	}
	return len(counted)
}
