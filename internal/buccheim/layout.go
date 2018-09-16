package buccheim

func Layout(tree *Tree) {
	root := Treeify(tree)
	reset(tree, root, -1, root, 0, 0)

	var layout func(n ID)
	layout = func(n ID) {

	}

	layout(root)
}

func reset(tree *Tree, id, parent, leftmost ID, depth, number int) {
	node := &tree.Nodes[id]
	node.X, node.Y = -1.0, float32(depth)
	node.Parent = parent
	node.Ancestor = id
	node.Thread = -1
	node.LeftMostSibling = -1
	if leftmost != id {
		node.LeftMostSibling = leftmost
	}

	node.Offset = 0
	node.Change = 0
	node.Shift = 0
	node.Number = number

	edges := tree.Edges[node.Start:node.End]
	for i, cid := range edges {
		reset(tree, cid, id, edges[0], depth+1, i)
	}
}

func leftBrother(tree *Tree, id ID) *Node {
	node := &tree.Nodes[id]
	if node.Parent < 0 || node.Number == 0 {
		return nil
	}
	parent := &tree.Nodes[node.Parent]
	return &tree.Nodes[tree.Edges[parent.Start+node.Number-1]]
}

func firstWalk(tree *Tree, id ID, distance float32) {
	node := &tree.Nodes[id]
	lbrother := leftBrother(tree, id)
	if node.Outdegree() == 0 {
		if lbrother != nil {
			node.X = lbrother.X + distance
		} else {
			node.X = 0
		}
		return
	}
	children := tree.Edges[node.Start:node.End]

	defaultAncestor := children[0]
	for _, child := range children {
		firstWalk(tree, child, 1)
		defaultAncestor = apportion(tree, child, defaultAncestor, distance)
	}

	executeShifts(tree, id)

	first := children[0]
	last := children[len(children)-1]
	midpoint := (tree.Nodes[first].X + tree.Nodes[last].X) * 0.5

	if lbrother != nil {
		node.X = lbrother.X + distance
		node.Mod = node.X - midpoint
	} else {
		node.X = midpoint
	}
}

func apportion(tree *Tree, id, defaultAncestor ID, distance float32) ID {
	return 0
}

func executeShifts(tree *Tree, id ID) {

}
