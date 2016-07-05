package glay

type NodeID int
type NodeIDs []NodeID

func (nodes *NodeIDs) Add(id NodeID) { *nodes = append(*nodes, id) }
func (nodes *NodeIDs) Remove(id NodeID) {
	for i, nid := range *nodes {
		if nid == id {
			*nodes = append((*nodes)[:i], (*nodes)[i+1:]...)
			return
		}
	}
	panic("id not found")
}

type Graph struct {
	Nodes  map[NodeID]*Node
	ByRank []NodeIDs
	nextID NodeID
}

func NewGraph() *Graph {
	return &Graph{
		Nodes: make(map[NodeID]*Node),
	}
}

type Node struct {
	ID      NodeID
	In      NodeIDs
	Out     NodeIDs
	Rank    int
	Virtual bool
}

func (g *Graph) Node() (NodeID, *Node) {
	n := &Node{ID: g.nextID}
	g.Nodes[n.ID] = n
	g.nextID++
	return n.ID, n
}

func (g *Graph) Edge(sid, did NodeID) {
	src, dst := g.Nodes[sid], g.Nodes[did]
	src.Out.Add(did)
	dst.In.Add(sid)
}

/*
type Graph struct {
	Nodes  []NodeInfo
	Edges  []EdgeInfo
	ByRank [][]NodeID
}

func NewGraph() *Graph {
	return &Graph{}
}

type Vector struct{ X, Y float32 }

func (a Vector) Add(b Vector) Vector    { return Vector{a.X + b.X, a.Y + b.Y} }
func (a Vector) Scale(s float32) Vector { return Vector{a.X * s, a.Y * s} }

type NodeID uint32
type EdgeID uint32

type NodeInfo struct {
	Center   Vector
	HalfSize Vector
	Rank     int

	Out   []NodeID
	Dummy bool
}

type EdgeInfo struct {
	Source      NodeID
	Destination NodeID
	Center      Vector
	HalfSize    Vector

	Dummy  bool
	Ignore bool
}

func (g *Graph) Node(info NodeInfo) NodeID {
	id := NodeID(len(g.Nodes))
	g.Nodes = append(g.Nodes, info)
	return id
}

func (g *Graph) Edge(source, destination NodeID) EdgeID {
	return g.EdgeEx(EdgeInfo{Source: source, Destination: destination})
}

func (g *Graph) EdgeEx(edge EdgeInfo) EdgeID {
	id := EdgeID(len(g.Edges))
	g.Edges = append(g.Edges, edge)

	s := &g.Nodes[edge.Source]
	s.Out = append(s.Out, edge.Destination)
	return id
}

func BreakCycles(graph *Graph) {

}

func CreateDummies(graph *Graph) {
	dummies := []NodeInfo{}
	nextID := NodeID(len(graph.Nodes))
	for sid := range graph.Nodes {
		src := &graph.Nodes[sid]
		for i, did := range src.Out {
			dst := &graph.Nodes[did]
			if dst.Rank-src.Rank <= 1 {
				continue
			}
			for rank := dst.Rank - 1; rank > src.Rank; rank-- {
				up := NodeInfo{Out: []NodeID{did}, Dummy: true}
				graph.Edges = append(graph.Edges, EdgeInfo{Source: did, Destination: nextID})
				did = nextID
				nextID++
				dummies = append(dummies, up)
				graph.ByRank[rank] = append(graph.ByRank[rank], did)
			}
			graph.Edges = append(graph.Edges, EdgeInfo{Source: NodeID(sid), Destination: did})
			src.Out[i] = did
		}
	}
	graph.Nodes = append(graph.Nodes, dummies...)
}

func AssignPositions(graph *Graph) {
	const padding = float32(10.0)

	top := padding
	for _, nodes := range graph.ByRank {
		var rowheight float32
		for _, id := range nodes {
			n := &graph.Nodes[id]
			rowheight = maxf32(rowheight, n.HalfSize.Y*2)
		}

		left := padding
		for _, id := range nodes {
			n := &graph.Nodes[id]
			n.Center.Y = top + rowheight/2
			n.Center.X = left + n.HalfSize.X
			left += n.HalfSize.X*2 + padding
		}

		top += rowheight + padding
	}

	for eid := range graph.Edges {
		e := &graph.Edges[eid]
		s, d := &graph.Nodes[e.Source], &graph.Nodes[e.Destination]

		e.Center = s.Center.Add(d.Center).Scale(0.5)
	}
}

*/
