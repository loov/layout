package layout_test

import (
	"bytes"
	"math"
	"path/filepath"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
)

// TestPinned round-trips a layout through dot: positions written by
// dot.Write are read back as pos and kept by Hierarchical.
func TestPinned(t *testing.T) {
	graphs, err := dot.ParseFile(filepath.Join("testdata", "graphviz", "fsm.gv"))
	if err != nil {
		t.Fatal(err)
	}
	graph := graphs[0]
	l, err := layout.Hierarchical(graph, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := dot.Write(&out, l); err != nil {
		t.Fatal(err)
	}
	parsed, err := dot.Parse(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	pinned := parsed[0]
	for _, node := range pinned.Nodes {
		if node.Pos == nil {
			t.Fatalf("expected node %v to be pinned", node)
		}
	}
	again, err := layout.Hierarchical(pinned, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	near := func(a, b layout.Vector) bool {
		return math.Abs(float64(a.X-b.X)) < 0.05 && math.Abs(float64(a.Y-b.Y)) < 0.05
	}
	for i, node := range graph.Nodes {
		want, got := l.Nodes[i].Center, again.Node(pinned.NodeByID[node.ID]).Center
		if !near(got, want) {
			t.Errorf("node %v moved from %v to %v", node.ID, want, got)
		}
	}
	for i, edge := range graph.Edges {
		want, got := l.Edges[i], again.Edges[i]
		if len(got.Path) != len(want.Path) || !near(got.Path[0], want.Path[0]) || !near(got.Path[len(got.Path)-1], want.Path[len(want.Path)-1]) {
			t.Errorf("edge %v path changed: %v -> %v", edge, want.Path, got.Path)
		}
		if edge.Label != "" && !near(got.LabelCenter, want.LabelCenter) {
			t.Errorf("edge %v label moved from %v to %v", edge, want.LabelCenter, got.LabelCenter)
		}
	}
}
