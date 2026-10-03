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
		"<<TABLE><TR><TD>a</TD><TD>b</TD></TR><TR><TD>c</TD></TR></TABLE>>": "a b\nc",
		"<  spaced   <I>out</I>  >":                                         "spaced out",
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
	graph.ForText = true
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, graph); err != nil {
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
	graph.ForText = true
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, graph); err != nil {
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
	graph.ForText = true
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, graph); err != nil {
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
	wide.Label, wide.Radius.X = "x            y", 108
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
