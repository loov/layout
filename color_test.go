package layout_test

import (
	"testing"

	"github.com/loov/layout"
)

// TestHSL checks that hues are in degrees.
func TestHSL(t *testing.T) {
	for _, test := range []struct {
		hsl  layout.HSL
		want layout.RGB
	}{
		{layout.HSL{0, 1, 0.5}, layout.RGB{R: 0xFF}},
		{layout.HSL{120, 1, 0.5}, layout.RGB{G: 0xFF}},
		{layout.HSL{240, 1, 0.5}, layout.RGB{B: 0xFF}},
		{layout.HSL{360, 1, 0.5}, layout.RGB{R: 0xFF}},
		{layout.HSL{-120, 1, 0.5}, layout.RGB{B: 0xFF}},
	} {
		r, g, b, a := test.hsl.RGBA8()
		if got := (layout.RGB{R: r, G: g, B: b}); got != test.want || a != 0xFF {
			t.Errorf("%v = %v alpha %v, want %v", test.hsl, got, a, test.want)
		}
	}
}

// TestParseColor checks the color names and hex forms that dot files and
// glay flags accept.
func TestParseColor(t *testing.T) {
	for _, test := range []struct {
		value string
		want  layout.Color
	}{
		{"red", layout.RGB{R: 255}},
		{"Light Blue", layout.RGB{R: 173, G: 216, B: 230}},
		{"#102030", layout.RGB{R: 0x10, G: 0x20, B: 0x30}},
		{"#10203040", layout.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0x40}},
		{"#123", nil},
		{"#+12345", nil},
		{"#gg0000", nil},
		{"nosuchcolor", nil},
		{"", nil},
	} {
		got, ok := layout.ParseColor(test.value)
		if got != test.want || ok != (test.want != nil) {
			t.Errorf("ParseColor(%q) = %v, %v; want %v", test.value, got, ok, test.want)
		}
	}
}
