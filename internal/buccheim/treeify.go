package buccheim

// Treeify converts invalid tree into a proper tree
func Treeify(tree *Tree) ID {
	root := SingleRoot(tree)
	DeleteDepthFirstBacklinks(tree, root)
	return root
}

// SingleRoot ensures that there is a single root
func SingleRoot(tree *Tree) ID {
	roots := tree.Roots()
	if len(roots) == 1 {
		return roots[0]
	}

	id := tree.AddNode()
	tree.Nodes[id].Virtual = true
	tree.SetEdges(id, roots)

	return id
}

// DeleteDepthFirstBacklinks deletes edges going upwards
func DeleteDepthFirstBacklinks(tree *Tree, root ID) {
	seen := make([]bool, len(tree.Nodes))

	var recurse func(id ID)
	recurse = func(id ID) {
		seen[id] = true

		node := &tree.Nodes[id]
		children := tree.Edges[node.Start:node.End]
		children0 := children[:0]
		for _, child := range children {
			if !seen[child] {
				tree.Nodes[child].Indegree--
				children0 = append(children0, child)
			}
		}

		node.End = node.Start + node.End
		for _, child := range children0 {
			recurse(child)
		}
	}

	recurse(root)
}
