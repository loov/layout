package layout

import (
	"math"
	"strconv"
	"strings"
)

// Color is anything that can be expressed as 8-bit RGBA.
type Color interface {
	// RGBA8 returns the non-alpha-premultiplied red, green, blue and alpha values
	// for the color. Each value ranges within [0, 0xff].
	RGBA8() (r, g, b, a uint8)
}

// RGB is an opaque 24-bit color.
type RGB struct{ R, G, B uint8 }

// RGBA8 implements Color.
func (rgb RGB) RGBA8() (r, g, b, a uint8) { return rgb.R, rgb.G, rgb.B, 0xFF }

// RGBA is a 32-bit color with alpha.
type RGBA struct{ R, G, B, A uint8 }

// RGBA8 implements Color.
func (rgb RGBA) RGBA8() (r, g, b, a uint8) { return rgb.R, rgb.G, rgb.B, rgb.A }

// ParseColor parses an X11 color name, see ColorByName, or a hex color
// written #RRGGBB or #RRGGBBAA.
func ParseColor(value string) (Color, bool) {
	hex, ok := strings.CutPrefix(value, "#")
	if !ok {
		return ColorByName(value)
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	switch {
	case err != nil:
	case len(hex) == 6:
		return RGB{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v)}, true
	case len(hex) == 8:
		return RGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}, true
	}
	return nil, false
}

// HSL is an opaque color in hue, saturation and lightness space;
// hue is in degrees, saturation and lightness in [0, 1].
type HSL struct{ H, S, L float32 }

// RGBA8 implements Color.
func (hsl HSL) RGBA8() (r, g, b, a uint8) {
	return HSLA{hsl.H, hsl.S, hsl.L, 1.0}.RGBA8()
}

// HSLA is a color in hue, saturation and lightness space with alpha in [0, 1].
type HSLA struct{ H, S, L, A float32 }

// RGBA8 implements Color.
func (hsl HSLA) RGBA8() (r, g, b, a uint8) {
	rf, gf, bf, af := hsla(hsl.H, hsl.S, hsl.L, hsl.A)
	return sat8(rf), sat8(gf), sat8(bf), sat8(af)
}

func hue(v1, v2, h float32) float32 {
	if h < 0 {
		h += 1
	}
	if h > 1 {
		h -= 1
	}
	if 6*h < 1 {
		return v1 + (v2-v1)*6*h
	} else if 2*h < 1 {
		return v2
	} else if 3*h < 2 {
		return v1 + (v2-v1)*(2.0/3.0-h)*6
	}

	return v1
}

func hsla(h, s, l, a float32) (r, g, b, ra float32) {
	if s == 0 {
		return l, l, l, a
	}

	// degrees to a fraction of the full turn, in [0, 1)
	h = float32(math.Mod(float64(h), 360) / 360)
	if h < 0 {
		h += 1
	}

	var v2 float32
	if l < 0.5 {
		v2 = l * (1 + s)
	} else {
		v2 = (l + s) - s*l
	}

	v1 := 2*l - v2
	r = hue(v1, v2, h+1.0/3.0)
	g = hue(v1, v2, h)
	b = hue(v1, v2, h-1.0/3.0)
	ra = a

	return
}

// sat8 converts 0..1 float to 0..0xFF uint16
func sat8(v float32) uint8 {
	v = v * 0xFF
	if v >= 0xFF {
		return 0xFF
	} else if v <= 0 {
		return 0
	}
	return uint8(v)
}
