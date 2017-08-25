package layout

type Edge struct {
	Directed bool
	From, To *Node
	Weight   float64

	Tooltip string

	Label     string
	FontSize  Length
	FontColor Color

	PenWidth Length
	PenColor Color

	// computed in layouting
	Path []Vector
}

func (edge *Edge) String() string {
	return edge.From.String() + "->" + edge.To.String()
}
