package text

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/loov/layout"
	"github.com/loov/layout/internal/draw"
)

func TestPlain(t *testing.T) {
	for label, want := range map[string]string{
		"plain\ntext":               "plain\ntext",
		"<<B>bold</B>>":             "bold",
		"<one<br/>two &amp; three>": "one\ntwo & three",
		"<<TABLE><TR><TD>a</TD><TD>b</TD></TR><TR><TD>c</TD></TR></TABLE>>":    "a b\nc",
		"<  spaced   <I>out</I>  >":                                            "spaced out",
		"<<TABLE>\n\t<TR>\n\t\t<TD>a</TD>\n\t\t<TD>b</TD>\n\t</TR>\n</TABLE>>": "a b",
	} {
		if got := draw.PlainLabel(label); got != want {
			t.Errorf("draw.PlainLabel(%q) = %q, want %q", label, got, want)
		}
	}
}

// TestHTMLLabel checks that an HTML-like label is drawn as its text in a
// complete box.
func TestHTMLLabel(t *testing.T) {
	graph := layout.NewDigraph()
	node := graph.Node("a")
	node.Shape = layout.Box
	node.Label = "<<B>bold</B>>"
	l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, l); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !regexp.MustCompile(`(?m)^ *┌(─+)┐\n *│ *bold *│\n *└(─+)┘$`).MatchString(got) {
		t.Errorf("want bold in a whole box:\n%s", got)
	}
}

// TestWideLabel checks that a label of wide characters fills two columns
// per character, so that the box around it lines up.
func TestWideLabel(t *testing.T) {
	graph := layout.NewDigraph()
	node := graph.Node("a")
	node.Shape = layout.Box
	node.Label = "漢字漢字"
	l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, l); err != nil {
		t.Fatal(err)
	}
	var widths []int
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		widths = append(widths, draw.Columns(strings.TrimSpace(line)))
	}
	if len(widths) != 3 || widths[0] != widths[1] || widths[1] != widths[2] || widths[1] < 10 {
		t.Errorf("box rows are %v columns wide, want at least 10 each:\n%s", widths, buf.String())
	}
}

// TestClusterLabel checks that a long cluster label is drawn whole on the
// top of its frame.
func TestClusterLabel(t *testing.T) {
	graph := layout.NewDigraph()
	label := "Extremely long cluster label spanning many characters"
	graph.Clusters = []*layout.Cluster{{ID: "c", Label: label, Nodes: []*layout.Node{graph.Node("a")}}}
	l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, l); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "┌ "+label+" ") {
		t.Errorf("label is cut:\n%s", buf.String())
	}
}

// TestBlanksInLabels checks that carving keeps the blank columns and rows
// inside labels and boxes.
func TestBlanksInLabels(t *testing.T) {
	graph := layout.NewDigraph()
	wide := graph.Node("a")
	wide.Label, wide.MinSize.X = "x            y", 216
	graph.Node("b").Label = "top\n\n\n\nbottom"
	graph.Edge("c", "d").Label = "p      q"
	got := render(t, graph)
	for _, want := range []string{"x            y", "p      q"} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q:\n%s", want, got)
		}
	}
	lines := strings.Split(got, "\n")
	top := slices.IndexFunc(lines, func(s string) bool { return strings.Contains(s, "top") })
	bottom := slices.IndexFunc(lines, func(s string) bool { return strings.Contains(s, "bottom") })
	if bottom-top != 4 {
		t.Errorf("want 3 blank lines between top and bottom, got %d:\n%s", bottom-top-1, got)
	}
}

// TestControlCharacters checks that control characters in labels are not
// written, as they would garble the terminal, and take no columns.
func TestControlCharacters(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Shape = layout.Box
	graph.Node("a").Label = "x\x1b[31my\tz\x00"
	graph.Edge("a", "b").Label = "e\x1b[0m"
	got := render(t, graph)
	if strings.ContainsFunc(got, func(r rune) bool { return r != '\n' && unicode.IsControl(r) }) {
		t.Errorf("control characters are written:\n%q", got)
	}
	if !regexp.MustCompile(`│ *x\[31myz *│`).MatchString(got) {
		t.Errorf("want the label without control characters in a whole box:\n%s", got)
	}
}

// TestFitsLayout checks that the layout reserves the cells text draws in:
// node boxes and cluster frames don't grow past their layout boxes, and
// node boxes don't overlap, and edge labels get their columns.
func TestFitsLayout(t *testing.T) {
	labels := []string{"a", "hello world", "one\ntwo\nthree", "漢字", "é 👩‍💻", "iiiiiiii", "<<b>bold</b> text>",
		"<<TABLE><TR><TD>a</TD><TD>b</TD></TR><TR><TD>c</TD></TR></TABLE>>"}
	records := []string{"a|b", "{a|b|c}", "<p> left|{top|mid|bottom}|right", "x|{y|{z|w}}", "{a\\nb|c}"}
	shapes := []layout.Shape{layout.Box, layout.Ellipse, layout.Circle, layout.Square, layout.None, layout.Record, layout.Auto}
	for _, dir := range []layout.RankDir{layout.TopToBottom, layout.LeftToRight} {
		for _, shape := range shapes {
			for peripheries := 1; peripheries <= 2; peripheries++ {
				graph := layout.NewDigraph()
				graph.RankDir = dir
				for i := range 8 {
					node := graph.Node(string(rune('a' + i)))
					node.Shape, node.Peripheries = shape, peripheries
					node.Label = labels[(i+peripheries)%len(labels)]
					if shape == layout.Record {
						node.Label = records[i%len(records)]
					}
				}
				for i := 1; i < 8; i++ {
					graph.Edge(graph.Nodes[(i-1)/2].ID, graph.Nodes[i].ID).Label = labels[i%len(labels)]
				}
				graph.Edge("d", "d")
				graph.Clusters = append(graph.Clusters, &layout.Cluster{ID: "c", Label: "iiiiiiiiiiiiiiii", Nodes: graph.Nodes[1:2]})
				l, err := layout.Hierarchical(graph, layout.Options{ForText: true})
				if err != nil {
					t.Fatal(err)
				}
				c := newCanvas(l)
				for _, node := range graph.Nodes {
					c.drawNode(node)
				}
				for i, node := range graph.Nodes {
					box := l.Nodes[i]
					inset := layout.Length(max(node.Peripheries-1, 0)) * peripheryGap
					want := [4]int{c.col(box.Left() + inset), c.row(box.Top() + inset), c.col(box.Right() - inset), c.row(box.Bottom() - inset)}
					if got := c.boxes[node]; got[2] > want[2] || got[3] > want[3] {
						t.Errorf("%v %v %d: %q drawn at %v, laid out at %v", dir, shape, peripheries, node.Label, got, want)
					}
					for _, other := range graph.Nodes[i+1:] {
						p, q := c.boxes[node], c.boxes[other]
						if p[0] <= q[2] && q[0] <= p[2] && p[1] <= q[3] && q[1] <= p[3] {
							t.Errorf("%v %v %d: %q at %v overlaps %q at %v", dir, shape, peripheries, node.Label, p, other.Label, q)
						}
					}
				}
				for i, edge := range graph.Edges {
					at := l.Edges[i]
					if w := draw.TextColumns(draw.PlainLabel(edge.Label)); c.col(at.LabelCenter.X+at.LabelSize.X/2)-c.col(at.LabelCenter.X-at.LabelSize.X/2) < w {
						t.Errorf("%v %v %d: edge label %q has %.1f for %d columns", dir, shape, peripheries, edge.Label, at.LabelSize.X, w)
					}
				}
				box := l.Clusters[0]
				if need := c.col(box.TopLeft.X) + clusterLabelWidth(graph.Clusters[0]); need > c.col(box.BottomRight.X) {
					t.Errorf("%v %v %d: cluster label needs column %d, frame ends at %d", dir, shape, peripheries, need, c.col(box.BottomRight.X))
				}
			}
		}
	}
}

// TestWidenedBoxFits checks that the canvas holds a box widened past its
// layout for its label, as when the layout wasn't made for text.
func TestWidenedBoxFits(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Node("iiiiiiiiiiiiiiiiiiii")
	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Write(&out, l); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "iiii │") {
		t.Errorf("right border cut off:\n%s", out.String())
	}
}
