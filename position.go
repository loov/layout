package layout

const (
	nodesize   = 15
	padding    = 10
	rowpadding = 30
)

func Position(graph *Graph) {
	graph.Positions = make([]Vector, len(graph.Nodes))
	Position_Initial_LeftToRight(graph)
	for i := 0; i < 100; i++ {
		Position_Improve_Median(graph, i%2 == 0)
	}
}

func Position_Initial_LeftToRight(graph *Graph) {
	top := float32(rowpadding)
	for _, nodes := range graph.ByRank {
		left := float32(padding)
		for _, id := range nodes {
			graph.Positions[id].X = left + nodesize/2
			graph.Positions[id].Y = top + nodesize/2
			left += nodesize + padding
		}
		top += nodesize + rowpadding
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
