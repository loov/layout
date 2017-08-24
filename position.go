package layout

import "fmt"

const (
	padding    = float32(10)
	rowpadding = float32(30)
)

func Position(graph *Graph) {
	const nodeWidth = float32(20)

	for _, node := range graph.Nodes {
		if node.Virtual {
			node.Radius.X = nodeWidth * 0.5
			node.Radius.Y = nodeWidth * 0.5
		} else {
			node.Radius.X = nodeWidth * 0.5
			node.Radius.Y = nodeWidth * 0.5
		}
	}

	Position_Initial_LeftToRight(graph)

	// TODO: fold nudge into Node parameter
	nudge := nodeWidth
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
		bottom := float32(rowpadding)
		for _, node := range nodes {
			node.Position.X = left + node.Radius.X + node.Position.X
			node.Position.Y = top + node.Radius.Y + node.Position.Y

			left += node.Position.X + node.Radius.X + padding
			bottom = maxf32(node.Position.Y+node.Radius.Y+padding, bottom)
		}
		sanityCheckLayer(graph, nodes)
		top = bottom
	}
}

func iterateLayers(graph *Graph, leftToRight bool, dy int,
	fn func(layer Nodes, i int, node *Node)) {
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

func NodeWalls(graph *Graph, layer Nodes, i int, node *Node, leftToRight bool) (wallLeft, wallRight float32) {
	if i > 0 {
		wallLeft = layer[i-1].Position.X + layer[i-1].Radius.X
	}

	if i+1 < len(layer) {
		wallRight = layer[i+1].Position.X - layer[i+1].Radius.X
	} else {
		wallRight = float32(len(graph.Nodes)) * (padding + node.Radius.X + node.Radius.Y)
	}

	wallLeft += padding
	wallRight -= padding

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
		if node.Position.X < wallLeft+node.Radius.X {
			node.Position.X = wallLeft + node.Radius.X
		}
	} else {
		if node.Position.X > wallRight-node.Radius.X {
			node.Position.X = wallRight - node.Radius.X
		}
	}

	return wallLeft, wallRight
}

func Position_Incoming(graph *Graph, leftToRight bool, nudge float32) {
	iterateLayers(graph, leftToRight, 1,
		func(layer Nodes, i int, node *Node) {
			wallLeft, wallRight := NodeWalls(graph, layer, i, node, leftToRight)

			// calculate average location based on incoming
			if len(node.In) == 0 {
				return
			}
			center := float32(0.0)
			for _, node := range node.In {
				center += node.Position.X
			}
			center /= float32(len(node.In))

			center = clampf32(center, wallLeft+node.Radius.X-nudge, wallRight-node.Radius.Y+nudge)

			// is between sides
			node.Position.X = center
		})
}

func Position_Outgoing(graph *Graph, leftToRight bool, nudge float32) {
	iterateLayers(graph, leftToRight, -1,
		func(layer Nodes, i int, node *Node) {
			wallLeft, wallRight := NodeWalls(graph, layer, i, node, leftToRight)

			// calculate average location based on incoming
			if len(node.Out) == 0 {
				return
			}
			center := float32(0.0)
			for _, node := range node.Out {
				center += node.Position.X
			}
			center /= float32(len(node.Out))

			center = clampf32(center, wallLeft+node.Radius.X-nudge, wallRight-node.Radius.X+nudge)

			// is between sides
			node.Position.X = center
		})
}

func sanityCheckLayer(graph *Graph, layer Nodes) {
	return

	deltas := []float32{}
	positions := []float32{}
	fail := false
	wallLeft := float32(padding)
	for _, node := range layer {
		delta := (node.Position.X - node.Radius.X) - wallLeft
		if delta < 0 {
			fail = true
		}
		deltas = append(deltas, delta)
		positions = append(positions, node.Position.X)
		wallLeft = node.Position.X + node.Radius.X + padding
	}

	if fail {
		fmt.Println("=")
		fmt.Println(deltas)
		fmt.Println(positions)
	}
}

func flushLeft(graph *Graph) {
	node := graph.Nodes[0]
	minleft := node.Position.X - node.Radius.X
	for _, node := range graph.Nodes[1:] {
		if node.Position.X-node.Radius.X < minleft {
			minleft = node.Position.X - node.Radius.X
		}
	}

	for _, node := range graph.Nodes {
		node.Position.X -= minleft - padding
	}
}
