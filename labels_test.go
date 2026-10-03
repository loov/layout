package layout_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/format/graphml"
	"github.com/loov/layout/format/svg"
	"github.com/loov/layout/format/text"
)

// TestEmptyNodeLabel checks that a node with an explicitly empty label is
// drawn without text, rather than with its id, but still gets a size.
func TestEmptyNodeLabel(t *testing.T) {
	graphs, err := dot.ParseString(`digraph { blank_node [label=""]; plain_node; blank_node -> plain_node }`)
	if err != nil {
		t.Fatal(err)
	}
	graph := graphs[0]
	blank, plain := graph.Node("blank_node"), graph.Node("plain_node")
	if got := blank.DefaultLabel(); got != "" {
		t.Errorf("label=\"\" gives label %q", got)
	}
	if got := plain.DefaultLabel(); got != "plain_node" {
		t.Errorf("no label gives label %q, want the id", got)
	}
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	if blank.Radius.X <= 0 || blank.Radius.Y <= 0 {
		t.Errorf("empty label node has radius %v", blank.Radius)
	}

	writers := map[string]func(*bytes.Buffer) error{
		"svg":     func(b *bytes.Buffer) error { return svg.Write(b, graph) },
		"text":    func(b *bytes.Buffer) error { return text.Write(b, graph) },
		"graphml": func(b *bytes.Buffer) error { return graphml.Write(b, graph) },
	}
	for name, write := range writers {
		var out bytes.Buffer
		if err := write(&out); err != nil {
			t.Fatal(err)
		}
		// svg and graphml also have the id as an attribute, look for text
		shown := func(id string) bool {
			if name == "text" {
				return strings.Contains(out.String(), id)
			}
			return strings.Contains(out.String(), ">"+id+"<")
		}
		if shown("blank_node") {
			t.Errorf("%s shows the id of the node with an empty label:\n%s", name, out.String())
		}
		if !shown("plain_node") {
			t.Errorf("%s misses the id of the unlabeled node:\n%s", name, out.String())
		}
	}

	var out bytes.Buffer
	if err := dot.Write(&out, graph); err != nil {
		t.Fatal(err)
	}
	again, err := dot.ParseString(out.String())
	if err != nil {
		t.Fatal(err)
	}
	if got := again[0].Node("blank_node").DefaultLabel(); got != "" {
		t.Errorf("written as dot, the empty label became %q:\n%s", got, out.String())
	}
}

// TestLabelsApart checks that labels near a crowded node don't cover each
// other when no spot is clear of every edge.
func TestLabelsApart(t *testing.T) {
	graphs, err := dot.ParseString(`digraph { rankdir=BT;
		n0 -> n8 [label="e0_8"]; n1 -> n5 [label="e1_5"]; n1 -> n7 [label="e1_7"];
		n1 -> n8 [label="e1_8"]; n2 -> n5 [label="e2_5"]; n2 -> n6 [label="e2_6"];
		n3 -> n7 [label="e3_7"]; n4 -> n9 [label="e4_9"]; n5 -> n6 [label="e5_6"];
		n5 -> n8 [label="e5_8"]; n5 -> n9 [label="e5_9"]; n6 -> n7 [label="e6_7"];
		n7 -> n8 [label="e7_8"]; n8 -> n9 [label="e8_9"];
		n0 [label="node 0"]; n1 [label="node 1"]; n2 [label="node 2"]; n3 [label="node 3"]; n4 [label="node 4"];
		n5 [label="node 5"]; n6 [label="node 6"]; n7 [label="node 7"]; n8 [label="node 8"]; n9 [label="node 9"];
	}`)
	if err != nil {
		t.Fatal(err)
	}
	graph := graphs[0]
	if err := layout.Hierarchical(graph); err != nil {
		t.Fatal(err)
	}
	abs := func(v layout.Length) layout.Length { return max(v, -v) }
	for i, a := range graph.Edges {
		for _, b := range graph.Edges[i+1:] {
			if abs(a.LabelPos.X-b.LabelPos.X) < a.LabelRadius.X+b.LabelRadius.X && abs(a.LabelPos.Y-b.LabelPos.Y) < a.LabelRadius.Y+b.LabelRadius.Y {
				t.Errorf("labels %s at %v and %s at %v overlap", a.Label, a.LabelPos, b.Label, b.LabelPos)
			}
		}
	}
}
