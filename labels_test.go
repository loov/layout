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
	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if size := l.Node(blank).Size; size.X <= 0 || size.Y <= 0 {
		t.Errorf("empty label node has size %v", size)
	}

	writers := map[string]func(*bytes.Buffer) error{
		"svg":     func(b *bytes.Buffer) error { return svg.Write(b, l) },
		"text":    func(b *bytes.Buffer) error { return text.Write(b, l) },
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
	if err := dot.Write(&out, l); err != nil {
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
	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	abs := func(v layout.Length) layout.Length { return max(v, -v) }
	for i, a := range graph.Edges {
		for j, b := range graph.Edges[i+1:] {
			pa, pb := l.Edges[i], l.Edges[i+1+j]
			if abs(pa.LabelCenter.X-pb.LabelCenter.X) < (pa.LabelSize.X+pb.LabelSize.X)/2 && abs(pa.LabelCenter.Y-pb.LabelCenter.Y) < (pa.LabelSize.Y+pb.LabelSize.Y)/2 {
				t.Errorf("labels %s at %v and %s at %v overlap", a.Label, pa.LabelCenter, b.Label, pb.LabelCenter)
			}
		}
	}
}
