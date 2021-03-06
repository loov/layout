package layout

import (
	"strings"
)

type Node struct {
	ID string

	Label  string
	Weight float64

	Tooltip   string
	FontName  string
	FontSize  Length
	FontColor Color

	LineWidth Length
	LineColor Color

	Shape     Shape
	FillColor Color
	Radius    Vector

	// computed in layouting
	Center Vector
}

func NewNode(id string) *Node {
	node := &Node{}
	node.ID = id
	node.Weight = 1.0
	node.LineWidth = Point
	return node
}

func (node *Node) String() string {
	if node == nil {
		return "?"
	}
	if node.ID != "" {
		return node.ID
	}
	return node.Label
}

func (node *Node) DefaultLabel() string {
	if node.Label != "" {
		return node.Label
	}
	return node.ID
}

func (node *Node) approxLabelRadius(lineHeight Length) Vector {
	const HeightWidthRatio = 0.5
	if lineHeight < node.FontSize {
		lineHeight = node.FontSize
	}

	size := Vector{}
	lines := strings.Split(node.DefaultLabel(), "\n")
	for _, line := range lines {
		width := Length(len(line)) * node.FontSize * HeightWidthRatio
		if width > size.X {
			size.X = width
		}
		size.Y += lineHeight
	}

	size.X *= 0.5
	size.Y = Length(len(lines)) * lineHeight * 0.5
	return size
}

func (node *Node) TopLeft() Vector     { return Vector{node.Left(), node.Top()} }
func (node *Node) BottomRight() Vector { return Vector{node.Right(), node.Bottom()} }

func (node *Node) TopCenter() Vector    { return Vector{node.Center.X, node.Top()} }
func (node *Node) BottomCenter() Vector { return Vector{node.Center.X, node.Bottom()} }

func (node *Node) Left() Length   { return node.Center.X - node.Radius.X }
func (node *Node) Top() Length    { return node.Center.Y - node.Radius.Y }
func (node *Node) Right() Length  { return node.Center.X + node.Radius.X }
func (node *Node) Bottom() Length { return node.Center.Y + node.Radius.Y }
