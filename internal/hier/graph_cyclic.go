package hier

// IsCyclic checks whether graph is cyclic
func (graph *Graph) IsCyclic() bool {
	visited := NewNodeSet(graph.NodeCount())
	recursing := NewNodeSet(graph.NodeCount())

	var isCyclic func(node *Node) bool
	isCyclic = func(node *Node) bool {
		if !visited.Include(node) {
			return false
		}

		recursing.Add(node)
		for _, child := range node.Out {
			if isCyclic(child) {
				return true
			} else if recursing.Contains(child) {
				return true
			}
		}
		recursing.Remove(node)

		return false
	}

	for _, node := range graph.Nodes {
		if isCyclic(node) {
			return true
		}
	}

	return false
}
