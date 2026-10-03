package layout_test

import (
	"bytes"
	"math"
	"slices"
	"testing"

	"github.com/loov/layout"
	"github.com/loov/layout/format/svg"
)

func TestForce(t *testing.T) {
	graph := layout.NewGraph()
	// a 5-cycle plus a pendant node
	for _, e := range [][2]string{{"A", "B"}, {"B", "C"}, {"C", "D"}, {"D", "E"}, {"E", "A"}, {"A", "F"}} {
		graph.Edge(e[0], e[1])
	}
	if err := layout.Force(graph); err != nil {
		t.Fatal(err)
	}
	for _, a := range graph.Nodes {
		if a.Center.X < a.Radius.X || a.Center.Y < a.Radius.Y {
			t.Errorf("%v is outside the drawing at %v", a, a.Center)
		}
		for _, b := range graph.Nodes {
			if a == b {
				continue
			}
			d := math.Hypot(float64(a.Center.X-b.Center.X), float64(a.Center.Y-b.Center.Y))
			if d < float64(a.Radius.X+b.Radius.X) {
				t.Errorf("%v and %v overlap, distance %.1f", a, b, d)
			}
		}
	}
	for _, edge := range graph.Edges {
		if len(edge.Path) != 2 {
			t.Errorf("%v has no path", edge)
		}
	}
	var got bytes.Buffer
	if err := svg.Write(&got, graph); err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "testdata/force.svg", got.Bytes())
}

func TestForcePreservesPinnedCentersAndPaths(t *testing.T) {
	graph := layout.NewGraph()
	graph.Pinned = true
	existing := graph.Edge("a", "b")
	missing := graph.Edge("b", "c")
	centers := []layout.Vector{{100, 100}, {100, 200}, {200, 200}}
	for i, node := range graph.Nodes {
		node.Center = centers[i]
	}
	path := []layout.Vector{{100, 116}, {80, 150}, {100, 184}}
	existing.Path = slices.Clone(path)
	existing.Label, existing.LabelPos = "label", layout.Vector{80, 150}
	if err := layout.Force(graph); err != nil {
		t.Fatal(err)
	}
	for i, node := range graph.Nodes {
		if node.Center != centers[i] {
			t.Errorf("node %s moved to %v", node.ID, node.Center)
		}
	}
	if !slices.Equal(existing.Path, path) || existing.LabelPos != (layout.Vector{80, 150}) {
		t.Errorf("existing path or label moved: %v, %v", existing.Path, existing.LabelPos)
	}
	if len(missing.Path) != 2 {
		t.Fatalf("missing path was not computed: %v", missing.Path)
	}
}

// TestForceClusterBoxes checks that force layouts box their clusters
// around their nodes, and nested clusters inside their parent.
func TestForceClusterBoxes(t *testing.T) {
	graph := layout.NewDigraph()
	graph.Edge("a", "b")
	graph.Edge("b", "c")
	outer := &layout.Cluster{ID: "outer", Label: "outer", Nodes: []*layout.Node{graph.Node("a"), graph.Node("b"), graph.Node("c")}}
	inner := &layout.Cluster{ID: "inner", Label: "inner", Nodes: []*layout.Node{graph.Node("a"), graph.Node("b")}, Parent: outer}
	graph.Clusters = []*layout.Cluster{outer, inner}
	if err := layout.Force(graph); err != nil {
		t.Fatal(err)
	}
	inside := func(tl, br, otl, obr layout.Vector) bool {
		return otl.X <= tl.X && otl.Y <= tl.Y && br.X <= obr.X && br.Y <= obr.Y
	}
	for _, cluster := range graph.Clusters {
		if cluster.BottomRight.X <= cluster.TopLeft.X || cluster.BottomRight.Y <= cluster.TopLeft.Y {
			t.Fatalf("%s has no box: %v %v", cluster.ID, cluster.TopLeft, cluster.BottomRight)
		}
		for _, node := range cluster.Nodes {
			if !inside(node.TopLeft(), node.BottomRight(), cluster.TopLeft, cluster.BottomRight) {
				t.Errorf("%s is outside %s", node.ID, cluster.ID)
			}
		}
	}
	if !inside(inner.TopLeft, inner.BottomRight, outer.TopLeft, outer.BottomRight) {
		t.Errorf("inner %v %v is not inside outer %v %v", inner.TopLeft, inner.BottomRight, outer.TopLeft, outer.BottomRight)
	}
}

// TestForceParallelEdges checks that edges between the same nodes take
// routes of their own, with labels apart, both ways and the same way.
func TestForceParallelEdges(t *testing.T) {
	for name, back := range map[string]bool{"opposite": true, "same": false} {
		graph := layout.NewDigraph()
		first := layout.NewEdge(graph.Node("a"), graph.Node("b"))
		first.Label = "forward"
		second := layout.NewEdge(graph.Node("a"), graph.Node("b"))
		if back {
			second = layout.NewEdge(graph.Node("b"), graph.Node("a"))
		}
		second.Label = "back"
		graph.AddEdge(first)
		graph.AddEdge(second)
		if err := layout.Force(graph); err != nil {
			t.Fatal(err)
		}
		a, b := first.Path, slices.Clone(second.Path)
		if back {
			slices.Reverse(b)
		}
		if slices.Equal(a, b) {
			t.Errorf("%s: edges share the route %v", name, a)
		}
		la, lb := first, second
		if math.Abs(float64(la.LabelPos.X-lb.LabelPos.X)) < float64(la.LabelRadius.X+lb.LabelRadius.X) &&
			math.Abs(float64(la.LabelPos.Y-lb.LabelPos.Y)) < float64(la.LabelRadius.Y+lb.LabelRadius.Y) {
			t.Errorf("%s: labels at %v and %v overlap", name, la.LabelPos, lb.LabelPos)
		}
	}
}
