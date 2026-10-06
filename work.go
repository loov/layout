package layout

import (
	"math"
	"slices"
)

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

	// ForText prepares the layout for format/text, see Options.ForText
	ForText bool
	// Pinned means every node has a Pos; layout keeps them
	Pinned bool
	// merged holds the group ids of the edge ends that merge, at the
	// start and the end, see mergeEdges; the components of a graph
	// share it
	merged map[*ledge][2]int
}

// lnode is the working copy of a Node.
type lnode struct {
	Node
	def *Node

	Center Vector
	// Radius is half the size
	Radius Vector
	// pad is the padding layout added to Radius, for extra peripheries
	// and packed edge ends
	pad Vector
}

// ledge is the working copy of an Edge, between working nodes.
type ledge struct {
	Edge
	def *Edge

	From, To    *lnode
	Path        []Vector
	LabelPos    Vector // center of the label, when Label is set
	LabelRadius Vector // half size of the label
	// the labels at the ends, when HeadLabel and TailLabel are set
	head, tail endLabel
}

// endLabel is a label at an end of an edge: its center and half size
type endLabel struct{ pos, radius Vector }

// lcluster is the working copy of a Cluster, of working nodes.
type lcluster struct {
	Cluster
	def *Cluster

	Nodes  []*lnode
	Parent *lcluster

	TopLeft, BottomRight Vector
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
			w = &lnode{Node: *n, def: n, Radius: Vector{n.MinSize.X / 2, n.MinSize.Y / 2}}
			if n.Pos != nil {
				w.Center = *n.Pos
			}
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
			w = &ledge{Edge: *e, def: e, From: node(e.From), To: node(e.To), Path: slices.Clone(e.Pos)}
			if e.LabelPos != nil {
				w.LabelPos = *e.LabelPos
			}
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
	g.Pinned = pinned(def)
	return g
}

// pinned reports whether every node of the graph has a Pos.
func pinned(graph *Graph) bool {
	for _, node := range graph.Nodes {
		if node.Pos == nil {
			return false
		}
	}
	return len(graph.Nodes) > 0
}

// result returns the computed layout, index-aligned with the caller's
// graph.
func (g *lgraph) result() *Layout {
	settings := g.Graph
	def := g.def
	settings.NodeByID = def.NodeByID
	settings.Nodes, settings.Edges, settings.Clusters = def.Nodes, def.Edges, def.Clusters
	settings.SameRank, settings.MinRank, settings.MaxRank = def.SameRank, def.MinRank, def.MaxRank

	l := &Layout{
		Graph:        &settings,
		nodeIndex:    indexOf(def.Nodes),
		edgeIndex:    indexOf(def.Edges),
		clusterIndex: indexOf(def.Clusters),
	}
	for _, n := range g.Nodes {
		l.Nodes = append(l.Nodes, NodeBox{
			Center:   n.Center,
			Size:     Vector{2 * n.Radius.X, 2 * n.Radius.Y},
			Shape:    n.Shape,
			FontSize: n.FontSize,
			Label:    n.DefaultLabel(),
		})
	}
	for _, e := range g.Edges {
		path := EdgePath{Path: e.Path, FontSize: e.FontSize, Merged: g.merged[e]}
		if e.Label != "" {
			path.LabelCenter = e.LabelPos
			path.LabelSize = Vector{2 * e.LabelRadius.X, 2 * e.LabelRadius.Y}
		}
		if e.HeadLabel != "" {
			path.HeadLabelCenter, path.HeadLabelSize = e.head.pos, Vector{2 * e.head.radius.X, 2 * e.head.radius.Y}
		}
		if e.TailLabel != "" {
			path.TailLabelCenter, path.TailLabelSize = e.tail.pos, Vector{2 * e.tail.radius.X, 2 * e.tail.radius.Y}
		}
		l.Edges = append(l.Edges, path)
	}
	for _, c := range g.Clusters {
		l.Clusters = append(l.Clusters, ClusterBox{TopLeft: c.TopLeft, BottomRight: c.BottomRight})
	}
	return l
}

// work returns the working copy of a computed layout, for measuring it.
func (l *Layout) work() *lgraph {
	g := newWorkGraph(l.Graph)
	for i, n := range g.Nodes {
		box := l.Nodes[i]
		n.Center = box.Center
		n.Radius = Vector{box.Size.X / 2, box.Size.Y / 2}
		n.Shape, n.FontSize = box.Shape, box.FontSize
	}
	for i, e := range g.Edges {
		path := l.Edges[i]
		e.Path = path.Path
		e.LabelPos = path.LabelCenter
		e.LabelRadius = Vector{path.LabelSize.X / 2, path.LabelSize.Y / 2}
		e.head = endLabel{path.HeadLabelCenter, Vector{path.HeadLabelSize.X / 2, path.HeadLabelSize.Y / 2}}
		e.tail = endLabel{path.TailLabelCenter, Vector{path.TailLabelSize.X / 2, path.TailLabelSize.Y / 2}}
		e.FontSize = path.FontSize
	}
	for i, c := range g.Clusters {
		c.TopLeft, c.BottomRight = l.Clusters[i].TopLeft, l.Clusters[i].BottomRight
	}
	return g
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
		for _, end := range []endLabel{edge.head, edge.tail} {
			if end.radius != (Vector{}) {
				minvector(&min, end.pos.Sub(end.radius))
				maxvector(&max, end.pos.Add(end.radius))
			}
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
