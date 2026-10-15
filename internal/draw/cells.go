package draw

import (
	"strings"
	"unicode"
)

// Columns returns the columns s takes in a terminal: two for wide
// characters, none for marks on the character before, for control
// characters and for the sequences of styles, which are not drawn, see
// Style
func Columns(s string) int {
	n := 0
	for _, r := range StripStyle(s) {
		switch {
		case IsZeroWidth(r), unicode.IsControl(r):
		case IsWide(r):
			n += 2
		default:
			n++
		}
	}

	return n
}

// TextColumns returns the columns of the widest line of s
func TextColumns(s string) int {
	w := 0
	for _, line := range strings.Split(s, "\n") {
		w = max(w, Columns(line))
	}
	return w
}

// LabelBox returns the size of the box text draws plain text in, as the
// columns and rows from one border to the other: the text with a space
// on either side, between borders on the rows above and below it
func LabelBox(text string) (w, h int) {
	return TextColumns(text) + 3, strings.Count(text, "\n") + 2
}

// PlainBox returns the size of the box text draws a label without an
// outline in, as LabelBox does: the text between blank borders where
// edges end, without the spaces beside it
func PlainBox(text string) (w, h int) {
	return TextColumns(text) + 1, strings.Count(text, "\n") + 2
}

// RecordRows returns the rows the fields of a record need inside its
// box: a row per line of text, and one per divider between fields
// stacked vertically
func RecordRows(rec *Record) int {
	if rec.Leaf() {
		return strings.Count(rec.Text, "\n") + 1
	}
	rows := 0
	for _, field := range rec.Fields {
		if rec.Vertical {
			rows += RecordRows(field)
		} else {
			rows = max(rows, RecordRows(field))
		}
	}

	if rec.Vertical {
		rows += len(rec.Fields) - 1
	}
	return rows
}

// DiamondSteps returns the size k of the diamond that text draws around a
// label, see format/text: 2k+1 rows of 4k-1 columns, with its points in
// the middles of the top and bottom rows and of the sides of row k. Row r
// of the top half is 4r-3 columns wide inside, mirrored below, and the
// lines of the label go on the middle rows, each a column in from the
// sides. Without a label text draws the one of CornerDiamondSteps.
func DiamondSteps(label string) int {
	if label == "" {
		return 1
	}
	lines := Lines(label)
	inside := func(k, r int) int { return 4*min(r, 2*k-r) - 3 }
	for k := 1; ; k++ {
		top := k - (len(lines)-1)/2
		fits := top >= 1
		for i, line := range lines {
			fits = fits && Columns(line.Text)+2 <= inside(k, top+i)
		}
		if fits {
			return k
		}
	}
}

// CornerDiamondSteps returns the size k of the diamond that text draws
// around a label with its points on the corners of cells, see format/text,
// for more edges at a point than DiamondSteps: 2k rows of 4k columns, row
// r of the top half 4r columns wide inside, mirrored below, the lines of
// the label on the middle rows, each a column in from the sides
func CornerDiamondSteps(label string) int {
	if label == "" {
		return 1
	}
	lines := Lines(label)
	inside := func(k, r int) int { return 4 * min(r, 2*k-1-r) }
	for k := 1; ; k++ {
		top := k - (len(lines)+1)/2
		fits := top >= 1
		for i, line := range lines {
			fits = fits && Columns(line.Text)+2 <= inside(k, top+i)
		}
		if fits {
			return k
		}
	}
}

// StepDiamondSteps returns the size k of the diamond that text draws
// around a label without the diagonals of Symbols for Legacy Computing,
// see format/text: 2k+2 rows of 4k+2 columns, with steps of ▁╱ and ╲▔,
// where row r of the top half is 4r columns wide inside, mirrored below,
// and the lines of the label go on the middle rows, each a column in from
// the sides. Without a label it is the small one of 4 columns and 2 rows,
// as the one CornerDiamondSteps sizes.
func StepDiamondSteps(label string) int {
	if label == "" {
		return 1
	}
	lines := Lines(label)
	inside := func(k, r int) int { return 4 * min(r, 2*k+1-r) }
	for k := 1; ; k++ {
		top := k - (len(lines)-1)/2
		fits := top >= 1
		for i, line := range lines {
			fits = fits && Columns(line.Text)+2 <= inside(k, top+i)
		}
		if fits {
			return k
		}
	}
}

// DiamondSize returns the columns and rows that any diamond that text
// draws around a label fits in, see DiamondSteps and CornerDiamondSteps,
// so that a layout leaves room for whichever is drawn. The one of
// StepDiamondSteps fits in the cells of CornerDiamondSteps: a step less,
// with as much room inside on the rows of the label.
func DiamondSize(label string) (cols, rows int) {
	k, c := DiamondSteps(label), CornerDiamondSteps(label)
	if label == "" {
		return 4 * c, 2 * c // no pointed one without a label
	}
	return max(4*k-1, 4*c), max(2*k+1, 2*c)
}
