package layout

type NodeID int
type NodeIDs []NodeID

type Graph struct {
	Nodes     []*Node
	ByRank    []NodeIDs
	Positions []Position
}

func NewGraph() *Graph {
	return &Graph{}
}

type Node struct {
	ID      NodeID
	In      NodeIDs
	Out     NodeIDs
	Rank    int
	Virtual bool
}

type Position struct{ X, Y float32 }

func (graph *Graph) Node() (NodeID, *Node) {
	n := &Node{ID: NodeID(len(graph.Nodes))}
	graph.Nodes = append(graph.Nodes, n)
	return n.ID, n
}

func (graph *Graph) Edge(sid, did NodeID) {
	src, dst := graph.Nodes[sid], graph.Nodes[did]
	src.Out.Add(did)
	dst.In.Add(sid)
}

func (graph *Graph) CrossingsUp(uid, vid NodeID) int {
	u, v := graph.Nodes[uid], graph.Nodes[vid]
	assert(u.Rank == v.Rank, "u and v are from different ranks")
	if u.Rank == 0 {
		return 0
	}

	count := 0
	prev := graph.ByRank[u.Rank-1]
	for _, w := range u.In {
		for _, z := range v.In {
			if prev.IndexOf(z) < prev.IndexOf(w) {
				count++
			}
		}
	}
	return count
}

func (graph *Graph) CrossingsDown(uid, vid NodeID) int {
	u, v := graph.Nodes[uid], graph.Nodes[vid]
	assert(u.Rank == v.Rank, "u and v are from different ranks")
	if u.Rank == len(graph.ByRank)-1 {
		return 0
	}

	count := 0
	next := graph.ByRank[u.Rank+1]
	for _, w := range u.In {
		for _, z := range v.In {
			if next.IndexOf(z) < next.IndexOf(w) {
				count++
			}
		}
	}
	return count
}

func (graph *Graph) Crossings(uid, vid NodeID) int {
	return graph.CrossingsDown(uid, vid) + graph.CrossingsUp(uid, vid)
}

func (a NodeIDs) Copy() NodeIDs {
	r := make(NodeIDs, len(a))
	copy(r, a)
	return r
}

func (nodes *NodeIDs) Add(b NodeID) { *nodes = append(*nodes, b) }

func (nodes *NodeIDs) Remove(b NodeID) {
	i := nodes.IndexOf(b)
	assert(i >= 0, "id not found")
	*nodes = append((*nodes)[:i], (*nodes)[i+1:]...)
}

func (nodes NodeIDs) IndexOf(b NodeID) int {
	for i, id := range nodes {
		if id == b {
			return i
		}
	}
	return -1
}
