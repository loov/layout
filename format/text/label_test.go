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
