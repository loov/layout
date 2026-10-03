package text

import (
	"bytes"
	"strings"
	"testing"

	"github.com/loov/layout"
)

func TestPlain(t *testing.T) {
	for label, want := range map[string]string{
		"plain\ntext":               "plain\ntext",
		"<<B>bold</B>>":             "bold",
		"<one<br/>two &amp; three>": "one\ntwo & three",
		"<<TABLE><TR><TD>a</TD><TD>b</TD></TR><TR><TD>c</TD></TR></TABLE>>": "a b\nc",
		"<  spaced   <I>out</I>  >":                                         "spaced out",
	} {
		if got := plain(label); got != want {
			t.Errorf("plain(%q) = %q, want %q", label, got, want)
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
	Prepare(graph)
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, graph); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); !strings.Contains(got, "│bold│") || !strings.Contains(got, "┌────┐") || !strings.Contains(got, "└────┘") {
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
	Prepare(graph)
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, graph); err != nil {
		t.Fatal(err)
	}
	var widths []int
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		widths = append(widths, width(strings.TrimSpace(line)))
	}
	if len(widths) != 3 || widths[0] != widths[1] || widths[1] != widths[2] || widths[1] != 10 {
		t.Errorf("box rows are %v columns wide, want 10 each:\n%s", widths, buf.String())
	}
}

// TestClusterLabel checks that a long cluster label is drawn whole on the
// top of its frame.
func TestClusterLabel(t *testing.T) {
	graph := layout.NewDigraph()
	label := "Extremely long cluster label spanning many characters"
	graph.Clusters = []*layout.Cluster{{ID: "c", Label: label, Nodes: []*layout.Node{graph.Node("a")}}}
	Prepare(graph)
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
