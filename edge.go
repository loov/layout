package layout

// Edge connects two nodes. Path is filled in by layouting.
type Edge struct {
	Directed bool
	From, To *Node
	Weight   float64
	// MinLen is the minimum number of ranks the edge must span in a
	// hierarchical layout; 0 uses the default of 1.
	MinLen int

	// FromPort and ToPort pin the edge ends to compass points on the nodes
	FromPort, ToPort Compass
	// ArrowHead and ArrowTail select the markers at the To and From ends
	ArrowHead, ArrowTail Arrow

	Tooltip string

	Label     string
	FontName  string
	FontSize  Length
	FontColor Color

	LineWidth Length
	LineColor Color
	LineStyle LineStyle

	// computed in layouting
	Path        []Vector
	LabelPos    Vector // center of the label, when Label is set
	LabelRadius Vector // half size of the label
}

// NewEdge creates an edge from one node to another with default styling.
func NewEdge(from, to *Node) *Edge {
	edge := &Edge{}
	edge.From = from
	edge.To = to
	edge.Weight = 1.0
	edge.LineWidth = Point
	return edge
}

// String returns the edge as "from->to".
func (edge *Edge) String() string {
	return edge.From.String() + "->" + edge.To.String()
}
