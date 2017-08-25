package layout

type Shape string

const (
	Auto    Shape = ""
	Box           = "box"
	Square        = "square"
	Circle        = "circle"
	Ellipse       = "ellipse"
)

type Vector struct{ X, Y Length }

// Length is a value represented in points
type Length float64

const (
	Point = 1
	Inch  = 72
	Twip  = Inch / 1440

	Meter      = 39.3701 * Inch
	Centimeter = Meter * 0.01
	Millimeter = Meter * 0.001
)
