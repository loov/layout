package layout

type Edge struct {
	Directed bool
	From, To *Node
	Weight   float64

	Tooltip string

	Label     string
	FontSize  Length
	FontColor Color

	LineWidth Length
	LineColor Color

	// computed in layouting
	Path []Vector
}

func NewEdge(from, to *Node) *Edge {
	edge := &Edge{}
	edge.From = from
	edge.To = to
	edge.Weight = 1.0
	edge.LineWidth = Point
	return edge
}

func (edge *Edge) String() string {
	return edge.From.String() + "->" + edge.To.String()
}
