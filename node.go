package layout

import (
	"math"
	"strings"
)

// Node is a vertex in a graph. Radius is half the node size; Center is
// filled in by layouting.
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

// NewNode creates a node with the given id and default styling.
func NewNode(id string) *Node {
	node := &Node{}
	node.ID = id
	node.Weight = 1.0
	node.LineWidth = Point
	return node
}

// String returns the node id, or its label when there is no id.
func (node *Node) String() string {
	if node == nil {
		return "?"
	}
	if node.ID != "" {
		return node.ID
	}
	return node.Label
}

// DefaultLabel returns the label, falling back to the id.
func (node *Node) DefaultLabel() string {
	if node.Label != "" {
		return node.Label
	}
	return node.ID
}

// approxLabelRadius estimates the half size of the label text
// assuming a fixed height to width ratio for characters.
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

// TopLeft returns the top left corner of the node bounds.
func (node *Node) TopLeft() Vector { return Vector{node.Left(), node.Top()} }

// BottomRight returns the bottom right corner of the node bounds.
func (node *Node) BottomRight() Vector { return Vector{node.Right(), node.Bottom()} }

// TopCenter returns the middle of the top edge of the node bounds.
func (node *Node) TopCenter() Vector { return Vector{node.Center.X, node.Top()} }

// BottomCenter returns the middle of the bottom edge of the node bounds.
func (node *Node) BottomCenter() Vector { return Vector{node.Center.X, node.Bottom()} }

// Left returns the x coordinate of the left side of the node bounds.
func (node *Node) Left() Length { return node.Center.X - node.Radius.X }

// Top returns the y coordinate of the top side of the node bounds.
func (node *Node) Top() Length { return node.Center.Y - node.Radius.Y }

// Right returns the x coordinate of the right side of the node bounds.
func (node *Node) Right() Length { return node.Center.X + node.Radius.X }

// Bottom returns the y coordinate of the bottom side of the node bounds.
func (node *Node) Bottom() Length { return node.Center.Y + node.Radius.Y }

// Boundary returns the point on the node outline where the ray from the
// center towards p exits the node.
func (node *Node) Boundary(p Vector) Vector {
	dx, dy := float64(p.X-node.Center.X), float64(p.Y-node.Center.Y)
	if dx == 0 && dy == 0 {
		return node.Center
	}
	rx, ry := float64(node.Radius.X), float64(node.Radius.Y)

	var t float64
	switch node.Shape {
	case Box, Square:
		t = math.Min(rx/math.Abs(dx), ry/math.Abs(dy)) // ray-rect; Inf for zero component is fine
	default: // ellipse and circle
		t = 1 / math.Hypot(dx/rx, dy/ry)
	}
	return Vector{node.Center.X + Length(dx*t), node.Center.Y + Length(dy*t)}
}
