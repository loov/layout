package layout

const (
	nodesize   = float32(15)
	nodeleft   = nodesize * 0.5
	noderight  = nodesize * 0.5
	padding    = float32(10)
	rowpadding = float32(30)
)

func Position(graph *Graph) {
	graph.Positions = make([]Vector, len(graph.Nodes))
	Position_Initial_LeftToRight(graph)
	Position_Improve_Median_Incoming(graph, 1)
	Position_Improve_Median_Outgoing(graph, 1)
	return
	power := float32(1.0)
	for i := 0; i < 1000; i++ {
		if i%2 == 0 {
			Position_Improve_Median_Incoming(graph, power)
		} else {
			Position_Improve_Median_Outgoing(graph, power)
		}
		//Position_Improve_Median_Average(graph, power)

		power *= 0.9999
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
			graph.Positions[id].X = left + nodeleft
			graph.Positions[id].Y = top + nodesize/2
			left += nodeleft + noderight + padding
		}
		top += nodesize + rowpadding
	}
}

func Position_Improve_Median_Incoming(graph *Graph, power float32) {
	for _, layer := range graph.ByRank {
		wall := float32(0)

		for i, nid := range layer {
			node := graph.Nodes[nid]

			var wallLeft, wallRight float32
			if i > 0 {
				wallLeft = graph.Positions[layer[i-1]].X + noderight + padding
			}

			if i+1 < len(layer) {
				wallRight = graph.Positions[layer[i+1]].X - nodeleft - padding
			} else {
				wallRight = float32(len(graph.Nodes)) * nodesize
			}
			_, _ = wallLeft, wallRight

			// ensure we are not overlapping with the previous
			if graph.Positions[nid].X-nodeleft < wall {
				graph.Positions[nid].X = wall + nodeleft
			}

			// do we have something to use for placement?
			if len(node.In) == 0 {
				// don't move node
				wall = graph.Positions[nid].X + noderight + padding
				continue
			}

			// calculate average location based on incoming
			total := float32(0.0)
			for _, oid := range node.In {
				total += graph.Positions[oid].X
			}
			total /= float32(len(node.In))

			// is it before wall
			if total-nodeleft < wall {
				// move to wall
				graph.Positions[nid].X = wall + nodeleft
				wall = graph.Positions[nid].X + noderight + padding
				continue
			}

			graph.Positions[nid].X = graph.Positions[nid].X*(1-power) + total*power
			wall = graph.Positions[nid].X + noderight + padding
		}
	}
}

func Position_Improve_Median_Outgoing(graph *Graph, power float32) {
	for i := len(graph.ByRank) - 1; i >= 0; i-- {
		layer := graph.ByRank[i]
		wall := float32(0)

		for _, nid := range layer {
			node := graph.Nodes[nid]

			// ensure we are not overlapping with the previous
			if graph.Positions[nid].X-nodeleft < wall {
				graph.Positions[nid].X = wall + nodeleft
			}

			// do we have something to use for placement?
			if len(node.Out) == 0 {
				// don't move node
				wall = graph.Positions[nid].X + noderight + padding
				continue
			}

			// calculate average location based on incoming
			total := float32(0.0)
			for _, oid := range node.Out {
				total += graph.Positions[oid].X
			}
			total /= float32(len(node.Out))

			// is it before wall
			if total-nodeleft < wall {
				// move to wall
				graph.Positions[nid].X = wall + nodeleft
				wall = graph.Positions[nid].X + noderight + padding
				continue
			}

			graph.Positions[nid].X = graph.Positions[nid].X*(1-power) + total*power
			wall = graph.Positions[nid].X + noderight + padding
		}
	}
}

func Position_Improve_Median_Average(graph *Graph, power float32) {
	for i := len(graph.ByRank) - 1; i >= 0; i-- {
		layer := graph.ByRank[i]
		wall := float32(0)

		for _, nid := range layer {
			node := graph.Nodes[nid]

			// ensure we are not overlapping with the previous
			if graph.Positions[nid].X-nodeleft < wall {
				graph.Positions[nid].X = wall + nodeleft
			}

			// do we have something to use for placement?
			if len(node.Out) == 0 {
				// don't move node
				wall = graph.Positions[nid].X + noderight + padding
				continue
			}

			// calculate average location based on incoming
			total := float32(0.0)
			for _, oid := range node.Out {
				total += graph.Positions[oid].X
			}
			for _, oid := range node.In {
				total += graph.Positions[oid].X
			}
			total /= float32(len(node.Out) + len(node.In))

			// is it before wall
			if total-nodeleft < wall {
				// move to wall
				graph.Positions[nid].X = wall + nodeleft
				wall = graph.Positions[nid].X + noderight + padding
				continue
			}

			graph.Positions[nid].X = graph.Positions[nid].X*(1-power) + total*power
			wall = graph.Positions[nid].X + noderight + padding
		}
	}
}

func Position_Flush_Left(graph *Graph) {
	minleft := graph.Positions[0].X - nodeleft
	for _, node := range graph.Nodes {
		if graph.Positions[node.ID].X-nodeleft < minleft {
			minleft = graph.Positions[node.ID].X - nodeleft
		}
	}
	if minleft < 0 {
		return
	}

	for _, node := range graph.Nodes {
		graph.Positions[node.ID].X -= minleft
	}
}

func Position_Improve_Median(graph *Graph, down bool) {
	if down {
		for _, nodes := range graph.ByRank {
			left := Vector{}
			for _, nid := range nodes {
				node := graph.Nodes[nid]
				if len(node.In) == 0 {
					left.X = graph.Positions[nid].X
					continue
				}

				total := Vector{}
				for _, oid := range node.In {
					total.X += graph.Positions[oid].X
				}
				total.X /= float32(len(node.In))
				if total.X < left.X+nodesize+padding {
					left.X = graph.Positions[nid].X
					continue
				}
				graph.Positions[nid].X = total.X
				left.X = graph.Positions[nid].X
			}
		}
	} else {
		for _, nodes := range graph.ByRank {
			left := Vector{}
			for _, nid := range nodes {
				node := graph.Nodes[nid]
				if len(node.Out) == 0 {
					left.X = graph.Positions[nid].X
					continue
				}

				total := Vector{}
				for _, oid := range node.Out {
					total.X += graph.Positions[oid].X
				}
				total.X /= float32(len(node.Out))
				if total.X < left.X+nodesize+padding {
					left.X = graph.Positions[nid].X
					continue
				}
				graph.Positions[nid].X = total.X
				left.X = graph.Positions[nid].X
			}
		}
	}
}
