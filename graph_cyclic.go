package layout

// IsCyclic checks whether graph is cyclic
func (graph *Graph) IsCyclic() bool {
	visited := make([]bool, graph.NodeCount())
	recursing := make([]bool, graph.NodeCount())

	var isCyclic func(node *Node) bool
	isCyclic = func(node *Node) bool {
		if visited[node.ID] {
			return false
		}

		visited[node.ID] = true
		recursing[node.ID] = true
		for _, child := range node.Out {
			if !visited[child.ID] && isCyclic(child) {
				return true
			}
			if recursing[child.ID] {
				return true
			}
		}
		recursing[node.ID] = false

		return false
	}

	for _, node := range graph.Nodes {
		if isCyclic(node) {
			return true
		}
	}

	return false
}
