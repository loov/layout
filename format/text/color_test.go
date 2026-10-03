package text

import (
	"bytes"
	"strings"
	"testing"

	"github.com/loov/layout"
)

func TestANSI16(t *testing.T) {
	for _, tc := range []struct {
		color uint32
		role  role
		want  int
	}{
		{0xFF0000, ink, 31},
		{0xFFA020, ink, 33}, // orange is closest to yellow
		{0xA0FFA0, ink, 92}, // light colors are bright
		{0x0000FF, ink, 34},
		{0x00FFFF, ink, 36},
		{0x80FFFF, ink, 96},
		{0x800080, ink, 35},
		{0x202020, ink, 0}, // dark and light grays are the theme's text color
		{0xF0F0F0, ink, 0},
		{0x808080, ink, 90},
		{0x202020, inkOnFill, 30},
		{0xF0F0F0, inkOnFill, 97},
		{0xEEEEFF, fill, 0}, // washed out fills are dropped
		{0x808080, fill, 0},
		{0xFFA020, fill, 43},
		{0xE87C7C, fill, 101},
		{0xFFFFFF, ground, 107},
		{0x000000, ground, 40},
		{0x606060, ground, 100},
	} {
		if got := ansi16(1<<24|tc.color, tc.role); got != tc.want {
			t.Errorf("ansi16(%06X, %v) = %v, want %v", tc.color, tc.role, got, tc.want)
		}
	}
	if got := ansi16(0, ink); got != 0 {
		t.Errorf("unset color: got %v, want 0", got)
	}
}

func TestBackground(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Edge("A", "B")
	l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		opts Options
		want string // every line starts with this
	}{
		{Options{Background: layout.RGB{R: 0xFF, G: 0xFF, B: 0xFF}}, "\x1b[39;107m"},
		{Options{Background: layout.RGB{}, Palette: TrueColor}, "\x1b[39;48;2;0;0;0m"},
	} {
		var buf bytes.Buffer
		if err := WriteColor(&buf, l, tc.opts); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
		for _, line := range lines {
			if !strings.HasPrefix(line, tc.want) {
				t.Errorf("%+v: line %q does not start with %q", tc.opts, line, tc.want)
				break
			}
		}
		// lines without a color are black on white and white on black
		if dark := tc.opts.Palette == TrueColor; !strings.Contains(buf.String(), map[bool]string{false: "\x1b[30;107m", true: "\x1b[38;2;255;255;255;48;2;0;0;0m"}[dark]) {
			t.Errorf("%+v: lines are not drawn in a contrasting color\n%q", tc.opts, buf.String())
		}
	}
}
