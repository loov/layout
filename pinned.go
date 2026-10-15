package layout

import (
	"slices"
)

// layoutPinned keeps node positions and gives edges without a path a
// straight line, labels without a position the middle of their path.
func layoutPinned(graph *lgraph) {
	boxClusters(graph)
	loops := countLoops(graph.Edges)
	given := map[*ledge]bool{} // label positions that stay
	for _, edge := range graph.Edges {
		given[edge] = !edge.LabelPos.IsZero()
	}
	defer nudgeLabels(graph.Edges, graph.Nodes, graph.Clusters, graph.EdgePadding, 2*graph.RowPadding, given)
	pairs := newPairs(graph)
	for _, edge := range graph.Edges {
		if len(edge.Path) < 2 {
			if edge.loop() {
				edge.Path = loopPath(edge, edge.From.Radius.X, edge.From.Radius.X, loops.next(edge), loops.count[edge.From])
			} else {
				// edges between the same nodes run side by side,
				// shifted across the line between the nodes
				shift := pairs.shift(edge, 2*graph.EdgePadding)
				from := edge.From.outlineAlong(edge.From.Center.Add(shift), edge.To.Center.Add(shift))
				to := edge.To.outlineAlong(edge.To.Center.Add(shift), edge.From.Center.Add(shift))
				if edge.FromPort != CompassAuto {
					from = edge.From.CompassPoint(edge.FromPort)
				}
				if edge.ToPort != CompassAuto {
					to = edge.To.CompassPoint(edge.ToPort)
				}
				edge.Path = []Vector{from, to}
			}
		}
		if edge.Label != "" && edge.LabelPos.IsZero() {
			edge.LabelPos = edge.Path.middle().Add(Vector{edge.LabelRadius.X + graph.EdgePadding, 0})
		}
	}
}

// boxClusters boxes clusters around their nodes and nested clusters,
// padded, with a strip for the label on top and room for its width, for
// layouts that place nodes without making room for clusters.
func boxClusters(graph *lgraph) {
	pad := max(graph.EdgePadding, graph.RowPadding/2)
	byDepth := slices.Clone(graph.Clusters)
	slices.SortStableFunc(byDepth, func(a, b *lcluster) int { return b.depth() - a.depth() })
	for _, cluster := range byDepth {
		b := emptyRect()
		for _, node := range cluster.Nodes {
			b.add(node.rect())
		}
		for _, inner := range graph.Clusters {
			if inner.Parent == cluster {
				b.add(inner.rect())
			}
		}
		if b.empty() {
			continue // nothing inside
		}
		b = b.grow(pad)
		tl, br := b.Min, b.Max
		if cluster.Label != "" {
			label := graph.textRadius(cluster.Label, "", graph.FontSize)
			tl.Y -= 2 * label.Y
			if grow := 2*(label.X+graph.EdgePadding) - (br.X - tl.X); grow > 0 {
				tl.X, br.X = tl.X-grow/2, br.X+grow/2
			}
		}
		cluster.TopLeft, cluster.BottomRight = tl, br
	}
}
