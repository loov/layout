package glay

/*
func AssignPositions(graph *Graph) {
	const padding = float32(10.0)

	top := padding
	for _, nodes := range graph.ByRank {
		var rowheight float32
		for _, id := range nodes {
			n := &graph.Nodes[id]
			rowheight = maxf32(rowheight, n.HalfSize.Y*2)
		}

		left := padding
		for _, id := range nodes {
			n := &graph.Nodes[id]
			n.Center.Y = top + rowheight/2
			n.Center.X = left + n.HalfSize.X
			left += n.HalfSize.X*2 + padding
		}

		top += rowheight + padding
	}

	for eid := range graph.Edges {
		e := &graph.Edges[eid]
		s, d := &graph.Nodes[e.Source], &graph.Nodes[e.Destination]

		e.Center = s.Center.Add(d.Center).Scale(0.5)
	}
}

*/
