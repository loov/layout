package layout

import (
	"math"
	"strings"
	"unicode"
)

// Node is a vertex in a graph. Radius is half the node size; Center is
// filled in by layouting.
type Node struct {
	ID string

	Label string

	Tooltip   string
	FontName  string
	FontSize  Length
	FontColor Color

	LineWidth Length
	LineColor Color
	LineStyle LineStyle
	// Peripheries is the number of outlines; 0 and 1 draw one
	Peripheries int

	Shape     Shape
	FillColor Color
	// Radius is the minimum half size; the label can grow it unless
	// FixedSize is set
	Radius Vector
	// peripheryPad is the padding added to Radius for extra peripheries,
	// removed again before the next layout recomputes it
	peripheryPad Vector
	FixedSize    bool
	// Image is a URL drawn inside the node
	Image string

	// computed in layouting
	Center Vector
}

// NewNode creates a node with the given id and default styling.
func NewNode(id string) *Node {
	node := &Node{}
	node.ID = id
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
	if node.Shape == Dot {
		return ""
	}
	if node.Label != "" {
		return node.Label
	}
	return node.ID
}

// textRadius returns the half size of multi-line text, measuring each
// line with graph.MeasureText or the built-in approximation.
func (graph *Graph) textRadius(text string, fontName string, fontSize Length) Vector {
	lineHeight := graph.LineHeight
	if lineHeight < fontSize {
		lineHeight = fontSize
	}
	measure := graph.MeasureText
	if measure == nil {
		measure = approxTextWidth
	}

	size := Vector{}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		size.X = max(size.X, measure(line, fontName, fontSize).X)
	}
	size.Y = Length(len(lines)) * lineHeight * 0.5
	return size
}

// approxTextWidth estimates the half size of one line of proportional text
// from per-character width classes.
func approxTextWidth(line string, _ string, fontSize Length) Vector {
	width := Length(0)
	for _, r := range line {
		var em Length
		switch {
		case IsWide(r):
			em = 1
		case IsZeroWidth(r):
			em = 0
		case strings.ContainsRune("il.,:;'|!I", r):
			em = 0.28
		case strings.ContainsRune("jtfr ()[]-", r):
			em = 0.36
		case strings.ContainsRune("mwMW@", r):
			em = 0.85
		case unicode.IsUpper(r):
			em = 0.68
		default:
			em = 0.52
		}
		width += em * fontSize
	}
	return Vector{X: width / 2, Y: fontSize / 2}
}

// IsWide reports whether r is a wide character: East Asian wide and
// fullwidth characters and emoji, about an em wide in proportional fonts
// and two columns wide in terminals.
func IsWide(r rune) bool {
	for _, span := range [...][2]rune{
		{0x1100, 0x115F},   // Hangul Jamo initials
		{0x2E80, 0x303E},   // CJK radicals, symbols and punctuation
		{0x3041, 0x33FF},   // kana, Bopomofo, Hangul compatibility, CJK compatibility
		{0x3400, 0x4DBF},   // CJK extension A
		{0x4E00, 0x9FFF},   // CJK unified ideographs
		{0xA000, 0xA4CF},   // Yi
		{0xAC00, 0xD7A3},   // Hangul syllables
		{0xF900, 0xFAFF},   // CJK compatibility ideographs
		{0xFE30, 0xFE4F},   // CJK compatibility forms
		{0xFF00, 0xFF60},   // fullwidth forms
		{0xFFE0, 0xFFE6},   // fullwidth signs
		{0x1F300, 0x1F64F}, // pictographs and emoticons
		{0x1F900, 0x1F9FF}, // supplemental pictographs
		{0x20000, 0x3FFFD}, // CJK extensions B and later
	} {
		if span[0] <= r && r <= span[1] {
			return true
		}
	}
	return false
}

// IsZeroWidth reports whether r takes no room of its own: combining
// marks, joiners and variation selectors, which modify the character
// before them.
func IsZeroWidth(r rune) bool {
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf) || 0xFE00 <= r && r <= 0xFE0F
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

// CompassPoint returns the point on the node outline at the compass
// direction, or the center for Center and CompassAuto.
func (node *Node) CompassPoint(c Compass) Vector {
	var dir Vector
	switch c {
	case North:
		dir = Vector{0, -1}
	case NorthEast:
		dir = Vector{1, -1}
	case East:
		dir = Vector{1, 0}
	case SouthEast:
		dir = Vector{1, 1}
	case South:
		dir = Vector{0, 1}
	case SouthWest:
		dir = Vector{-1, 1}
	case West:
		dir = Vector{-1, 0}
	case NorthWest:
		dir = Vector{-1, -1}
	default:
		return node.Center
	}
	return node.Boundary(Vector{node.Center.X + dir.X*node.Radius.X, node.Center.Y + dir.Y*node.Radius.Y})
}

// outlineAlong returns where the line from start, inside the node,
// towards p leaves the node's outline, or the outline towards p from the
// center when start is outside.
func (node *Node) outlineAlong(start, p Vector) Vector {
	inside := func(v Vector) bool {
		dx, dy := float64(v.X-node.Center.X), float64(v.Y-node.Center.Y)
		rx, ry := float64(node.Radius.X), float64(node.Radius.Y)
		switch node.Shape {
		case Box, Square, Record:
			return math.Abs(dx) <= rx && math.Abs(dy) <= ry
		}
		return (dx/rx)*(dx/rx)+(dy/ry)*(dy/ry) <= 1
	}
	if !inside(start) || inside(p) {
		return node.Boundary(p)
	}
	in, out := start, p
	for range 32 {
		mid := Vector{(in.X + out.X) / 2, (in.Y + out.Y) / 2}
		if inside(mid) {
			in = mid
		} else {
			out = mid
		}
	}
	return in
}

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
	case Box, Square, Record:
		t = math.Min(rx/math.Abs(dx), ry/math.Abs(dy)) // ray-rect; Inf for zero component is fine
	default: // ellipse and circle
		t = 1 / math.Hypot(dx/rx, dy/ry)
	}
	return Vector{node.Center.X + Length(dx*t), node.Center.Y + Length(dy*t)}
}
