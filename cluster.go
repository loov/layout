package layout

// Cluster is a group of nodes drawn inside a common box. The box corners
// are computed by layouting.
type Cluster struct {
	ID    string
	Label string
	Nodes []*Node

	LineColor Color
	FillColor Color

	// computed in layouting
	TopLeft, BottomRight Vector
}
