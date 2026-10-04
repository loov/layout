package layout

import "math"

// Layout is a computed drawing of a Graph. Nodes, Edges and Clusters are
// index-aligned with the graph's: Nodes[i] is where Graph.Nodes[i] goes.
type Layout struct {
	// Graph is a copy of the laid out graph with its defaults filled in,
	// such as FontSize and the paddings, and with the settings
	// Options.ForText changes. Its Nodes, Edges and Clusters are the
	// caller's own.
	Graph *Graph

	Nodes    []NodeBox
	Edges    []EdgePath
	Clusters []ClusterBox

	// indexes into the slices above, built by layout
	nodeIndex    map[*Node]int
	edgeIndex    map[*Edge]int
	clusterIndex map[*Cluster]int
}

// NodeBox is the computed box of a node.
type NodeBox struct {
	Center Vector
	// Size is the full width and height, including extra outlines.
	Size Vector
	// Shape and FontSize are the node's own, or the graph defaults when
	// unset.
	Shape    Shape
	FontSize Length
	// Label is the text drawn in the node, see Node.DefaultLabel.
	Label string
}

// EdgePath is the computed path of an edge.
type EdgePath struct {
	Path []Vector
	// LabelCenter and LabelSize place the label, when the edge has one.
	LabelCenter Vector
	LabelSize   Vector
	// FontSize is the edge's own, or the graph default when unset.
	FontSize Length
	// Merged is the group of edges this one merges with at its start
	// and at its end, see Graph.MergeEdges; 0 where it doesn't. Merged
	// edges share the end, and run together to where they turn apart.
	Merged [2]int
}

// ClusterBox is the computed box of a cluster.
type ClusterBox struct {
	TopLeft, BottomRight Vector
}

// Node returns the box of a node of the graph.
func (l *Layout) Node(node *Node) NodeBox {
	return l.Nodes[find(l.nodeIndex, l.Graph.Nodes, node, "node "+node.String())]
}

// Edge returns the path of an edge of the graph.
func (l *Layout) Edge(edge *Edge) EdgePath {
	return l.Edges[find(l.edgeIndex, l.Graph.Edges, edge, "edge "+edge.String())]
}

// Cluster returns the box of a cluster of the graph.
func (l *Layout) Cluster(cluster *Cluster) ClusterBox {
	return l.Clusters[find(l.clusterIndex, l.Graph.Clusters, cluster, "cluster "+cluster.ID)]
}

// find returns the index of item, using index when layout built one
func find[T comparable](index map[T]int, items []T, item T, name string) int {
	if i, ok := index[item]; ok {
		return i
	}
	if index == nil {
		for i, it := range items {
			if it == item {
				return i
			}
		}
	}
	panic("layout: " + name + " is not in the graph")
}

// indexOf maps each item to its position; a repeated item maps to the last
func indexOf[T comparable](items []T) map[T]int {
	index := make(map[T]int, len(items))
	for i, item := range items {
		index[item] = i
	}
	return index
}

// Bounds returns the bounding box of all nodes, clusters, edge paths and
// labels.
func (l *Layout) Bounds() (min, max Vector) {
	inf := Length(math.Inf(1))
	min, max = Vector{inf, inf}, Vector{-inf, -inf}
	for _, box := range l.Nodes {
		minvector(&min, box.TopLeft())
		maxvector(&max, box.BottomRight())
	}
	for _, box := range l.Clusters {
		minvector(&min, box.TopLeft)
		maxvector(&max, box.BottomRight)
	}
	for i, path := range l.Edges {
		for _, p := range path.Path {
			minvector(&min, p)
			maxvector(&max, p)
		}
		if l.Graph.Edges[i].Label != "" {
			half := Vector{path.LabelSize.X / 2, path.LabelSize.Y / 2}
			minvector(&min, path.LabelCenter.Sub(half))
			maxvector(&max, path.LabelCenter.Add(half))
		}
	}
	if min.X > max.X { // nothing to bound
		return Vector{}, Vector{}
	}
	return min, max
}

// TopLeft returns the top left corner of the box.
func (box NodeBox) TopLeft() Vector { return Vector{box.Left(), box.Top()} }

// BottomRight returns the bottom right corner of the box.
func (box NodeBox) BottomRight() Vector { return Vector{box.Right(), box.Bottom()} }

// TopCenter returns the middle of the top side of the box.
func (box NodeBox) TopCenter() Vector { return Vector{box.Center.X, box.Top()} }

// BottomCenter returns the middle of the bottom side of the box.
func (box NodeBox) BottomCenter() Vector { return Vector{box.Center.X, box.Bottom()} }

// Left returns the x coordinate of the left side of the box.
func (box NodeBox) Left() Length { return box.Center.X - box.Size.X/2 }

// Top returns the y coordinate of the top side of the box.
func (box NodeBox) Top() Length { return box.Center.Y - box.Size.Y/2 }

// Right returns the x coordinate of the right side of the box.
func (box NodeBox) Right() Length { return box.Center.X + box.Size.X/2 }

// Bottom returns the y coordinate of the bottom side of the box.
func (box NodeBox) Bottom() Length { return box.Center.Y + box.Size.Y/2 }

// Boundary returns the point on the outline of the box's shape where the
// ray from the center towards p leaves it.
func (box NodeBox) Boundary(p Vector) Vector { return box.node().Boundary(p) }

// CompassPoint returns the point on the outline of the box's shape at the
// compass direction, or the center for Center and CompassAuto.
func (box NodeBox) CompassPoint(c Compass) Vector { return box.node().CompassPoint(c) }

// node returns the working node with the box's geometry
func (box NodeBox) node() *lnode {
	return &lnode{Node: Node{Shape: box.Shape}, Center: box.Center, Radius: Vector{box.Size.X / 2, box.Size.Y / 2}}
}
