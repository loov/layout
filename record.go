package layout

import "strings"

// RecordField is a field of a record shaped node: either text or a row/column
// of sub-fields. Bounds are relative to the node's top left corner and
// filled in by LayoutRecord.
type RecordField struct {
	Text     string
	Fields   []*RecordField
	Vertical bool // sub-fields are stacked top to bottom

	TopLeft, BottomRight Vector
}

// ParseRecord parses a record label such as "a|{b|c}|<p> d": fields are
// separated by |, braces flip the stacking direction, <port> prefixes are
// dropped and \l \r \n line breaks become newlines.
func ParseRecord(label string) *RecordField {
	rec, _ := parseRecord(label, false)
	return rec
}

func parseRecord(s string, vertical bool) (*RecordField, string) {
	rec := &RecordField{Vertical: vertical}
	var text strings.Builder
	grouped := false // the pending field is a brace group, not text
	flush := func() {
		if !grouped {
			rec.Fields = append(rec.Fields, &RecordField{Text: cleanRecordText(text.String())})
		}
		text.Reset()
		grouped = false
	}
	for len(s) > 0 {
		c := s[0]
		switch {
		case c == '\\' && len(s) > 1:
			if strings.IndexByte("nlr", s[1]) >= 0 {
				text.WriteByte('\n')
			} else {
				text.WriteByte(s[1])
			}
			s = s[2:]
			continue
		case c == '|':
			flush()
		case c == '{':
			sub, rest := parseRecord(s[1:], !vertical)
			// a brace group replaces the pending text field
			if strings.TrimSpace(text.String()) != "" {
				flush()
			} else {
				text.Reset()
			}
			rec.Fields = append(rec.Fields, sub)
			grouped = true
			s = rest
			// text after a group before the next | is ignored
			for len(s) > 0 && s[0] != '|' && s[0] != '}' {
				s = s[1:]
			}
			continue
		case c == '}':
			flush()
			return rec, s[1:]
		default:
			text.WriteByte(c)
		}
		s = s[1:]
	}
	flush()
	return rec, ""
}

// cleanRecordText strips a <port> prefix; parseRecord already turned
// escapes into the characters they stand for
func cleanRecordText(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "<") {
		if end := strings.Index(s, ">"); end >= 0 {
			s = strings.TrimSpace(s[end+1:])
		}
	}
	return s
}

// LayoutRecord parses the node's record label and computes the field
// boxes for the node's current size. Fields get their natural size, with
// any extra room shared equally.
func (graph *Graph) LayoutRecord(node *Node) *RecordField {
	rec := ParseRecord(node.DefaultLabel())
	graph.measureRecord(rec, node.FontName, node.FontSize)
	graph.placeRecord(rec, Vector{}, Vector{2 * node.Radius.X, 2 * node.Radius.Y})
	return rec
}

// recordRadius returns the natural half size of a record label
func (graph *Graph) recordRadius(node *Node) Vector {
	rec := ParseRecord(node.DefaultLabel())
	size := graph.measureRecord(rec, node.FontName, node.FontSize)
	return Vector{size.X / 2, size.Y / 2}
}

// measureRecord computes the natural size of each field, storing it in
// BottomRight (relative to a zero TopLeft), and returns the total.
func (graph *Graph) measureRecord(rec *RecordField, fontName string, fontSize Length) Vector {
	pad := fontSize * 0.5
	if len(rec.Fields) == 0 {
		r := graph.textRadius(rec.Text, fontName, fontSize)
		rec.BottomRight = Vector{2*r.X + 2*pad, 2*r.Y + pad}
		return rec.BottomRight
	}
	var total Vector
	for _, field := range rec.Fields {
		size := graph.measureRecord(field, fontName, fontSize)
		if rec.Vertical {
			total.X = max(total.X, size.X)
			total.Y += size.Y
		} else {
			total.X += size.X
			total.Y = max(total.Y, size.Y)
		}
	}
	rec.BottomRight = total
	return total
}

// placeRecord assigns final boxes, stretching fields to fill size
func (graph *Graph) placeRecord(rec *RecordField, topLeft, size Vector) {
	rec.TopLeft = topLeft
	rec.BottomRight = topLeft.Add(size)
	if len(rec.Fields) == 0 {
		return
	}
	var natural Length
	for _, field := range rec.Fields {
		if rec.Vertical {
			natural += field.BottomRight.Y
		} else {
			natural += field.BottomRight.X
		}
	}
	offset := topLeft
	for _, field := range rec.Fields {
		fieldSize := size
		if rec.Vertical {
			extra := (size.Y - natural) / Length(len(rec.Fields))
			fieldSize.Y = field.BottomRight.Y + extra
		} else {
			extra := (size.X - natural) / Length(len(rec.Fields))
			fieldSize.X = field.BottomRight.X + extra
		}
		graph.placeRecord(field, offset, fieldSize)
		if rec.Vertical {
			offset.Y += fieldSize.Y
		} else {
			offset.X += fieldSize.X
		}
	}
}
