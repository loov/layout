package layout

import (
	"fmt"
	"math"
)

// maxMinLen is the largest Edge.MinLen accepted; every rank an edge
// spans costs a virtual node
const maxMinLen = 1000

// finite reports whether none of values is NaN or infinite
func finite(values ...Length) bool {
	for _, v := range values {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return false
		}
	}
	return true
}

// validate checks that every edge connects nodes of the graph and that
// the sizes, and the positions a pinned layout keeps, are finite
func (graph *Graph) validate() error {
	if !finite(graph.LineHeight, graph.FontSize, graph.NodePadding, graph.RowPadding, graph.EdgePadding) {
		return fmt.Errorf("graph line height, font size or padding is not finite")
	}
	known := make(map[*Node]bool, len(graph.Nodes))
	for i, node := range graph.Nodes {
		switch {
		case node == nil:
			return fmt.Errorf("node %d is nil", i)
		case !finite(node.MinSize.X, node.MinSize.Y, node.FontSize, node.LineWidth):
			return fmt.Errorf("node %v: size, font size or line width is not finite", node)
		case node.Pos != nil && !finite(node.Pos.X, node.Pos.Y):
			return fmt.Errorf("node %v: position %v is not finite", node, *node.Pos)
		}
		known[node] = true
	}
	for i, edge := range graph.Edges {
		switch {
		case edge == nil:
			return fmt.Errorf("edge %d is nil", i)
		case edge.From == nil || edge.To == nil:
			return fmt.Errorf("edge %d has a nil endpoint", i)
		case !known[edge.From]:
			return fmt.Errorf("edge %v: node %q is not in the graph", edge, edge.From)
		case !known[edge.To]:
			return fmt.Errorf("edge %v: node %q is not in the graph", edge, edge.To)
		case !finite(Length(edge.Weight), edge.FontSize, edge.LineWidth):
			return fmt.Errorf("edge %v: weight, font size or line width is not finite", edge)
		case edge.MinLen > maxMinLen:
			return fmt.Errorf("edge %v: minimum length %d is over %d", edge, edge.MinLen, maxMinLen)
		case edge.LabelPos != nil && !finite(edge.LabelPos.X, edge.LabelPos.Y):
			return fmt.Errorf("edge %v: label position %v is not finite", edge, *edge.LabelPos)
		}
		for _, p := range edge.Pos {
			if !finite(p.X, p.Y) {
				return fmt.Errorf("edge %v: path point %v is not finite", edge, p)
			}
		}
	}
	group := func(what string, nodes []*Node) error {
		if len(nodes) == 0 {
			return fmt.Errorf("%s has no nodes", what)
		}
		for _, node := range nodes {
			if node == nil || !known[node] {
				return fmt.Errorf("%s: node %v is not in the graph", what, node)
			}
		}
		return nil
	}
	for i, nodes := range graph.SameRank {
		if err := group(fmt.Sprintf("same rank group %d", i), nodes); err != nil {
			return err
		}
	}
	for _, pinned := range []struct {
		what  string
		nodes []*Node
	}{{"min rank", graph.MinRank}, {"max rank", graph.MaxRank}} {
		if len(pinned.nodes) == 0 {
			continue
		}
		if err := group(pinned.what, pinned.nodes); err != nil {
			return err
		}
	}
	clusters := make(map[*Cluster]bool, len(graph.Clusters))
	for i, cluster := range graph.Clusters {
		if cluster == nil {
			return fmt.Errorf("cluster %d is nil", i)
		}
		clusters[cluster] = true
	}
	for _, cluster := range graph.Clusters {
		if err := group(fmt.Sprintf("cluster %q", cluster.ID), cluster.Nodes); err != nil {
			return err
		}
		depth := 0
		for parent := cluster.Parent; parent != nil; parent = parent.Parent {
			if !clusters[parent] {
				return fmt.Errorf("cluster %q: parent %q is not in the graph", cluster.ID, parent.ID)
			}
			if depth++; depth > len(graph.Clusters) {
				return fmt.Errorf("cluster %q: parent chain is a cycle", cluster.ID)
			}
		}
	}
	return nil
}
