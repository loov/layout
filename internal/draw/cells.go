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

// RecordRows returns the rows the fields of a record need inside its
// box: a row per line of text, and one per divider between fields
// stacked vertically
func RecordRows(rec *Record) int {
	if len(rec.Fields) == 0 {
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
