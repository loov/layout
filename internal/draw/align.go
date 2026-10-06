package draw

import "strings"

// Align is where a line of a label goes across its box
type Align int8

// The alignments of a line; the zero value centers it
const (
	Center Align = iota
	Left
	Right
)

// LeftMark and RightMark end a line of a label that goes against the left
// or the right side of its box, as \l and \r end one in Graphviz; a line
// without one is centered. They are control characters, which take no
// room, see Columns.
const (
	LeftMark  = "\x01"
	RightMark = "\x02"
)

// Line is a line of a label, without its mark, and where it goes
type Line struct {
	Text  string
	Align Align
}

// Lines splits a label into its lines and where each goes
func Lines(label string) []Line {
	var lines []Line
	for text := range strings.SplitSeq(label, "\n") {
		line := Line{Text: text}
		switch {
		case strings.HasSuffix(text, LeftMark):
			line = Line{strings.TrimSuffix(text, LeftMark), Left}
		case strings.HasSuffix(text, RightMark):
			line = Line{strings.TrimSuffix(text, RightMark), Right}
		}
		lines = append(lines, line)
	}
	return lines
}

// Mark returns the mark that ends a line that goes there, see Lines
func (align Align) Mark() string {
	switch align {
	case Left:
		return LeftMark
	case Right:
		return RightMark
	}
	return ""
}

// Offset returns how far from the start of room a line w wide goes
func (align Align) Offset(room, w float64) float64 {
	switch align {
	case Left:
		return 0
	case Right:
		return room - w
	}
	return (room - w) / 2
}
