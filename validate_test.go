package layout_test

import (
	"math"
	"testing"

	"github.com/loov/layout"
)

// TestValidateNumbers checks that layouts reject numbers that would turn
// into non-finite coordinates or runaway ranks.
func TestValidateNumbers(t *testing.T) {
	nan, inf := layout.Length(math.NaN()), layout.Length(math.Inf(1))
	for name, spoil := range map[string]func(*layout.Graph){
		"line height":   func(g *layout.Graph) { g.LineHeight = nan },
		"font size":     func(g *layout.Graph) { g.FontSize = inf },
		"node padding":  func(g *layout.Graph) { g.NodePadding = nan },
		"row padding":   func(g *layout.Graph) { g.RowPadding = inf },
		"edge padding":  func(g *layout.Graph) { g.EdgePadding = nan },
		"node radius":   func(g *layout.Graph) { g.Nodes[0].Radius.X = inf },
		"node font":     func(g *layout.Graph) { g.Nodes[0].FontSize = nan },
		"node line":     func(g *layout.Graph) { g.Nodes[0].LineWidth = inf },
		"pinned center": func(g *layout.Graph) { g.Pinned = true; g.Nodes[0].Center.Y = nan },
		"pinned path":   func(g *layout.Graph) { g.Pinned = true; g.Edges[0].Path = []layout.Vector{{0, 0}, {inf, 0}} },
		"pinned label":  func(g *layout.Graph) { g.Pinned = true; g.Edges[0].LabelPos.X = nan },
		"pinned cluster": func(g *layout.Graph) {
			g.Pinned = true
			g.Clusters = []*layout.Cluster{{Nodes: g.Nodes, TopLeft: layout.Vector{-inf, -inf}, BottomRight: layout.Vector{inf, inf}}}
		},
		"edge weight": func(g *layout.Graph) { g.Edges[0].Weight = math.NaN() },
		"edge font":   func(g *layout.Graph) { g.Edges[0].FontSize = inf },
		"edge line":   func(g *layout.Graph) { g.Edges[0].LineWidth = nan },
		"edge minlen": func(g *layout.Graph) { g.Edges[0].MinLen = 100000000 },
	} {
		for _, lay := range []func(*layout.Graph) error{layout.Hierarchical, layout.Force} {
			graph := layout.NewDigraph()
			graph.Edge("a", "b")
			spoil(graph)
			if err := lay(graph); err == nil {
				t.Errorf("%s: no error", name)
			}
		}
	}
}
