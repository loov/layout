package buccheim

// Tree is the basic tree
type Tree struct {
	Nodes []Node
	Edges []ID
}

// ID is an unique identifier to a Node
type ID = int

// Node is the basic information about a node
type Node struct {
	Virtual bool

	Indegree   int
	Start, End int

	Parent   ID
	Ancestor ID
	Change   int
	Shift    int
	Offset   int
	Number   int

	Thread          ID
	LeftMostSibling ID

	Mod  float32
	X, Y float32
}

// Outdegree returns number of outgoing edges
func (node *Node) Outdegree() int { return node.End - node.Start }

// Vector represents a 2D vector
type Vector struct {
	X, Y float32
}

// NewTree creates an empty tree
func NewTree() *Tree { return &Tree{} }

// ensureNode adds nodes until we have reached id
func (tree *Tree) ensureNode(id int) {
	for id >= len(tree.Nodes) {
		tree.AddNode()
	}
}

// NodeCount returns count of nodes
func (tree *Tree) NodeCount() int { return len(tree.Nodes) }

// AddNode adds a new node and returns it's ID
func (tree *Tree) AddNode() ID {
	id := ID(len(tree.Nodes))
	tree.Nodes = append(tree.Nodes, Node{})
	return id
}

// AddEdge adds a new edge to the node
func (tree *Tree) SetEdges(src ID, dst []ID) {
	n := &tree.Nodes[src]

	for _, id := range dst {
		tree.Nodes[id].Indegree++
	}

	start := len(tree.Edges)
	tree.Edges = append(tree.Edges, dst...)
	end := len(tree.Edges)

	n.Start, n.End = start, end
}

// Roots returns nodes without any incoming edges
func (tree *Tree) Roots() []ID {
	nodes := []ID{}
	for id, node := range tree.Nodes {
		if node.Indegree == 0 {
			nodes = append(nodes, ID(id))
		}
	}
	if len(nodes) == 0 {
		return []ID{0}
	}
	return nodes
}

// CountRoots returns count of roots
func (tree *Tree) CountRoots() int {
	total := 0
	for _, node := range tree.Nodes {
		if node.Outdegree() == 0 {
			total++
		}
	}
	return total
}
