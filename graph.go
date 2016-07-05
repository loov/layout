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
	Nodes  []*Node
	ByRank []NodeIDs
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

func (g *Graph) Node() (NodeID, *Node) {
	n := &Node{ID: NodeID(len(g.Nodes))}
	g.Nodes = append(g.Nodes, n)
	return n.ID, n
}

func (g *Graph) Edge(sid, did NodeID) {
	src, dst := g.Nodes[sid], g.Nodes[did]
	src.Out.Add(did)
	dst.In.Add(sid)
}

/*
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
