package draw

import "strings"

// Record is a field of a record shaped node: either text or a row/column
// of sub-fields. The bounds X0, Y0 to X1, Y1 are relative to the node's
// top left corner and filled in by LayoutRecord.
type Record struct {
	Text     string
	Fields   []*Record
	Vertical bool // sub-fields are stacked top to bottom

	X0, Y0, X1, Y1 float64
}

// ParseRecord parses a record label such as "a|{b|c}|<p> d": fields are
// separated by |, braces flip the stacking direction, <port> prefixes are
// dropped and \l \r \n line breaks become newlines. The fields stack top
// to bottom when vertical is set, as they do in graphs laid out sideways.
func ParseRecord(label string, vertical bool) *Record {
	rec, _ := parseRecord(label, vertical)
	return rec
}

func parseRecord(s string, vertical bool) (*Record, string) {
	rec := &Record{Vertical: vertical}
	var text strings.Builder
	grouped := false // the pending field is a brace group, not text
	flush := func() {
		if !grouped {
			rec.Fields = append(rec.Fields, &Record{Text: cleanRecordText(text.String())})
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

// LayoutRecord parses a record label, see ParseRecord, and computes the field boxes for a
// node of the given size. Fields get their natural size, with any extra
// room shared equally. The text is measured as by TextSize.
func LayoutRecord(label string, vertical bool, width, height, lineHeight, fontSize float64, lineWidth func(line string) float64) *Record {
	rec := ParseRecord(label, vertical)
	measureRecord(rec, lineHeight, fontSize, lineWidth)
	placeRecord(rec, 0, 0, width, height)
	return rec
}

// RecordSize returns the natural size of a record label, measuring the
// text as by TextSize.
func RecordSize(label string, vertical bool, lineHeight, fontSize float64, lineWidth func(line string) float64) (w, h float64) {
	return measureRecord(ParseRecord(label, vertical), lineHeight, fontSize, lineWidth)
}

// measureRecord computes the natural size of each field, storing it in
// X1, Y1 (relative to a zero X0, Y0), and returns the total.
func measureRecord(rec *Record, lineHeight, fontSize float64, lineWidth func(line string) float64) (w, h float64) {
	pad := fontSize * 0.5
	if len(rec.Fields) == 0 {
		w, h := TextSize(rec.Text, lineHeight, fontSize, lineWidth)
		rec.X1, rec.Y1 = w+2*pad, h+pad
		return rec.X1, rec.Y1
	}
	for _, field := range rec.Fields {
		fw, fh := measureRecord(field, lineHeight, fontSize, lineWidth)
		if rec.Vertical {
			w, h = max(w, fw), h+fh
		} else {
			w, h = w+fw, max(h, fh)
		}
	}
	rec.X1, rec.Y1 = w, h
	return w, h
}

// placeRecord assigns final boxes, stretching fields to fill the size
func placeRecord(rec *Record, x, y, width, height float64) {
	rec.X0, rec.Y0 = x, y
	rec.X1, rec.Y1 = x+width, y+height
	if len(rec.Fields) == 0 {
		return
	}
	var natural float64
	for _, field := range rec.Fields {
		if rec.Vertical {
			natural += field.Y1
		} else {
			natural += field.X1
		}
	}
	for _, field := range rec.Fields {
		if rec.Vertical {
			h := field.Y1 + (height-natural)/float64(len(rec.Fields))
			placeRecord(field, x, y, width, h)
			y += h
		} else {
			w := field.X1 + (width-natural)/float64(len(rec.Fields))
			placeRecord(field, x, y, w, height)
			x += w
		}
	}
}
