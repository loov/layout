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
	l, err := layout.Force(graph, layout.ForceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, a := range graph.Nodes {
		ab := l.Nodes[i]
		if ab.Left() < 0 || ab.Top() < 0 {
			t.Errorf("%v is outside the drawing at %v", a, ab.Center)
		}
		for j, b := range graph.Nodes {
			if a == b {
				continue
			}
			bb := l.Nodes[j]
			d := math.Hypot(float64(ab.Center.X-bb.Center.X), float64(ab.Center.Y-bb.Center.Y))
			if d < float64(ab.Size.X+bb.Size.X)/2 {
				t.Errorf("%v and %v overlap, distance %.1f", a, b, d)
			}
		}
	}
	for i, edge := range graph.Edges {
		if len(l.Edges[i].Path) != 2 {
			t.Errorf("%v has no path", edge)
		}
	}
	var got bytes.Buffer
	if err := svg.Write(&got, l); err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "testdata/force.svg", got.Bytes())
}

func TestForcePreservesPinnedCentersAndPaths(t *testing.T) {
	graph := layout.NewGraph()
	existing := graph.Edge("a", "b")
	missing := graph.Edge("b", "c")
	centers := []layout.Vector{{100, 100}, {100, 200}, {200, 200}}
	for i, node := range graph.Nodes {
		node.Pos = &centers[i]
	}
	path := []layout.Vector{{100, 116}, {80, 150}, {100, 184}}
	existing.Pos = slices.Clone(path)
	existing.Label, existing.LabelPos = "label", &layout.Vector{80, 150}
	l, err := layout.Force(graph, layout.ForceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, node := range graph.Nodes {
		if l.Nodes[i].Center != centers[i] {
			t.Errorf("node %s moved to %v", node.ID, l.Nodes[i].Center)
		}
	}
	if got := l.Edge(existing); !slices.Equal(got.Path, path) || got.LabelCenter != (layout.Vector{80, 150}) {
		t.Errorf("existing path or label moved: %v, %v", got.Path, got.LabelCenter)
	}
	if got := l.Edge(missing).Path; len(got) != 2 {
		t.Fatalf("missing path was not computed: %v", got)
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
	l, err := layout.Force(graph, layout.ForceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	inside := func(tl, br, otl, obr layout.Vector) bool {
		return otl.X <= tl.X && otl.Y <= tl.Y && br.X <= obr.X && br.Y <= obr.Y
	}
	for _, cluster := range graph.Clusters {
		box := l.Cluster(cluster)
		if box.BottomRight.X <= box.TopLeft.X || box.BottomRight.Y <= box.TopLeft.Y {
			t.Fatalf("%s has no box: %v %v", cluster.ID, box.TopLeft, box.BottomRight)
		}
		for _, node := range cluster.Nodes {
			if n := l.Node(node); !inside(n.TopLeft(), n.BottomRight(), box.TopLeft, box.BottomRight) {
				t.Errorf("%s is outside %s", node.ID, cluster.ID)
			}
		}
	}
	in, out := l.Cluster(inner), l.Cluster(outer)
	if !inside(in.TopLeft, in.BottomRight, out.TopLeft, out.BottomRight) {
		t.Errorf("inner %v %v is not inside outer %v %v", in.TopLeft, in.BottomRight, out.TopLeft, out.BottomRight)
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
		l, err := layout.Force(graph, layout.ForceOptions{})
		if err != nil {
			t.Fatal(err)
		}
		la, lb := l.Edge(first), l.Edge(second)
		a, b := la.Path, slices.Clone(lb.Path)
		if back {
			slices.Reverse(b)
		}
		if slices.Equal(a, b) {
			t.Errorf("%s: edges share the route %v", name, a)
		}
		if math.Abs(float64(la.LabelCenter.X-lb.LabelCenter.X)) < float64(la.LabelSize.X+lb.LabelSize.X)/2 &&
			math.Abs(float64(la.LabelCenter.Y-lb.LabelCenter.Y)) < float64(la.LabelSize.Y+lb.LabelSize.Y)/2 {
			t.Errorf("%s: labels at %v and %v overlap", name, la.LabelCenter, lb.LabelCenter)
		}
	}
}
