package hier

import "fmt"

// DefaultPosition does recommended positioning algorithm
func DefaultPosition(graph *Graph) *Graph {
	Position(graph)
	return graph
}

// Position does basic node positioning
func Position(graph *Graph) {
	PositionInitial(graph)

	// TODO: fold nudge into Node parameter
	nudge := float32(10.0)
	for i := 0; i < 100; i++ {
		PositionOutgoing(graph, false, nudge)
		PositionIncoming(graph, false, nudge)
		PositionOutgoing(graph, true, nudge)
		PositionIncoming(graph, true, nudge)
		nudge = nudge * 0.9
		flushLeft(graph)
	}

	for i := 0; i < 10; i++ {
		PositionIncoming(graph, true, 0)
		PositionOutgoing(graph, true, 0)

		flushLeft(graph)
	}
}

// PositionInitial assigns location based on size
func PositionInitial(graph *Graph) {
	top := float32(0)
	for _, nodes := range graph.ByRank {
		left := float32(0)

		halfrow := float32(0)
		for _, node := range nodes {
			halfrow = maxf32(halfrow, node.Radius.Y)
		}

		top += halfrow
		for _, node := range nodes {
			node.Center.X = left + node.Radius.X
			node.Center.Y = top
			left += node.Center.X + node.Radius.X
		}
		top += halfrow
	}
}

// iterateLayers can traverse layers/nodes in different directions
func iterateLayers(graph *Graph, leftToRight bool, dy int, fn func(layer Nodes, i int, node *Node)) {
	var starty int
	if dy < 0 {
		starty = len(graph.ByRank) - 1
	}

	if leftToRight {
		for y := starty; 0 <= y && y < len(graph.ByRank); y += dy {
			layer := graph.ByRank[y]
			for i, node := range layer {
				fn(layer, i, node)
			}
		}
	} else {
		for y := starty; 0 <= y && y < len(graph.ByRank); y += dy {
			layer := graph.ByRank[y]
			for i := len(layer) - 1; i >= 0; i-- {
				fn(layer, i, layer[i])
			}
		}
	}
}

// NodeWalls calculates bounds where node can be moved
func NodeWalls(graph *Graph, layer Nodes, i int, node *Node, leftToRight bool) (wallLeft, wallRight float32) {
	if i > 0 {
		wallLeft = layer[i-1].Center.X + layer[i-1].Radius.X
	}

	if i+1 < len(layer) {
		wallRight = layer[i+1].Center.X - layer[i+1].Radius.X
	} else {
		wallRight = float32(len(graph.Nodes)) * (2 * node.Radius.X)
	}

	// ensure we can fit at least one
	if leftToRight {
		if wallRight-node.Radius.X < wallLeft+node.Radius.X {
			wallRight = wallLeft + 2*node.Radius.X
		}
	} else {
		if wallRight-node.Radius.X < wallLeft+node.Radius.X {
			wallLeft = wallRight - 2*node.Radius.X
		}
	}

	if leftToRight {
		if node.Center.X < wallLeft+node.Radius.X {
			node.Center.X = wallLeft + node.Radius.X
		}
	} else {
		if node.Center.X > wallRight-node.Radius.X {
			node.Center.X = wallRight - node.Radius.X
		}
	}

	return wallLeft, wallRight
}

// PositionIncoming positions node based on incoming edges
func PositionIncoming(graph *Graph, leftToRight bool, nudge float32) {
	iterateLayers(graph, leftToRight, 1,
		func(layer Nodes, i int, node *Node) {
			wallLeft, wallRight := NodeWalls(graph, layer, i, node, leftToRight)

			// calculate average location based on incoming
			if len(node.In) == 0 {
				return
			}
			center := float32(0.0)
			for _, node := range node.In {
				center += node.Center.X
			}
			center /= float32(len(node.In))

			center = clampf32(center, wallLeft+node.Radius.X-nudge, wallRight-node.Radius.Y+nudge)

			// is between sides
			node.Center.X = center
		})
}

// PositionOutgoing positions node based on outgoing edges
func PositionOutgoing(graph *Graph, leftToRight bool, nudge float32) {
	iterateLayers(graph, leftToRight, -1,
		func(layer Nodes, i int, node *Node) {
			wallLeft, wallRight := NodeWalls(graph, layer, i, node, leftToRight)

			// calculate average location based on incoming
			if len(node.Out) == 0 {
				return
			}
			center := float32(0.0)
			for _, node := range node.Out {
				center += node.Center.X
			}
			center /= float32(len(node.Out))

			center = clampf32(center, wallLeft+node.Radius.X-nudge, wallRight-node.Radius.X+nudge)

			// is between sides
			node.Center.X = center
		})
}

// sanityCheckLayer checks whether any nodes are overlapping
func sanityCheckLayer(graph *Graph, layer Nodes) {
	deltas := []float32{}
	positions := []float32{}
	fail := false
	wallLeft := float32(0)
	for _, node := range layer {
		delta := (node.Center.X - node.Radius.X) - wallLeft
		if delta < 0 {
			fail = true
		}
		deltas = append(deltas, delta)
		positions = append(positions, node.Center.X)
		wallLeft = node.Center.X + node.Radius.X
	}

	if fail {
		fmt.Println("=")
		fmt.Println(deltas)
		fmt.Println(positions)
	}
}

// flushLeft corrects for graph drift due to moving nodes around
func flushLeft(graph *Graph) {
	node := graph.Nodes[0]
	minleft := node.Center.X - node.Radius.X
	for _, node := range graph.Nodes[1:] {
		if node.Center.X-node.Radius.X < minleft {
			minleft = node.Center.X - node.Radius.X
		}
	}

	for _, node := range graph.Nodes {
		node.Center.X -= minleft
	}
}
