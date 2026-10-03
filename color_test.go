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
