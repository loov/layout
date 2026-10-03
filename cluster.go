package layout

// Cluster is a group of nodes drawn inside a common box. Layout leaves it
// unchanged; the computed box is in Layout.Clusters.
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
}
