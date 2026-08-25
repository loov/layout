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
