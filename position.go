package layout

const (
	nodeWidth  = float32(15)
	nodeLeft   = nodeWidth * 0.5
	nodeRight  = nodeWidth * 0.5
	padding    = float32(10)
	rowpadding = float32(30)
)

func Position(graph *Graph) {
	graph.Positions = make([]Vector, len(graph.Nodes))
	Position_Initial_LeftToRight(graph)

	power := float32(1.0)
	for i := 0; i < 10000; i++ {
		Position_Improve_Median_Incoming(graph, power)
		Position_Improve_Median_Outgoing(graph, power)
		Position_Improve_Median_Average(graph, power)

		power *= 0.999
		if power < 0.01 {
			break
		}

		Position_Flush_Left(graph)
	}
}

func Position_Initial_LeftToRight(graph *Graph) {
	top := float32(rowpadding)
	for _, nodes := range graph.ByRank {
		left := float32(0)
		for _, id := range nodes {
			graph.Positions[id].X = left + nodeLeft
			graph.Positions[id].Y = top + nodeWidth/2
			left += nodeLeft + nodeRight + padding
		}
		top += nodeWidth + rowpadding
	}
}

func Position_Improve_Median_Incoming(graph *Graph, power float32) {
	for _, layer := range graph.ByRank {
		for i, nid := range layer {
			node := graph.Nodes[nid]

			var wallLeft, wallRight float32
			if i > 0 {
				wallLeft = graph.Positions[layer[i-1]].X + nodeRight + padding
			}
			if i+1 < len(layer) {
				wallRight = graph.Positions[layer[i+1]].X - nodeLeft - padding
			} else {
				wallRight = float32(len(graph.Nodes)) * nodeWidth * 4
			}

			// ensure we are not overlapping with the previous
			if graph.Positions[nid].X < wallLeft+nodeLeft {
				graph.Positions[nid].X = wallLeft + nodeLeft
			}

			// do we have something to use for placement?
			if len(node.In) == 0 {
				continue
			}

			// calculate average location based on incoming
			center := float32(0.0)
			for _, oid := range node.In {
				center += graph.Positions[oid].X
			}
			center /= float32(len(node.In))

			// is between sides
			if wallLeft+nodeLeft < center && center < wallRight-nodeRight {
				graph.Positions[nid].X = graph.Positions[nid].X*(1-power) + center*power
				continue
			}
		}
	}
}

func Position_Improve_Median_Outgoing(graph *Graph, power float32) {
	for k := len(graph.ByRank) - 1; k >= 0; k-- {
		layer := graph.ByRank[k]

		for i, nid := range layer {
			node := graph.Nodes[nid]

			var wallLeft, wallRight float32
			if i > 0 {
				wallLeft = graph.Positions[layer[i-1]].X + nodeRight + padding
			}
			if i+1 < len(layer) {
				wallRight = graph.Positions[layer[i+1]].X - nodeLeft - padding
			} else {
				wallRight = float32(len(graph.Nodes)) * nodeWidth * 4
			}

			// ensure we are not overlapping with the previous
			if graph.Positions[nid].X-nodeLeft < wallLeft {
				graph.Positions[nid].X = wallLeft + nodeLeft
			}

			// do we have something to use for placement?
			if len(node.Out) == 0 {
				continue
			}

			// calculate average location based on outgoing
			center := float32(0.0)
			for _, oid := range node.Out {
				center += graph.Positions[oid].X
			}
			center /= float32(len(node.Out))

			// is between sides
			if wallLeft+nodeLeft < center && center < wallRight-nodeRight {
				graph.Positions[nid].X = graph.Positions[nid].X*(1-power) + center*power
				continue
			}
		}
	}
}

func Position_Improve_Median_Average(graph *Graph, power float32) {
	for _, layer := range graph.ByRank {
		for i, nid := range layer {
			node := graph.Nodes[nid]

			var wallLeft, wallRight float32
			if i > 0 {
				wallLeft = graph.Positions[layer[i-1]].X + nodeRight + padding
			}
			if i+1 < len(layer) {
				wallRight = graph.Positions[layer[i+1]].X - nodeLeft - padding
			} else {
				wallRight = float32(len(graph.Nodes)) * nodeWidth * 4
			}

			// ensure we are not overlapping with the previous
			if graph.Positions[nid].X < wallLeft+nodeLeft {
				graph.Positions[nid].X = wallLeft + nodeLeft
			}

			// do we have something to use for placement?
			if len(node.In) == 0 && len(node.Out) == 0 {
				continue
			}

			// calculate average location based on incoming
			center := float32(0.0)
			for _, oid := range node.In {
				center += graph.Positions[oid].X
			}
			for _, oid := range node.Out {
				center += graph.Positions[oid].X
			}
			center /= float32(len(node.In) + len(node.Out))

			// is between sides
			if wallLeft+nodeLeft < center && center < wallRight-nodeRight {
				graph.Positions[nid].X = graph.Positions[nid].X*(1-power) + center*power
				continue
			}
		}
	}
}

func Position_Flush_Left(graph *Graph) {
	minleft := graph.Positions[0].X - nodeLeft
	for _, node := range graph.Nodes {
		if graph.Positions[node.ID].X-nodeLeft < minleft {
			minleft = graph.Positions[node.ID].X - nodeLeft
		}
	}
	if minleft < 0 {
		return
	}

	for _, node := range graph.Nodes {
		graph.Positions[node.ID].X -= minleft
	}
}
