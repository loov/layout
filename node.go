package layout

type Node struct {
	ID string

	Label  string
	Weight float64

	Tooltip   string
	FontSize  Length
	FontColor Color

	PenWidth Length
	PenColor Color

	Shape     Shape
	FillColor Color
	Size      Vector

	// computed in layouting
	Center Vector
}

func (node *Node) String() string {
	if node == nil {
		return "?"
	}
	return node.ID
}
