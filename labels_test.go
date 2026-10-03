package layout_test

import (
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
)

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
