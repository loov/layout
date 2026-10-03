package draw

import (
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

// Columns returns the columns s takes in a terminal: two for wide
// characters, none for marks on the character before and for control
// characters, which are not drawn
func Columns(s string) int {
	n := 0
	for _, r := range s {
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

// PlainLabel returns a label as text draws it: an HTML-like label without its
// markup, with a line per <br> and per table row and cells apart
func PlainLabel(label string) string {
	if !IsHTMLLabel(label) {
		return label
	}
	nodes, err := html.ParseFragment(strings.NewReader(label[1:len(label)-1]), &html.Node{Type: html.ElementNode})
	if err != nil {
		return label
	}
	var out strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.TextNode:
			out.WriteString(n.Data)
		case n.Type == html.ElementNode && n.Data == "br":
			out.WriteByte('\n')
		case n.Type == html.ElementNode && (n.Data == "td" || n.Data == "th"):
			out.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && n.Data == "tr" {
			out.WriteByte('\n')
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	// markup whitespace doesn't show, like in a browser
	lines := strings.Split(out.String(), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}
