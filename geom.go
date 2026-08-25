package layout

// Shape is the outline drawn for a node.
type Shape string

// Node shapes. Auto uses the graph default.
const (
	Auto    Shape = ""
	None          = "none"
	Box           = "box"
	Square        = "square"
	Circle        = "circle"
	Ellipse       = "ellipse"
)

// RankDir is the direction in which ranks progress.
type RankDir string

// Rank directions; the zero value is TopToBottom.
const (
	TopToBottom RankDir = ""
	LeftToRight RankDir = "LR"
	BottomToTop RankDir = "BT"
	RightToLeft RankDir = "RL"
)

// Vector is a point or size in the plane.
type Vector struct{ X, Y Length }

// Length is a distance in points.
type Length float64

// Units of Length.
const (
	Point = 1
	Inch  = 72
	Twip  = Inch / 1440

	Meter      = 39.3701 * Inch
	Centimeter = Meter * 0.01
	Millimeter = Meter * 0.001
)
