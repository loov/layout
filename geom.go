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

// Compass is a point on a node's outline where an edge attaches.
type Compass string

// Compass points; the zero value lets the layout choose.
const (
	CompassAuto Compass = ""
	North       Compass = "n"
	NorthEast   Compass = "ne"
	East        Compass = "e"
	SouthEast   Compass = "se"
	South       Compass = "s"
	SouthWest   Compass = "sw"
	West        Compass = "w"
	NorthWest   Compass = "nw"
	Center      Compass = "c"
)

// Arrow is the marker drawn at an edge end.
type Arrow string

// Arrow styles; the zero value is a normal arrowhead for directed edges'
// heads and nothing otherwise.
const (
	ArrowDefault Arrow = ""
	ArrowNormal  Arrow = "normal"
	ArrowNone    Arrow = "none"
	ArrowDot     Arrow = "dot"
	ArrowODot    Arrow = "odot"
	ArrowVee     Arrow = "vee"
)

// Vector is a point or size in the plane.
type Vector struct{ X, Y Length }

// Add returns the component-wise sum of v and o.
func (v Vector) Add(o Vector) Vector { return Vector{X: v.X + o.X, Y: v.Y + o.Y} }

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
