package text

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/loov/layout"
)

func TestWrite(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Edge("A", "B").Label = "yes"
	graph.Edge("A", "C")
	graph.Edge("B", "D")
	graph.Edge("C", "D")
	Prepare(graph)
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, graph); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	t.Log("\n" + got)
	for _, want := range []string{"A", "B", "C", "D", "yes", "▼", "╭", "│"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestOverlap(t *testing.T) {
	c := &canvas{w: 5, h: 3}
	c.cells = []rune(strings.Repeat(" ", c.w*c.h))
	c.lines = make([]int, c.w*c.h)
	c.heavy = make([]int, c.w*c.h)
	c.fg = make([]uint32, c.w*c.h)
	c.owner = make([][4]int, c.w*c.h)
	c.solid = make([]bool, c.w*c.h)
	c.edge = 1
	c.walk(0, 0, 4, 2) // right along the top, then down
	c.edge = 2
	c.walk(2, 0, 4, 1) // shares the top from x=2 and the start of the drop
	got := string(c.cells)
	if want := "──╼━┓    ╿    │"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestArrows(t *testing.T) {
	markers := "▲▼◀▶↑↓←→●○"
	for _, tc := range []struct {
		head, tail layout.Arrow
		sideways   bool   // lay out left to right
		want       string // markers in reading order
	}{
		{layout.ArrowDefault, layout.ArrowDefault, false, "▼"},
		{layout.ArrowNone, layout.ArrowNone, false, ""},
		{layout.ArrowNormal, layout.ArrowNormal, false, "▲▼"},
		{layout.ArrowVee, layout.ArrowVee, false, "↑↓"},
		{layout.ArrowDot, layout.ArrowDot, false, "●●"},
		{layout.ArrowODot, layout.ArrowODot, false, "○○"},
		{layout.ArrowVee, layout.ArrowDot, false, "●↓"},
		{layout.ArrowNormal, layout.ArrowNormal, true, "◀▶"},
		{layout.ArrowVee, layout.ArrowVee, true, "←→"},
	} {
		graph := layout.NewDigraph()
		edge := graph.Edge("A", "B")
		edge.ArrowHead, edge.ArrowTail = tc.head, tc.tail
		if tc.sideways {
			graph.RankDir = layout.LeftToRight
		}
		Prepare(graph)
		if err := layout.Hierarchical(graph); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := Write(&buf, graph); err != nil {
			t.Fatal(err)
		}
		got := strings.Map(func(r rune) rune {
			if strings.ContainsRune(markers, r) {
				return r
			}
			return -1
		}, buf.String())
		if got != tc.want {
			t.Errorf("head %q tail %q: got markers %q, want %q\n%s", tc.head, tc.tail, got, tc.want, buf.String())
		}
	}
}

func TestLineStyles(t *testing.T) {
	for _, tc := range []struct {
		style    layout.LineStyle
		sideways bool // lay out left to right
		want     rune
	}{
		{layout.Solid, false, '│'},
		{layout.Bold, false, '│'},
		{layout.Dashed, false, '┊'},
		{layout.Dotted, false, '┊'},
		{layout.Solid, true, '─'},
		{layout.Dashed, true, '┈'},
		{layout.Dotted, true, '┈'},
	} {
		graph := layout.NewDigraph()
		graph.Edge("A", "B").LineStyle = tc.style
		if tc.sideways {
			graph.RankDir = layout.LeftToRight
		}
		Prepare(graph)
		if err := layout.Hierarchical(graph); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := Write(&buf, graph); err != nil {
			t.Fatal(err)
		}
		// the run between the boxes, past the box sides
		var run []rune
		for _, line := range strings.Split(buf.String(), "\n") {
			r := []rune(strings.TrimSpace(line))
			switch {
			case tc.sideways && len(r) > 0 && strings.ContainsRune(line, '▶'):
				run = r[slices.Index(r, '├')+1 : slices.Index(r, '▶')]
			case !tc.sideways && len(r) == 1 && r[0] != '▼':
				run = append(run, r[0])
			}
		}
		if len(run) == 0 || strings.Trim(string(run), string(tc.want)) != "" {
			t.Errorf("%q sideways=%v: got run %q, want only %q\n%s", tc.style, tc.sideways, string(run), tc.want, buf.String())
		}
	}
}

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
	Prepare(graph)
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		opts Options
		want string // every line starts with this
	}{
		{Options{Background: layout.RGB{0xFF, 0xFF, 0xFF}}, "\x1b[39;107m"},
		{Options{Background: layout.RGB{0, 0, 0}, Palette: TrueColor}, "\x1b[39;48;2;0;0;0m"},
	} {
		var buf bytes.Buffer
		if err := WriteColor(&buf, graph, tc.opts); err != nil {
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
