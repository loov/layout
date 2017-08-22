package layout

import "fmt"

const (
	nodeWidth  = float32(20)
	nodeLeft   = nodeWidth * 0.5
	nodeRight  = nodeWidth * 0.5
	maxNudge   = nodeWidth * 0.5
	padding    = float32(10)
	rowpadding = float32(30)
)

func Position(graph *Graph) {
	graph.Positions = make([]Vector, len(graph.Nodes))
	Position_Initial_LeftToRight(graph)

	// TODO: fold nudge into Node parameter
	nudge := maxNudge
	for i := 0; i < 100; i++ {
		Position_Outgoing(graph, false, nudge)
		Position_Incoming(graph, false, nudge)
		Position_Outgoing(graph, true, nudge)
		Position_Incoming(graph, true, nudge)
		nudge = nudge * 0.9
		flushLeft(graph)
	}

	for i := 0; i < 10; i++ {
		Position_Incoming(graph, true, 0)
		Position_Outgoing(graph, true, 0)

		flushLeft(graph)
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
		sanityCheckLayer(graph, nodes)
		top += nodeWidth + rowpadding
	}
}

func iterateLayers(graph *Graph, leftToRight bool, dy int,
	fn func(layer NodeIDs, i int, node *Node)) {
	var starty int
	if dy < 0 {
		starty = len(graph.ByRank) - 1
	}

	if leftToRight {
		for y := starty; 0 <= y && y < len(graph.ByRank); y += dy {
			layer := graph.ByRank[y]
			for i, nid := range layer {
				fn(layer, i, graph.Nodes[nid])
			}
		}
	} else {
		for y := starty; 0 <= y && y < len(graph.ByRank); y += dy {
			layer := graph.ByRank[y]
			for i := len(layer) - 1; i >= 0; i-- {
				fn(layer, i, graph.Nodes[layer[i]])
			}
		}
	}
}

func NodeWalls(graph *Graph, layer NodeIDs, i int, node *Node, leftToRight bool) (wallLeft, wallRight float32) {
	if i > 0 {
		wallLeft = graph.Positions[layer[i-1]].X + nodeRight
	}

	if i+1 < len(layer) {
		wallRight = graph.Positions[layer[i+1]].X - nodeLeft
	} else {
		wallRight = float32(len(graph.Nodes)) * (padding + nodeLeft + nodeRight)
	}

	wallLeft += padding
	wallRight -= padding

	// ensure we can fit at least one
	if leftToRight {
		if wallRight-nodeRight < wallLeft+nodeLeft {
			wallRight = wallLeft + (nodeLeft + nodeRight)
		}
	} else {
		if wallRight-nodeRight < wallLeft+nodeLeft {
			wallLeft = wallRight - (nodeLeft + nodeRight)
		}
	}

	if leftToRight {
		if graph.Positions[node.ID].X < wallLeft+nodeLeft {
			graph.Positions[node.ID].X = wallLeft + nodeLeft
		}
	} else {
		if graph.Positions[node.ID].X > wallRight-nodeRight {
			graph.Positions[node.ID].X = wallRight - nodeRight
		}
	}

	return wallLeft, wallRight
}

func Position_Incoming(graph *Graph, leftToRight bool, nudge float32) {
	iterateLayers(graph, leftToRight, 1,
		func(layer NodeIDs, i int, node *Node) {
			wallLeft, wallRight := NodeWalls(graph, layer, i, node, leftToRight)

			// calculate average location based on incoming
			if len(node.In) == 0 {
				return
			}
			center := float32(0.0)
			for _, oid := range node.In {
				center += graph.Positions[oid].X
			}
			center /= float32(len(node.In))

			center = clampf32(center, wallLeft+nodeLeft-nudge, wallRight-nodeRight+nudge)

			// is between sides
			graph.Positions[node.ID].X = center
		})
}

func Position_Incoming_Baseline(graph *Graph, leftToRight bool, nudge float32) {
	for _, layer := range graph.ByRank {
		for i, nid := range layer {
			node := graph.Nodes[nid]

			var wallLeft, wallRight float32
			if i > 0 {
				wallLeft = graph.Positions[layer[i-1]].X + nodeRight
			}

			if i+1 < len(layer) {
				wallRight = graph.Positions[layer[i+1]].X - nodeLeft
			} else {
				wallRight = wallLeft + float32(len(graph.Nodes))*(padding+nodeLeft+nodeRight)
			}

			// add padding to the walls
			wallLeft += padding
			wallRight -= padding

			// ensure we can fit at least one
			if wallRight-nodeRight < wallLeft+nodeLeft {
				wallRight = wallLeft + nodeLeft + nodeRight
			}

			// ensure we are not overlapping with the previous
			if graph.Positions[nid].X < wallLeft+nodeLeft {
				graph.Positions[nid].X = wallLeft + nodeLeft
			}

			if len(node.In) == 0 {
				continue
			}

			// calculate average location based on incoming
			center := float32(0.0)
			for _, oid := range node.In {
				center += graph.Positions[oid].X
			}
			center /= float32(len(node.In))

			if center < wallLeft+nodeLeft-nudge {
				center = wallLeft + nodeLeft - nudge
			}
			if wallRight-nodeRight+nudge < center {
				center = wallRight - nodeRight + nudge
			}

			// is between sides
			graph.Positions[nid].X = center
		}
		sanityCheckLayer(graph, layer)
	}
}

func Position_Outgoing(graph *Graph, leftToRight bool, nudge float32) {
	iterateLayers(graph, leftToRight, -1,
		func(layer NodeIDs, i int, node *Node) {
			wallLeft, wallRight := NodeWalls(graph, layer, i, node, leftToRight)

			// calculate average location based on incoming
			if len(node.Out) == 0 {
				return
			}
			center := float32(0.0)
			for _, oid := range node.Out {
				center += graph.Positions[oid].X
			}
			center /= float32(len(node.Out))

			center = clampf32(center, wallLeft+nodeLeft-nudge, wallRight-nodeRight+nudge)

			// is between sides
			graph.Positions[node.ID].X = center
		})
}

func Position_Outgoing_Baseline(graph *Graph, leftToRight bool, nudge float32) {
	for k := len(graph.ByRank) - 1; k >= 0; k-- {
		layer := graph.ByRank[k]

		for i, nid := range layer {
			node := graph.Nodes[nid]

			var wallLeft, wallRight float32
			if i > 0 {
				wallLeft = graph.Positions[layer[i-1]].X + nodeRight
			}

			if i+1 < len(layer) {
				wallRight = graph.Positions[layer[i+1]].X - nodeLeft
			} else {
				wallRight = wallLeft + float32(len(graph.Nodes))*(padding+nodeLeft+nodeRight)
			}

			// add padding to the walls
			wallLeft += padding
			wallRight -= padding

			// ensure we can fit at least one
			if wallRight-nodeRight < wallLeft+nodeLeft {
				wallRight = wallLeft + nodeLeft + nodeRight
			}

			// ensure we are not overlapping with the previous
			if graph.Positions[nid].X < wallLeft+nodeLeft {
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

			if center < wallLeft+nodeLeft-nudge {
				center = wallLeft + nodeLeft - nudge
			}
			if wallRight-nodeRight+nudge < center {
				center = wallRight - nodeRight + nudge
			}

			// is between sides
			graph.Positions[nid].X = center
		}
		sanityCheckLayer(graph, layer)
	}
}

func sanityCheckLayer(graph *Graph, layer NodeIDs) {
	return

	deltas := []float32{}
	positions := []float32{}
	fail := false
	wallLeft := float32(padding)
	for _, nid := range layer {
		p := graph.Positions[nid]
		delta := (p.X - nodeLeft) - wallLeft
		if delta < 0 {
			fail = true
		}
		deltas = append(deltas, delta)
		positions = append(positions, p.X)
		wallLeft = p.X + nodeRight + padding
	}

	if fail {
		fmt.Println("=")
		fmt.Println(deltas)
		fmt.Println(positions)
	}
}

func flushLeft(graph *Graph) {
	minleft := graph.Positions[0].X - nodeLeft
	for _, node := range graph.Nodes {
		if graph.Positions[node.ID].X-nodeLeft < minleft {
			minleft = graph.Positions[node.ID].X - nodeLeft
		}
	}

	for _, node := range graph.Nodes {
		graph.Positions[node.ID].X -= minleft - padding
	}
}
