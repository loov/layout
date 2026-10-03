package layout_test

import (
	"github.com/loov/layout"
	"math"
	"testing"
)

func TestHierarchicalCompassPortsKeepPhysicalDirections(t *testing.T) {
	for _, dir := range []layout.RankDir{layout.TopToBottom, layout.LeftToRight, layout.BottomToTop, layout.RightToLeft} {
		for _, port := range []layout.Compass{layout.North, layout.NorthEast, layout.East, layout.SouthEast, layout.South, layout.SouthWest, layout.West, layout.NorthWest, layout.Center} {
			t.Run(string(dir)+"/"+string(port), func(t *testing.T) {
				g := layout.NewDigraph()
				g.RankDir = dir
				e := g.Edge("a", "b")
				e.From.Radius = layout.Vector{X: 30, Y: 15}
				e.To.Radius = layout.Vector{X: 20, Y: 10}
				e.FromPort, e.ToPort = port, port
				for range 2 {
					if err := layout.Hierarchical(g); err != nil {
						t.Fatal(err)
					}
					assertPortEndpoints(t, e)
					if e.FromPort != port || e.ToPort != port {
						t.Fatal("layout changed port attributes")
					}
				}
			})
		}
	}
}

func assertPortEndpoints(t *testing.T, e *layout.Edge) {
	t.Helper()
	if len(e.Path) < 2 {
		t.Fatalf("missing path: %v", e.Path)
	}
	for _, end := range []struct{ got, want layout.Vector }{
		{e.Path[0], e.From.CompassPoint(e.FromPort)},
		{e.Path[len(e.Path)-1], e.To.CompassPoint(e.ToPort)},
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
	if err := layout.Hierarchical(g); err != nil {
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
			if err := layout.Hierarchical(g); err != nil {
				t.Fatal(err)
			}
			if len(e.Path) != 2 {
				t.Fatalf("straight path has %d points", len(e.Path))
			}
			assertPortEndpoints(t, e)
		})
	}
}

func TestPinnedLayoutsHonorPorts(t *testing.T) {
	for name, run := range map[string]func(*layout.Graph) error{"hierarchical": layout.Hierarchical, "force": layout.Force} {
		t.Run(name, func(t *testing.T) {
			g := layout.NewDigraph()
			g.Pinned = true
			e := g.Edge("a", "b")
			e.From.Center, e.To.Center = layout.Vector{X: 100, Y: 100}, layout.Vector{X: 100, Y: 200}
			e.FromPort, e.ToPort = layout.East, layout.West
			if err := run(g); err != nil {
				t.Fatal(err)
			}
			assertPortEndpoints(t, e)
		})
	}
}
