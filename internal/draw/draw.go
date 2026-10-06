// Package draw holds the measuring and geometry helpers that the layout
// and the format writers share. It works in points as float64, since it
// cannot import the layout types.
package draw

import (
	"math"
	"strings"
	"unicode"
)

// Point is a position in the plane.
type Point struct{ X, Y float64 }

// CornerRadius returns the radius to round the corner at p with: the
// given radius, limited to half of the adjacent segments and so that the
// curve stays within maxDeviation of the corner. Writers use it so that
// rounded edges stay clear of the obstacles the paths were routed around.
func CornerRadius(prev, p, next Point, radius, maxDeviation float64) float64 {
	l1, l2 := math.Hypot(p.X-prev.X, p.Y-prev.Y), math.Hypot(next.X-p.X, next.Y-p.Y)
	if l1 == 0 || l2 == 0 {
		return 0
	}
	// the quadratic curve's midpoint is r*cos(θ/2)/2 from the corner,
	// θ being the angle between the two segments
	ux, uy := (prev.X-p.X)/l1, (prev.Y-p.Y)/l1
	vx, vy := (next.X-p.X)/l2, (next.Y-p.Y)/l2
	cosHalf := math.Hypot(ux+vx, uy+vy) / 2
	r := math.Min(radius, math.Min(l1, l2)/2)
	if cosHalf > 0 {
		r = math.Min(r, 2*maxDeviation/cosHalf)
	}
	return r
}

// IsHTMLLabel reports whether label is an HTML-like label, "<...>".
func IsHTMLLabel(label string) bool {
	return len(label) >= 2 && label[0] == '<' && label[len(label)-1] == '>'
}

// IsWide reports whether r is a wide character: East Asian wide and
// fullwidth characters and emoji, about an em wide in proportional fonts
// and two columns wide in terminals.
func IsWide(r rune) bool {
	for _, span := range [...][2]rune{
		{0x1100, 0x115F},   // Hangul Jamo initials
		{0x2E80, 0x303E},   // CJK radicals, symbols and punctuation
		{0x3041, 0x33FF},   // kana, Bopomofo, Hangul compatibility, CJK compatibility
		{0x3400, 0x4DBF},   // CJK extension A
		{0x4E00, 0x9FFF},   // CJK unified ideographs
		{0xA000, 0xA4CF},   // Yi
		{0xAC00, 0xD7A3},   // Hangul syllables
		{0xF900, 0xFAFF},   // CJK compatibility ideographs
		{0xFE30, 0xFE4F},   // CJK compatibility forms
		{0xFF00, 0xFF60},   // fullwidth forms
		{0xFFE0, 0xFFE6},   // fullwidth signs
		{0x1F300, 0x1F64F}, // pictographs and emoticons
		{0x1F900, 0x1F9FF}, // supplemental pictographs
		{0x20000, 0x3FFFD}, // CJK extensions B and later
	} {
		if span[0] <= r && r <= span[1] {
			return true
		}
	}
	return false
}

// IsZeroWidth reports whether r takes no room of its own: combining
// marks, joiners and variation selectors, which modify the character
// before them.
func IsZeroWidth(r rune) bool {
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf) || 0xFE00 <= r && r <= 0xFE0F
}

// ApproxLineWidth estimates the width of one line of proportional text
// from per-character width classes.
func ApproxLineWidth(line string, fontSize float64) float64 {
	width := 0.0
	for _, r := range StripStyle(line) {
		var em float64
		switch {
		case IsWide(r):
			em = 1
		case IsZeroWidth(r), unicode.IsControl(r):
			em = 0
		case strings.ContainsRune("il.,:;'|!I", r):
			em = 0.28
		case strings.ContainsRune("jtfr ()[]-", r):
			em = 0.36
		case strings.ContainsRune("mwMW@", r):
			em = 0.85
		case unicode.IsUpper(r):
			em = 0.68
		default:
			em = 0.52
		}
		width += em * fontSize
	}
	return width
}

// TextSize returns the size of multi-line text: the widest line measured
// with lineWidth, or ApproxLineWidth when it is nil, and a line height per
// line, at least the font size.
func TextSize(text string, lineHeight, fontSize float64, lineWidth func(line string) float64) (w, h float64) {
	if lineWidth == nil {
		lineWidth = func(line string) float64 { return ApproxLineWidth(line, fontSize) }
	}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		w = max(w, lineWidth(StripStyle(line)))
	}
	return w, float64(len(lines)) * max(lineHeight, fontSize)
}
