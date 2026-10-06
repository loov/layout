package draw

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

// PlainLabel returns a label as text draws it: an HTML-like label without its
// markup, with a line per <br> and per table row and cells apart. A line
// goes where the <br> that ends it says, or else where its cell does, and
// a row where its first cell does, see Lines.
func PlainLabel(label string) string {
	nodes, ok := parseHTMLLabel(label)
	if !ok {
		return label
	}
	var out strings.Builder
	for _, n := range nodes {
		plainText(&out, n, "", Style{})
	}
	return collapse(out.String())
}

// TableRecord returns an HTML-like label that is a table with borders as
// a record label, see ParseRecord, which text draws with the borders as
// dividers: the rows stack and the cells of a row are side by side; a
// cell holds its lines, or the fields of a table in it with borders of
// its own. The fields stack top to bottom at the top level when vertical
// is set, as ParseRecord is told. It reports false for other labels.
// Spans of rows and columns are not kept.
func TableRecord(label string, vertical bool) (string, bool) {
	nodes, ok := parseHTMLLabel(label)
	if !ok {
		return "", false
	}
	var table *html.Node
	for _, n := range nodes {
		if n.Type == html.TextNode && strings.TrimSpace(n.Data) == "" {
			continue
		}
		if table != nil || n.Type != html.ElementNode || n.Data != "table" {
			return "", false
		}
		table = n
	}
	if table == nil {
		return "", false
	}
	border, cells := borders(table)
	switch {
	case cells > 0:
		// the rows go top to bottom: within braces at the top level,
		// unless it stacks that way already
		rows := tableFields(table)
		if vertical {
			return rows, true
		}
		return "{" + rows + "}", true
	case border > 0:
		// a frame alone: one field of all the lines
		var out strings.Builder
		plainText(&out, table, "", Style{})
		return escapeRecord(collapse(out.String())), true
	}
	return "", false
}

// tableFields returns the rows of a table with cell borders as record
// fields stacked across the record they are in, with the cells of each
// side by side, braced to stack the other way
func tableFields(table *html.Node) string {
	var rows []string
	open := false // the last row is one cell without a border below
	for _, tr := range rowsOf(table) {
		var cells []string
		var only *html.Node // the cell of a row of one
		for td := tr.FirstChild; td != nil; td = td.NextSibling {
			if td.Type != html.ElementNode || td.Data != "td" && td.Data != "th" {
				continue
			}
			if inner := onlyTable(td); inner != nil {
				if _, c := borders(inner); c > 0 {
					cells = append(cells, "{"+tableFields(inner)+"}")
					continue
				}
			}
			var out strings.Builder
			plainText(&out, td, "", Style{})
			cells = append(cells, escapeRecord(collapse(out.String())))
			only = td
		}
		switch {
		case len(cells) != 1:
			rows = append(rows, "{"+strings.Join(cells, "|")+"}")
			only = nil
		case open && !side(only, 't'):
			// no border between the cells of this row and the last:
			// one field
			rows[len(rows)-1] += "\n" + cells[0]
		default:
			rows = append(rows, cells[0])
		}
		open = only != nil && !side(only, 'b')
	}
	return strings.Join(rows, "|")
}

// side reports whether a cell has a border on the side, given as l, t, r
// or b, as its sides attribute says, all of them when it doesn't
func side(td *html.Node, side byte) bool {
	sides := attr(td, "sides")
	return sides == "" || strings.IndexByte(sides, side) >= 0
}

// rowsOf returns the rows of a table, also those in its body
func rowsOf(table *html.Node) []*html.Node {
	var rows []*html.Node
	for n := table.FirstChild; n != nil; n = n.NextSibling {
		switch {
		case n.Type != html.ElementNode:
		case n.Data == "tr":
			rows = append(rows, n)
		case n.Data == "tbody" || n.Data == "thead" || n.Data == "tfoot":
			rows = append(rows, rowsOf(n)...)
		}
	}
	return rows
}

// onlyTable returns the table that is all of a cell, if there is one
func onlyTable(td *html.Node) *html.Node {
	var table *html.Node
	for n := td.FirstChild; n != nil; n = n.NextSibling {
		switch {
		case n.Type == html.TextNode && strings.TrimSpace(n.Data) == "":
		case n.Type == html.ElementNode && n.Data == "table" && table == nil:
			table = n
		default:
			return nil
		}
	}
	return table
}

// borders returns the border of a table and of its cells, which is the
// table's when it doesn't say, as in Graphviz, where a table has one
func borders(table *html.Node) (border, cells int) {
	border = 1
	if v, err := strconv.Atoi(attr(table, "border")); err == nil {
		border = v
	}
	cells = border
	if v, err := strconv.Atoi(attr(table, "cellborder")); err == nil {
		cells = v
	}
	return border, cells
}

// parseHTMLLabel parses the markup of an HTML-like label
func parseHTMLLabel(label string) ([]*html.Node, bool) {
	if !IsHTMLLabel(label) {
		return nil, false
	}
	nodes, err := html.ParseFragment(strings.NewReader(label[1:len(label)-1]), &html.Node{Type: html.ElementNode})
	return nodes, err == nil
}

// plainText writes the text of n, see PlainLabel; cell is the mark of
// where the lines of the cell n is in go, and style the style of the
// text around n, see Style
func plainText(out *strings.Builder, n *html.Node, cell string, style Style) {
	switch {
	case n.Type == html.TextNode:
		// line breaks in the markup are spaces, like in a browser
		out.WriteString(strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return ' '
			}
			return r
		}, n.Data))
		return
	case n.Type != html.ElementNode:
	case n.Data == "br":
		out.WriteString(alignMark(n, cell) + "\n")
		return
	case n.Data == "td" || n.Data == "th":
		out.WriteByte(' ')
		cell = alignMark(n, "")
	case n.Data == "tr":
		row, first := "", true
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") && first {
				row, first = alignMark(c, ""), false
			}
			plainText(out, c, cell, style)
		}
		out.WriteString(row + "\n")
		return
	case n.Data == "b" || n.Data == "i" || n.Data == "u" || n.Data == "s" || n.Data == "font":
		inner := style
		switch n.Data {
		case "b":
			inner.Bold = true
		case "i":
			inner.Italic = true
		case "u":
			inner.Underline = true
		case "s":
			inner.Strike = true
		case "font":
			if color := attr(n, "color"); color != "" {
				inner.Color = color
			}
		}
		out.WriteString(styleEnd + inner.open())
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			plainText(out, c, cell, inner)
		}
		out.WriteString(styleEnd + style.open())
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		plainText(out, c, cell, style)
	}
}

// alignMark returns the mark of where the align attribute of n puts its
// lines, or else def
func alignMark(n *html.Node, def string) string {
	switch attr(n, "align") {
	case "left":
		return LeftMark
	case "right":
		return RightMark
	case "center":
		return ""
	}
	return def
}

// collapse drops the whitespace of markup from lines, and the lines it
// leaves empty, as a browser shows them; a style that goes on past the
// end of a line starts the next again, see Style
func collapse(text string) string {
	var kept []string
	var style Style
	for _, line := range Lines(text) {
		var spans []Span
		spans, style = spansFrom(line.Text, style)
		if spans = collapseSpaces(spans); len(spans) > 0 {
			kept = append(kept, styledLine(spans)+line.Align.Mark())
		}
	}
	return strings.Join(kept, "\n")
}

// escapeRecord escapes the characters that record labels give meaning
func escapeRecord(text string) string {
	return strings.NewReplacer(`\`, `\\`, `{`, `\{`, `}`, `\}`, `|`, `\|`, `<`, `\<`, `>`, `\>`).Replace(text)
}

// attr returns the value of the attribute key of n, in lower case
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return strings.ToLower(a.Val)
		}
	}
	return ""
}
