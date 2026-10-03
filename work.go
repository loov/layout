package layout

import "math"

// lgraph is the working copy of a Graph that layout computes on, so that
// the caller's graph is never modified. The embedded Graph holds a copy
// of the settings; its node, edge and cluster slices are nil, and the
// fields below hold copies of them instead.
type lgraph struct {
	Graph
	def *Graph

	Nodes    []*lnode
	Edges    []*ledge
	Clusters []*lcluster
	SameRank [][]*lnode
	MinRank  []*lnode
	MaxRank  []*lnode
}

// lnode is the working copy of a Node.
type lnode struct {
	Node
	def *Node
}

// ledge is the working copy of an Edge, between working nodes.
type ledge struct {
	Edge
	def *Edge

	From, To *lnode
}

// lcluster is the working copy of a Cluster, of working nodes.
type lcluster struct {
	Cluster
	def *Cluster

	Nodes  []*lnode
	Parent *lcluster
}

// newWorkGraph copies a validated graph for layout. Nodes, edges and
// clusters listed more than once get one copy each.
func newWorkGraph(def *Graph) *lgraph {
	g := &lgraph{Graph: *def, def: def}
	g.Graph.NodeByID = nil
	g.Graph.Nodes, g.Graph.Edges, g.Graph.Clusters = nil, nil, nil
	g.Graph.SameRank, g.Graph.MinRank, g.Graph.MaxRank = nil, nil, nil

	nodes := make(map[*Node]*lnode, len(def.Nodes))
	node := func(n *Node) *lnode {
		w := nodes[n]
		if w == nil {
			w = &lnode{Node: *n, def: n}
			nodes[n] = w
		}
		return w
	}
	group := func(defs []*Node) []*lnode {
		var out []*lnode
		for _, n := range defs {
			out = append(out, node(n))
		}
		return out
	}
	g.Nodes = group(def.Nodes)

	edges := make(map[*Edge]*ledge, len(def.Edges))
	for _, e := range def.Edges {
		w := edges[e]
		if w == nil {
			w = &ledge{Edge: *e, def: e, From: node(e.From), To: node(e.To)}
			edges[e] = w
		}
		g.Edges = append(g.Edges, w)
	}

	clusters := make(map[*Cluster]*lcluster, len(def.Clusters))
	for _, c := range def.Clusters {
		if clusters[c] == nil {
			clusters[c] = &lcluster{Cluster: *c, def: c}
		}
		g.Clusters = append(g.Clusters, clusters[c])
	}
	for c, w := range clusters {
		w.Nodes = group(c.Nodes)
		if c.Parent != nil {
			w.Parent = clusters[c.Parent]
		}
	}

	for _, same := range def.SameRank {
		g.SameRank = append(g.SameRank, group(same))
	}
	g.MinRank = group(def.MinRank)
	g.MaxRank = group(def.MaxRank)
	return g
}

// copyBack writes the computed layout into the caller's graph.
func (g *lgraph) copyBack() {
	def := g.def
	settings := g.Graph
	settings.NodeByID = def.NodeByID
	settings.Nodes, settings.Edges, settings.Clusters = def.Nodes, def.Edges, def.Clusters
	settings.SameRank, settings.MinRank, settings.MaxRank = def.SameRank, def.MinRank, def.MaxRank
	*def = settings
	for _, n := range g.Nodes {
		*n.def = n.Node
	}
	for _, e := range g.Edges {
		*e.def = e.Edge
	}
	for _, c := range g.Clusters {
		*c.def = c.Cluster
	}
}

// Bounds returns the bounding box of all nodes and edge paths.
func (g *lgraph) Bounds() (min, max Vector) {
	inf := Length(math.Inf(1))
	min, max = Vector{inf, inf}, Vector{-inf, -inf}
	for _, node := range g.Nodes {
		minvector(&min, node.TopLeft())
		maxvector(&max, node.BottomRight())
	}
	for _, cluster := range g.Clusters {
		minvector(&min, cluster.TopLeft)
		maxvector(&max, cluster.BottomRight)
	}
	for _, edge := range g.Edges {
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

func (cluster *lcluster) depth() int {
	d := 0
	for c := cluster; c != nil; c = c.Parent {
		d++
	}
	return d
}
