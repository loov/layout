package draw

import "strings"

// Labels mark spans of their text with sequences that take no room, as
// PlainLabel does with the markup of HTML-like labels, and the writers
// draw them as they can. A sequence is DLE, a control character that
// labels have no use for, and a letter:
//
//	DLE b bold, DLE i italic, DLE u underline, DLE s struck through
//	DLE c color DLE  a color, by name or #RRGGBB, see layout.ParseColor
//	DLE 0            back to plain
//
// Every line starts plain, and a span holds its style until the next
// sequence; see Spans.
const (
	esc      = "\x10"
	styleEnd = esc + "0"
)

// Style is how a span of text is drawn
type Style struct {
	Bold, Italic, Underline, Strike bool
	// Color is a color name or #RRGGBB, empty for the default
	Color string
}

// Span is text drawn in one style
type Span struct {
	Text  string
	Style Style
}

// Spans splits a line of a label into the spans of its styles, see Style
func Spans(line string) []Span {
	spans, _ := spansFrom(line, Style{})
	return spans
}

// spansFrom splits line into the spans of its styles, starting in style,
// and returns the style it ends in
func spansFrom(line string, style Style) ([]Span, Style) {
	var spans []Span
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			spans = append(spans, Span{text.String(), style})
			text.Reset()
		}
	}
	for len(line) > 0 {
		at := strings.Index(line, esc)
		if at < 0 || at+1 == len(line) {
			text.WriteString(line)
			break
		}
		text.WriteString(line[:at])
		flush()
		code, rest := line[at+1], line[at+2:]
		switch code {
		case '0':
			style = Style{}
		case 'b':
			style.Bold = true
		case 'i':
			style.Italic = true
		case 'u':
			style.Underline = true
		case 's':
			style.Strike = true
		case 'c':
			if end := strings.Index(rest, esc); end >= 0 {
				style.Color, rest = rest[:end], rest[end+len(esc):]
			} else {
				style.Color, rest = rest, ""
			}
		}
		line = rest
	}
	flush()
	return spans, style
}

// StripStyle returns a line without the sequences of its styles
func StripStyle(line string) string {
	if !strings.Contains(line, esc) {
		return line
	}
	var out strings.Builder
	for _, span := range Spans(line) {
		out.WriteString(span.Text)
	}
	return out.String()
}

// open returns the sequences that start text in style
func (style Style) open() string {
	var out strings.Builder
	for _, on := range []struct {
		set  bool
		code string
	}{{style.Bold, "b"}, {style.Italic, "i"}, {style.Underline, "u"}, {style.Strike, "s"}} {
		if on.set {
			out.WriteString(esc + on.code)
		}
	}
	if style.Color != "" {
		out.WriteString(esc + "c" + style.Color + esc)
	}
	return out.String()
}

// styledLine returns spans as a line of a label, each opening its style
// and closing it after
func styledLine(spans []Span) string {
	var out strings.Builder
	for _, span := range spans {
		if open := span.Style.open(); open != "" {
			out.WriteString(open + span.Text + styleEnd)
		} else {
			out.WriteString(span.Text)
		}
	}
	return out.String()
}

// collapseSpaces drops the whitespace of markup from spans, as a browser
// shows them: a run of it is one space, in the span it starts in, and
// none starts or ends the line
func collapseSpaces(spans []Span) []Span {
	var out []Span
	space := true // the last character kept is a space, or there is none
	for _, span := range spans {
		var text strings.Builder
		for _, r := range span.Text {
			isSpace := r == ' ' || r == '\t'
			if isSpace && space {
				continue
			}
			space = isSpace
			if isSpace {
				r = ' '
			}
			text.WriteRune(r)
		}
		if text.Len() > 0 {
			out = append(out, Span{text.String(), span.Style})
		}
	}
	if n := len(out); n > 0 {
		if out[n-1].Text = strings.TrimSuffix(out[n-1].Text, " "); out[n-1].Text == "" {
			out = out[:n-1]
		}
	}
	return out
}
