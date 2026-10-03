package text

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
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

// TestSidewaysEdgesApart checks that edges on the side of a node in a
// sideways layout get rows of their own between its borders, instead of
// sharing one or running along a border.
func TestSidewaysEdgesApart(t *testing.T) {
	for name, edges := range map[string][][2]string{
		"fan out":  {{"a", "b"}, {"a", "c"}, {"a", "d"}},
		"straight": {{"s0", "s1"}, {"s0", "s2"}, {"s1", "s2"}},
	} {
		graph := layout.NewDigraph()
		graph.RankDir = layout.LeftToRight
		for _, e := range edges {
			graph.Edge(e[0], e[1]).Label = "x"
			graph.Node(e[0]).Shape = layout.Circle
		}
		Prepare(graph)
		if err := layout.HierarchicalWith(graph, layout.Options{Align: layout.AlignLeft}); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := Write(&buf, graph); err != nil {
			t.Fatal(err)
		}
		got := buf.String()
		// heavy lines are shared runs; a line beside a box corner runs
		// along the border
		if strings.ContainsAny(got, "━┃┮┶┾┥┝┑┙┕┍") || strings.Contains(got, "╯─") || strings.Contains(got, "╮─") {
			t.Errorf("%s: edges share a run or run along a border:\n%s", name, got)
		}
		if n := strings.Count(got, "▶"); n != len(edges) {
			t.Errorf("%s: got %d arrowheads, want %d:\n%s", name, n, len(edges), got)
		}
	}
}

// TestOffCanvas checks that pinned nodes outside the drawing don't make
// writing index outside the canvas.
func TestOffCanvas(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Pinned = true
	graph.Node("A").Center = layout.Vector{X: 50, Y: -100}
	graph.Node("B").Center = layout.Vector{X: 150, Y: 50}
	graph.Edge("A", "B").FromPort = layout.East
	Prepare(graph)
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, graph); err != nil {
		t.Fatal(err)
	}
}

// TestDotFanOut checks that edges fanning out of a dot each leave it on
// a side of its own, attached and without sharing a run.
func TestDotFanOut(t *testing.T) {
	for _, tc := range []struct {
		dir   layout.RankDir
		edges int
	}{{layout.TopToBottom, 2}, {layout.TopToBottom, 3}, {layout.LeftToRight, 2}, {layout.LeftToRight, 3}} {
		graph := layout.NewDigraph()
		graph.RankDir = tc.dir
		graph.Node("s").Shape = layout.Dot
		for _, id := range []string{"a", "b", "c"}[:tc.edges] {
			graph.Edge("s", id)
		}
		Prepare(graph)
		if err := layout.Hierarchical(graph); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := Write(&buf, graph); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(buf.String(), "\n")
		at := func(x, y int) rune {
			if y < 0 || y >= len(lines) || x < 0 || x >= len([]rune(lines[y])) {
				return ' '
			}
			return []rune(lines[y])[x]
		}
		touching := 0
		for y, line := range lines {
			if x := slices.Index([]rune(line), '●'); x >= 0 {
				for _, n := range []struct {
					r     rune
					cells string // characters with an arm toward the dot
				}{{at(x, y-1), "│┌┐├┤┬┼╭╮"}, {at(x, y+1), "│└┘├┤┴┼╰╯▼"}, {at(x-1, y), "─┌└├┬┴┼╭╰"}, {at(x+1, y), "─┐┘┤┬┴┼╮╯▶"}} {
					if strings.ContainsRune(n.cells, n.r) {
						touching++
					}
				}
			}
		}
		if touching != tc.edges || strings.ContainsAny(buf.String(), "━┃") {
			t.Errorf("%v with %d edges: %d lines touch the dot, want one per edge without overlaps:\n%s", tc.dir, tc.edges, touching, buf.String())
		}
	}
}

// TestLabelsBeforeOrigin checks that labels the layout nudges to
// negative coordinates are drawn whole.
func TestLabelsBeforeOrigin(t *testing.T) {
	graph := layout.NewDigraph()
	for _, e := range [][3]string{{"a", "b", "one"}, {"a", "b", "two"}, {"b", "a", "back"}} {
		edge := layout.NewEdge(graph.Node(e[0]), graph.Node(e[1]))
		edge.Label = e[2]
		graph.AddEdge(edge)
	}
	Prepare(graph)
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, graph); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"one", "two", "back"} {
		if !strings.Contains(buf.String(), label) {
			t.Errorf("label %q is missing:\n%s", label, buf.String())
		}
	}
}

// TestMultilineLabels checks that labels with several lines are drawn a
// line per row, inside record boxes and beside edges.
func TestMultilineLabels(t *testing.T) {
	graph := layout.NewDigraph()
	rec := graph.Node("r")
	rec.Shape = layout.Record
	rec.Label = "first\nsecond"
	graph.Edge("a", "b").Label = "one\ntwo"
	Prepare(graph)
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, graph); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{"│ first │", "│second │", "│ one", "│ two"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
}

// render prepares, lays out and draws graph
func render(t *testing.T, graph *layout.Graph) string {
	t.Helper()
	Prepare(graph)
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, graph); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// TestLabelsFitBoxes checks that the layout leaves room for boxes as wide
// as their labels take cells, so that they don't run into each other, and
// that record fields hold their texts between the dividers.
func TestLabelsFitBoxes(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Shape = layout.Box
	graph.Edge("a", "b")
	graph.Edge("a", "c")
	graph.Node("b").Label = "p                                  q"
	got := render(t, graph)
	if !regexp.MustCompile(`│p +q *│ +│ *c *│`).MatchString(got) {
		t.Errorf("b runs into c:\n%s", got)
	}

	for _, tc := range []struct{ label, want string }{
		{"{p   q|漢字漢字漢字漢字漢字漢字}", `│ *漢字漢字漢字漢字漢字漢字 *│`},
		{"<f0> left|<f1> mid dle|<f2> right", `│ *left *│ *mid dle *│ *right *│`},
	} {
		graph := layout.NewDigraph()
		node := graph.Node("a")
		node.Shape, node.Label = layout.Record, tc.label
		got := render(t, graph)
		if !regexp.MustCompile(tc.want).MatchString(got) {
			t.Errorf("%q: fields overflow:\n%s", tc.label, got)
		}
	}
}

// renderDot parses src and draws it like glay -t txt does
func renderDot(t *testing.T, src string) string {
	t.Helper()
	graphs, err := dot.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	return render(t, graphs[0])
}

// TestMarkerKept checks that an edge drawn later doesn't erase the
// arrowhead of an earlier one.
func TestMarkerKept(t *testing.T) {
	got := renderDot(t, `digraph { a [pos="0,0"]; b [shape=point,pos="0,200"]; a -> b; b -> b }`)
	if !strings.Contains(got, "▲") {
		t.Errorf("the arrowhead of a -> b is missing:\n%s", got)
	}
}

// TestOverlappingWideText checks that text written over half of a wide
// character blanks the other half, so that the row doesn't shift.
func TestOverlappingWideText(t *testing.T) {
	got := renderDot(t, `digraph {
		a [pos="0,0"]; b [pos="0,100"]; c [pos="200,0"]; d [pos="200,100"]
		a -> b [label="漢字漢字", lp="60,50"]; c -> d [label="xyz", lp="48,50"]
	}`)
	var widths []int
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "xyz") || strings.Count(line, "│") == 2 {
			widths = append(widths, width(line))
		}
	}
	if len(widths) < 2 || slices.Min(widths) != slices.Max(widths) {
		t.Errorf("rows between the nodes are %v columns wide, want them equal:\n%s", widths, got)
	}
}

// TestDiagonalEnds checks that a diagonal edge leaves and reaches the
// boxes across their borders, with the arrowhead pointing along the last
// leg.
func TestDiagonalEnds(t *testing.T) {
	got := renderDot(t, `digraph { a [pos="0,200"]; b [pos="300,0"]; a -> b [pos="0,182 300,18"] }`)
	if !strings.Contains(got, "▼") || strings.Contains(got, "▶") || !regexp.MustCompile(`╰─*┬─*╯`).MatchString(got) {
		t.Errorf("want the edge to leave a down and reach b down:\n%s", got)
	}
}
