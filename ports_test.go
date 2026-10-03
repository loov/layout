package layout_test

import (
	"math"
	"testing"

	"github.com/loov/layout"
)

func TestHierarchicalCompassPortsKeepPhysicalDirections(t *testing.T) {
	for _, dir := range []layout.RankDir{layout.TopToBottom, layout.LeftToRight, layout.BottomToTop, layout.RightToLeft} {
		for _, port := range []layout.Compass{layout.North, layout.NorthEast, layout.East, layout.SouthEast, layout.South, layout.SouthWest, layout.West, layout.NorthWest, layout.Center} {
			t.Run(string(dir)+"/"+string(port), func(t *testing.T) {
				g := layout.NewDigraph()
				g.RankDir = dir
				e := g.Edge("a", "b")
				e.From.MinSize = layout.Vector{X: 60, Y: 30}
				e.To.MinSize = layout.Vector{X: 40, Y: 20}
				e.FromPort, e.ToPort = port, port
				for range 2 {
					l, err := layout.Hierarchical(g, layout.Options{})
					if err != nil {
						t.Fatal(err)
					}
					assertPortEndpoints(t, l, e)
					if e.FromPort != port || e.ToPort != port {
						t.Fatal("layout changed port attributes")
					}
				}
			})
		}
	}
}

func assertPortEndpoints(t *testing.T, l *layout.Layout, e *layout.Edge) {
	t.Helper()
	path := l.Edge(e).Path
	if len(path) < 2 {
		t.Fatalf("missing path: %v", path)
	}
	for _, end := range []struct{ got, want layout.Vector }{
		{path[0], l.Node(e.From).CompassPoint(e.FromPort)},
		{path[len(path)-1], l.Node(e.To).CompassPoint(e.ToPort)},
	} {
		if math.Hypot(float64(end.got.X-end.want.X), float64(end.got.Y-end.want.Y)) > 1e-6 {
			t.Errorf("endpoint = %v, want %v", end.got, end.want)
		}
	}
}

func TestHierarchicalRestoresPortsOfRepeatedEdge(t *testing.T) {
	g := layout.NewDigraph()
	g.RankDir = layout.LeftToRight
	e := g.Edge("a", "b")
	g.Edges = append(g.Edges, e)
	e.FromPort = layout.East
	if _, err := layout.Hierarchical(g, layout.Options{}); err != nil {
		t.Fatal(err)
	}
	if e.FromPort != layout.East {
		t.Fatalf("FromPort = %q, want %q", e.FromPort, layout.East)
	}
}

func TestStraightLineRoutingPreservesExplicitPorts(t *testing.T) {
	for _, dir := range []layout.RankDir{layout.TopToBottom, layout.LeftToRight, layout.BottomToTop, layout.RightToLeft} {
		t.Run(string(dir), func(t *testing.T) {
			g := layout.NewDigraph()
			g.RankDir, g.Splines = dir, layout.SplinesLine
			e := g.Edge("a", "b")
			e.MinLen, e.FromPort, e.ToPort = 2, layout.West, layout.East
			l, err := layout.Hierarchical(g, layout.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if n := len(l.Edge(e).Path); n != 2 {
				t.Fatalf("straight path has %d points", n)
			}
			assertPortEndpoints(t, l, e)
		})
	}
}

func TestPinnedLayoutsHonorPorts(t *testing.T) {
	for name, run := range algorithms {
		t.Run(name, func(t *testing.T) {
			g := layout.NewDigraph()
			e := g.Edge("a", "b")
			e.From.Pos, e.To.Pos = &layout.Vector{X: 100, Y: 100}, &layout.Vector{X: 100, Y: 200}
			e.FromPort, e.ToPort = layout.East, layout.West
			l, err := run(g)
			if err != nil {
				t.Fatal(err)
			}
			assertPortEndpoints(t, l, e)
		})
	}
}

func TestSelfLoopsHonorPortsAndAvoidTheNode(t *testing.T) {
	ports := []layout.Compass{layout.CompassAuto, layout.North, layout.NorthEast, layout.East, layout.SouthEast, layout.South, layout.SouthWest, layout.West, layout.NorthWest}
	for _, dir := range []layout.RankDir{layout.TopToBottom, layout.LeftToRight, layout.BottomToTop, layout.RightToLeft, "pinned"} {
		for _, from := range ports {
			for _, to := range ports {
				g := layout.NewDigraph()
				e := g.Edge("a", "a")
				e.From.Shape = layout.Box
				e.FromPort, e.ToPort = from, to
				if dir == "pinned" {
					e.From.Pos = &layout.Vector{X: 100, Y: 100}
				} else {
					g.RankDir = dir
				}
				l, err := layout.Hierarchical(g, layout.Options{})
				if err != nil {
					t.Fatal(err)
				}
				name := string(dir) + "/" + string(from) + "-" + string(to)
				n, path := l.Node(e.From), l.Edge(e).Path
				if want := n.CompassPoint(from); from != layout.CompassAuto && path[0] != want {
					t.Errorf("%s: start %v, want %v", name, path[0], want)
				}
				if want := n.CompassPoint(to); to != layout.CompassAuto && path[len(path)-1] != want {
					t.Errorf("%s: end %v, want %v", name, path[len(path)-1], want)
				}
				for i := 0; i+1 < len(path); i++ {
					for k := 1; k < 20; k++ {
						f := layout.Length(k) / 20
						d := path[i+1].Sub(path[i])
						p := path[i].Add(layout.Vector{X: d.X * f, Y: d.Y * f})
						if p.X > n.Left()+0.01 && p.X < n.Right()-0.01 && p.Y > n.Top()+0.01 && p.Y < n.Bottom()-0.01 {
							t.Errorf("%s: segment %d passes through the node at %v: %v", name, i, p, path)
							break
						}
					}
				}
			}
		}
	}
}

func TestSelfLoopLabelFollowsPortedLoop(t *testing.T) {
	g := layout.NewDigraph()
	e := g.Edge("a", "a")
	e.Label = "loop"
	e.FromPort, e.ToPort = layout.NorthWest, layout.SouthWest
	l, err := layout.Hierarchical(g, layout.Options{})
	if err != nil {
		t.Fatal(err)
	}
	path, node := l.Edge(e), l.Node(e.From)
	if path.LabelCenter.X+path.LabelSize.X/2 > node.Left() {
		t.Fatalf("label at %v is not left of the node at %v", path.LabelCenter, node.Center)
	}
}
