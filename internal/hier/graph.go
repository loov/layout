package hier

import "strconv"

// Graph is the basic graph
type Graph struct {
	Nodes Nodes
	// SameRank groups nodes that must share a rank
	SameRank []Nodes
	// MinRank and MaxRank hold nodes pinned to the first and last rank
	MinRank, MaxRank Nodes
	// Clusters groups nodes into boxes, see AddClusterBorders
	Clusters []*Cluster
	// Flat holds edges between nodes on the same rank; they are removed
	// from In/Out by Rank and drawn sideways.
	Flat [][2]*Node
	// weights of edges by (src, dst); missing means 1
	weights map[[2]ID]float32
	// minlens holds the minimum rank span of edges by (src, dst);
	// missing means 1
	minlens map[[2]ID]int32
	// Ranking
	ByRank []Nodes
	// FamilyGap keeps neighbors in a rank further apart that are children
	// of different fans, so that the children of a node stay together
	FamilyGap float32
	// EndGap is how far apart packed ends of edges on a side of a node
	// are, from its anchor on, within its EndRoom; 0 when they aren't
	// packed, see Position
	EndGap float32
}

// ID is an unique identifier to a Node
type ID uint32

// Node is the basic information about a node
type Node struct {
	ID ID

	// the flags sit beside ID, in what would be padding
	Virtual bool
	// BorderLeft/BorderRight mark the virtual border nodes of a cluster
	BorderLeft, BorderRight bool

	// Cluster the node belongs to, if any
	Cluster *Cluster

	In  Nodes
	Out Nodes

	Label string

	// Rank info
	Rank int

	// Ordering info
	Coef  float32
	GridX float32
	Pos   int // index within its rank, see assignPos
	// neighbor positions cached for the transpose step, see OrderRanksTranspose
	inPos, outPos []int32

	// Visuals
	Center Vector
	Radius Vector
	// Anchor is the x offset from Center where the edge passes through
	// a virtual node, for nodes that carry a label beside the edge
	Anchor float32
	// EndRoom is how far packed ends of edges on a side can spread from
	// the anchor, see Graph.EndGap
	EndRoom float32
}

// String returns node label
func (node *Node) String() string {
	if node.Label == "" && node.Virtual {
		return "v" + strconv.Itoa(int(node.ID))
	}
	if node.Label == "" {
		return "#" + strconv.Itoa(int(node.ID))
	}
	return node.Label
}

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

// AddEdge adds a new edge with weight 1
func (graph *Graph) AddEdge(src, dst *Node) {
	src.Out.Append(dst)
	dst.In.Append(src)
}

// AddWeightedEdge adds a new edge with the given weight.
// Heavier edges are kept shorter and straighter.
func (graph *Graph) AddWeightedEdge(src, dst *Node, weight float32) {
	graph.AddEdge(src, dst)
	graph.SetWeight(src, dst, weight)
}

// Weight returns the weight of edge src -> dst
func (graph *Graph) Weight(src, dst *Node) float32 {
	if len(graph.weights) == 0 {
		return 1
	}
	if w, ok := graph.weights[[2]ID{src.ID, dst.ID}]; ok {
		return w
	}
	return 1
}

// SetWeight sets the weight of edge src -> dst
func (graph *Graph) SetWeight(src, dst *Node, weight float32) {
	if weight == 1 {
		delete(graph.weights, [2]ID{src.ID, dst.ID})
		return
	}
	if graph.weights == nil {
		graph.weights = map[[2]ID]float32{}
	}
	graph.weights[[2]ID{src.ID, dst.ID}] = weight
}

// MinLen returns the minimum number of ranks edge src -> dst must span
func (graph *Graph) MinLen(src, dst *Node) int32 {
	if len(graph.minlens) == 0 {
		return 1
	}
	if n, ok := graph.minlens[[2]ID{src.ID, dst.ID}]; ok {
		return n
	}
	return 1
}

// SetMinLen sets the minimum number of ranks edge src -> dst must span
func (graph *Graph) SetMinLen(src, dst *Node, minlen int32) {
	if minlen < 0 {
		minlen = 0
	}
	if minlen == 1 {
		delete(graph.minlens, [2]ID{src.ID, dst.ID})
		return
	}
	if graph.minlens == nil {
		graph.minlens = map[[2]ID]int32{}
	}
	graph.minlens[[2]ID{src.ID, dst.ID}] = minlen
}

// InWeight returns the total weight of incoming edges
func (graph *Graph) InWeight(node *Node) float32 {
	total := float32(0)
	for _, src := range node.In {
		total += graph.Weight(src, node)
	}
	return total
}

// OutWeight returns the total weight of outgoing edges
func (graph *Graph) OutWeight(node *Node) float32 {
	total := float32(0)
	for _, dst := range node.Out {
		total += graph.Weight(node, dst)
	}
	return total
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

// CountRoots returns count of roots
func (graph *Graph) CountRoots() int {
	total := 0
	for _, node := range graph.Nodes {
		if node.InDegree() == 0 {
			total++
		}
	}
	return total
}

// CountEdges counts all edges, including duplicates
func (graph *Graph) CountEdges() int {
	total := 0
	for _, src := range graph.Nodes {
		total += len(src.Out)
	}
	return total
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
