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

func (node *Node) InDegree() int  { return len(node.In) }
func (node *Node) OutDegree() int { return len(node.Out) }

type Vector struct {
	X, Y float32
}

func NewGraph() *Graph {
	return &Graph{}
}

func NewGraphFromEdgeList(edgeList [][]int) *Graph {
	graph := NewGraph()

	for range edgeList {
		graph.AddNode()
	}

	for from, out := range edgeList {
		for _, to := range out {
			graph.AddEdge(graph.Nodes[from], graph.Nodes[to])
		}
	}

	return graph
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
