package layout

const (
	nodesize   = 10
	padding    = 3
	rowpadding = 10
)

func AssignPositions(graph *Graph) {
	graph.Positions = make([]Position, len(graph.Nodes))
	AssignInitialPositions(graph)
}

func AssignInitialPositions(graph *Graph) {
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
