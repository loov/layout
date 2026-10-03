package layout

// Cluster is a group of nodes drawn inside a common box. The box corners
// are computed by layouting.
type Cluster struct {
	ID    string
	Label string
	Nodes []*Node
	// Parent is the enclosing cluster, if any; Nodes includes the nodes of
	// nested clusters.
	Parent *Cluster

	LineColor Color
	FillColor Color
	// Invisible keeps the cluster in the layout but leaves it out of the
	// drawing, like Graphviz style=invis.
	Invisible bool

	// computed in layouting
	TopLeft, BottomRight Vector
}

func (cluster *Cluster) depth() int {
	d := 0
	for c := cluster; c != nil; c = c.Parent {
		d++
	}
	return d
}
