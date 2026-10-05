package text

import (
	"fmt"
	"math"
	"strconv"

	"github.com/loov/layout"
	"strings"
)

// rgb packs a color for a cell: 0 is the terminal default, anything else
// is 1<<24 | red<<16 | green<<8 | blue. Transparent colors are the default.
func rgb(color layout.Color) uint32 {
	if color == nil {
		return 0
	}
	r, g, b, a := color.RGBA8()
	if a == 0 {
		return 0
	}
	return 1<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b)
}

// Options configure WriteColor.
type Options struct {
	// Palette selects how colors are written.
	Palette Palette
	// Background fills the whole drawing; nil leaves it to the terminal.
	Background layout.Color
}

// Palette selects the colors that WriteColor writes.
type Palette int

const (
	// ANSI16 maps colors to the 16 basic ANSI colors, which terminals draw
	// from their theme, so that the drawing fits light and dark themes.
	// Grays and washed out fills are left to the terminal.
	ANSI16 Palette = iota
	// TrueColor writes colors as given, with 24-bit escape codes.
	TrueColor
)

// codes returns the escape code parameters that select the cell colors
// fg and bg. Lines and text without a color are black or white on a fill
// or background, whichever stands out.
func (opts *Options) codes(fg, bg uint32) (string, string) {
	if opts.Palette == TrueColor {
		code := func(set int, color uint32) string {
			return fmt.Sprintf("%d;2;%d;%d;%d", set, color>>16&0xFF, color>>8&0xFF, color&0xFF)
		}
		if bg == 0 {
			bg = rgb(opts.Background)
		}
		f, b := "39", "49"
		if bg != 0 {
			b = code(48, bg)
			if fg == 0 && lightness(bg) >= 0.5 {
				fg = 1 << 24
			} else if fg == 0 {
				fg = 1<<24 | 0xFFFFFF
			}
		}
		if fg != 0 {
			f = code(38, fg)
		}
		return f, b
	}
	under := bg
	b := ansi16(bg, fill)
	if b == 0 {
		under = rgb(opts.Background)
		b = ansi16(under, ground)
	}
	f := ansi16(fg, ink)
	if b != 0 {
		f = ansi16(fg, inkOnFill)
		if f == 0 && lightness(under) >= 0.5 {
			f = 30
		} else if f == 0 {
			f = 97
		}
	}
	code := func(n int, reset string) string {
		if n == 0 {
			return reset
		}
		return strconv.Itoa(n)
	}
	return code(f, "39"), code(b, "49")
}

// role is what a color is used for when mapping it to the basic colors
type role int

const (
	ink       role = iota // lines and text on the terminal background
	inkOnFill             // lines and text on a fill or background
	fill                  // inside nodes and clusters
	ground                // the background of the whole drawing
)

// ansi16 returns the escape code of the basic ANSI color closest to color
// in its role, or 0 for the terminal default. Themes change the shades
// but keep the hues, so the hue picks the color and lightness picks
// between normal and bright. Grays on the terminal background become the
// default or bright black, which themes keep readable, and dark or light
// ones on a fill become black or bright white. Gray and washed out fills
// are dropped; a gray background picks the closest gray.
func ansi16(color uint32, role role) int {
	if color == 0 {
		return 0
	}
	r := float64(color>>16&0xFF) / 0xFF
	g := float64(color>>8&0xFF) / 0xFF
	b := float64(color&0xFF) / 0xFF
	hi, lo := max(r, g, b), min(r, g, b)
	light := (hi + lo) / 2
	base := 30
	if role >= fill {
		base = 40
	}
	if hi-lo < 0.15 || role == fill && light > 0.85 {
		switch {
		case role == fill:
			return 0
		case role == ground:
			return base + [4]int{0, 60, 7, 67}[min(int(light*4), 3)]
		case role == inkOnFill && light < 0.3:
			return 30
		case role == inkOnFill && light >= 0.7:
			return 97
		case light < 0.3 || light >= 0.7:
			return 0
		}
		return 90
	}
	var hue float64 // in sixths of the circle: red, yellow, green, cyan, blue, magenta
	switch hi {
	case r:
		hue = math.Mod((g-b)/(hi-lo)+6, 6)
	case g:
		hue = (b-r)/(hi-lo) + 2
	default:
		hue = (r-g)/(hi-lo) + 4
	}
	if light > 0.6 {
		base += 60
	}
	return base + [6]int{1, 3, 2, 6, 4, 5}[int(hue+0.5)%6]
}

// lightness returns the HSL lightness of a cell color, in [0, 1]
func lightness(color uint32) float64 {
	r, g, b := color>>16&0xFF, color>>8&0xFF, color&0xFF
	return float64(max(r, g, b)+min(r, g, b)) / (2 * 0xFF)
}

// encode writes the grid as lines of text without trailing blanks,
// colored with escape codes unless opts is nil
func encode(grid [][]cell, opts *Options) string {
	// the escape code parameters of each pair of cell colors
	codes := map[[2]uint32][2]string{}
	code := func(x cell) (string, string) {
		if opts == nil {
			return "39", "49"
		}
		k := [2]uint32{x.fg, x.bg}
		fb, ok := codes[k]
		if !ok {
			fb[0], fb[1] = opts.codes(x.fg, x.bg)
			codes[k] = fb
		}
		return fb[0], fb[1]
	}
	var out strings.Builder
	for _, line := range grid {
		end := 0
		for i, x := range line {
			if _, bg := code(x); x.r != ' ' || bg != "49" {
				end = i + 1
			}
		}
		f, b := "39", "49"
		for _, x := range line[:end] {
			want, bg := code(x)
			if x.r == ' ' {
				want = f // blanks show only the background
			}
			if want != f || bg != b {
				f, b = want, bg
				out.WriteString("\x1b[" + f + ";" + b + "m")
			}
			if x.r != covered {
				out.WriteRune(x.r)
			}
		}
		if f != "39" || b != "49" {
			out.WriteString("\x1b[0m")
		}
		out.WriteByte('\n')
	}
	return out.String()
}
