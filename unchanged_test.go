package layout_test

import (
	"reflect"
	"testing"

	"github.com/loov/layout"
)

// TestLayoutLeavesGraphUnchanged checks that layout only reads the graph:
// defaults, text preparation and computed sizes go into the Layout.
func TestLayoutLeavesGraphUnchanged(t *testing.T) {
	build := func() *layout.Graph {
		graph := layout.NewDigraph()
		graph.RankDir = layout.LeftToRight
		graph.Edge("a", "b").Label = "label"
		graph.Edge("b", "c").FromPort = layout.East
		graph.Edge("c", "c")
		graph.Node("a").Shape = layout.Circle
		graph.Node("b").Shape = layout.Record
		graph.Node("b").Label = "x|{y|z}"
		graph.Node("c").Peripheries = 2
		graph.Clusters = []*layout.Cluster{{ID: "c", Nodes: []*layout.Node{graph.Node("b"), graph.Node("c")}}}
		graph.SameRank = [][]*layout.Node{{graph.Node("a")}}
		return graph
	}
	snapshot := func(graph *layout.Graph) any {
		var nodes []layout.Node
		for _, node := range graph.Nodes {
			nodes = append(nodes, *node)
		}
		var edges []layout.Edge
		for _, edge := range graph.Edges {
			edges = append(edges, *edge)
		}
		var clusters []layout.Cluster
		for _, cluster := range graph.Clusters {
			clusters = append(clusters, *cluster)
		}
		settings := *graph
		settings.Nodes, settings.Edges, settings.Clusters, settings.NodeByID = nil, nil, nil, nil
		return []any{settings, nodes, edges, clusters}
	}
	for name, lay := range map[string]func(*layout.Graph) (*layout.Layout, error){
		"hierarchical": func(g *layout.Graph) (*layout.Layout, error) {
			return layout.Hierarchical(g, layout.Options{ForText: true})
		},
		"force": func(g *layout.Graph) (*layout.Layout, error) {
			return layout.Force(g, layout.ForceOptions{ForText: true})
		},
	} {
		graph := build()
		before := snapshot(graph)
		first, err := lay(graph)
		if err != nil {
			t.Fatal(err)
		}
		if after := snapshot(graph); !reflect.DeepEqual(before, after) {
			t.Errorf("%s: graph changed:\nbefore %+v\nafter  %+v", name, before, after)
		}
		second, err := lay(graph)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(first.Nodes, second.Nodes) || !reflect.DeepEqual(first.Edges, second.Edges) {
			t.Errorf("%s: laying out twice gave different results", name)
		}
	}
}
