package layout

type TestGraph struct {
	Name   string
	Cyclic bool
	Edges  [][]int
}

func (testgraph *TestGraph) Make() *Graph {
	return NewGraphFromEdgeList(testgraph.Edges)
}

var TestGraphs = []TestGraph{
	// acyclic graphs
	{"null", false, [][]int{}},
	{"node", false, [][]int{
		0: {},
	}},
	{"link", false, [][]int{
		0: {1},
		1: {},
	}},
	{"link-reverse", false, [][]int{
		0: {},
		1: {0},
	}},
	{"chain", false, [][]int{
		0: {1},
		1: {2},
		2: {3},
		3: {},
	}},
	{"chain-reverse", false, [][]int{
		0: {},
		1: {0},
		2: {1},
		3: {2},
	}},
	{"split", false, [][]int{
		0: {1},
		1: {2, 3},
		2: {4},
		3: {5},
		4: {},
		5: {},
	}},
	{"merge", false, [][]int{
		0: {1},
		1: {2, 3},
		2: {4},
		3: {4},
		4: {},
	}},
	// acyclic graphs with 2 components
	{"2-node", false, [][]int{
		0: {},
		1: {},
	}},
	{"2-link", false, [][]int{
		0: {1},
		1: {},

		2: {},
		3: {2},
	}},
	{"2-split", false, [][]int{
		0: {1, 2},
		1: {},
		2: {},

		4: {},
		5: {},
		6: {5, 4},
	}},
	{"2-merge", false, [][]int{
		0: {1, 2},
		1: {3},
		2: {3},
		3: {},

		4: {},
		5: {4},
		6: {4},
		7: {6, 5},
	}},

	// cyclic graphs
	{"loop", true, [][]int{
		0: {0},
	}},
	{"2-circle", true, [][]int{
		0: {1},
		1: {0},
	}},
	{"4-circle", true, [][]int{
		0: {1},
		1: {2},
		2: {3},
		3: {0},
	}},
	{"5-split-cycle", true, [][]int{
		0: {1},
		1: {2, 3},
		2: {4},
		3: {4},
		4: {0},
	}},
	{"5-split-2-cycle", true, [][]int{
		0: {1},
		1: {2, 3, 0},
		2: {4},
		3: {4, 2},
		4: {2},
	}},
	{"5-complete", true, [][]int{
		0: {0, 1, 2, 3, 4},
		1: {0, 1, 2, 3, 4},
		2: {0, 1, 2, 3, 4},
		3: {0, 1, 2, 3, 4},
		4: {0, 1, 2, 3, 4},
	}},

	// regressions
	{"regression-0", true, [][]int{
		0: {0, 1, 2, 4, 5},
		1: {0, 2, 3},
		2: {0, 1, 4, 5, 6},
		3: {0, 3, 4},
		4: {0, 1, 2, 3, 4, 5},
		5: {0, 1, 2},
		6: {0, 6},
	}},
	{"regression-1", true, [][]int{
		0: {1, 2, 3, 4},
		1: {1, 5},
		2: {1},
		3: {0, 1, 2, 3},
		4: {0, 2},
		5: {0, 1, 2, 6},
		6: {1, 3, 4},
	}},
	{"regression-2", true, [][]int{
		0: {1, 2, 3, 4, 5, 6},
		1: {0, 1, 2, 3, 4, 5, 6},
		2: {1},
		3: {0, 3, 4, 5, 6},
		4: {0, 1, 2, 3, 4, 5, 6},
		5: {0, 1, 2, 5, 6},
		6: {0, 1, 2, 3, 4, 5, 6},
	}},
}
