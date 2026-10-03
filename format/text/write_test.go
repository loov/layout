package text

import (
	"bytes"
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
