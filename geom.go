package layout

import (
	"strconv"
)

// Shape is the outline drawn for a node.
type Shape string

// Node shapes. Auto uses the graph default.
const (
	Auto    Shape = ""
	None    Shape = "none"
	Box     Shape = "box"
	Square  Shape = "square"
	Circle  Shape = "circle"
	Ellipse Shape = "ellipse"
	// Record draws the label as a table of fields, like Graphviz record:
	// "a|{b|c}|<port> d".
	Record Shape = "record"
	// PointShape is a small filled circle without a label, like Graphviz point;
	// it marks where edges start or meet, such as an automaton's start.
	PointShape Shape = "point"
	// Diamond and Octagon are the polygons of Graphviz diamond and octagon,
	// sized to hold the label inside the slanted sides.
	Diamond Shape = "diamond"
	Octagon Shape = "octagon"
)

// octagonCut is where the slanted sides of an octagon meet its straight
// ones, as a part of its half size from the middle: tan 22.5°, which makes
// a square octagon regular
const octagonCut = 0.41421356

// polygon returns the corners of the outline of a polygon shape, as parts
// of its half size from the middle, clockwise from the top; nil for the
// other shapes
func (shape Shape) polygon() []Vector {
	switch shape {
	case Diamond:
		return []Vector{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}
	case Octagon:
		const c = octagonCut
		return []Vector{{-c, -1}, {c, -1}, {1, -c}, {1, c}, {c, 1}, {-c, 1}, {-1, c}, {-1, -c}}
	}
	return nil
}

// sides calls side with the outward normal n and offset h of every side
// of a polygon from shape.polygon, so that a point v, as parts of the half
// size, is inside when n·v <= h for every side
func sides(corners []Vector, side func(nx, ny, h float64)) {
	for i, a := range corners {
		b := corners[(i+1)%len(corners)]
		nx, ny := float64(b.Y-a.Y), float64(a.X-b.X)
		h := nx*float64(a.X) + ny*float64(a.Y)
		if h < 0 {
			nx, ny, h = -nx, -ny, -h
		}
		side(nx, ny, h)
	}
}

// Splines selects how edge paths are drawn.
type Splines string

// Spline styles; the zero value draws rounded polylines.
const (
	SplinesRounded  Splines = ""
	SplinesPolyline Splines = "polyline" // straight segments through the path points
	SplinesLine     Splines = "line"     // one straight segment from node to node
	SplinesOrtho    Splines = "ortho"    // horizontal and vertical segments only
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

// LineStyle selects how outlines and edges are stroked.
type LineStyle string

// Line styles; the zero value is a solid line.
const (
	Solid  LineStyle = ""
	Dashed LineStyle = "dashed"
	Dotted LineStyle = "dotted"
	Bold   LineStyle = "bold"
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
// heads and nothing otherwise. Writers draw other values, such as
// Graphviz names without a constant here, as a normal arrowhead.
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

// Sub returns v - o.
func (v Vector) Sub(o Vector) Vector { return Vector{X: v.X - o.X, Y: v.Y - o.Y} }

// Length is a distance in points.
type Length float64

// String formats the length with float32 precision: plenty for drawing,
// and it hides cross-architecture rounding noise in the last bits.
func (l Length) String() string { return strconv.FormatFloat(float64(l), 'f', -1, 32) }

// Units of Length.
const (
	Point = 1
	Inch  = 72
	Twip  = Inch / 1440.0

	Meter      = 39.3701 * Inch
	Centimeter = Meter * 0.01
	Millimeter = Meter * 0.001
)
