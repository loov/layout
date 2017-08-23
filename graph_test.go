package layout

type TestGraph struct {
	Name   string
	Cyclic bool
	Edges  [][]int
}

func (testgraph *TestGraph) Make() *Graph { return NewGraphFromEdgeList(testgraph.Edges) }

var TestGraphs = []TestGraph{
	// acyclic graphs
	{"null", false, [][]int{}},
	{"node", false, [][]int{
		0: []int{},
	}},
	{"link", false, [][]int{
		0: []int{1},
		1: []int{},
	}},
	{"link-reverse", false, [][]int{
		0: []int{},
		1: []int{0},
	}},
	{"chain", false, [][]int{
		0: []int{1},
		1: []int{2},
		2: []int{3},
		3: []int{},
	}},
	{"chain-reverse", false, [][]int{
		0: []int{},
		1: []int{0},
		2: []int{1},
		3: []int{2},
	}},
	{"split", false, [][]int{
		0: []int{1},
		1: []int{2, 3},
		2: []int{4},
		3: []int{5},
		4: []int{},
		5: []int{},
	}},
	{"merge", false, [][]int{
		0: []int{1},
		1: []int{2, 3},
		2: []int{4},
		3: []int{4},
		4: []int{},
	}},
	// acyclic graphs with 2 components
	{"2-node", false, [][]int{
		0: []int{},
		1: []int{},
	}},
	{"2-link", false, [][]int{
		0: []int{1},
		1: []int{},

		2: []int{},
		3: []int{2},
	}},
	{"2-split", false, [][]int{
		0: []int{1, 2},
		1: []int{},
		2: []int{},

		4: []int{},
		5: []int{},
		6: []int{5, 4},
	}},
	{"2-merge", false, [][]int{
		0: []int{1, 2},
		1: []int{3},
		2: []int{3},
		3: []int{},

		4: []int{},
		5: []int{4},
		6: []int{4},
		7: []int{6, 5},
	}},

	// cyclic graphs
	{"loop", true, [][]int{
		0: []int{0},
	}},
	{"2-circle", true, [][]int{
		0: []int{1},
		1: []int{0},
	}},
	{"4-circle", true, [][]int{
		0: []int{1},
		1: []int{2},
		2: []int{3},
		3: []int{0},
	}},
	{"5-split-cycle", true, [][]int{
		0: []int{1},
		1: []int{2, 3},
		2: []int{4},
		3: []int{4},
		4: []int{0},
	}},
	{"5-split-2-cycle", true, [][]int{
		0: []int{1},
		1: []int{2, 3, 0},
		2: []int{4},
		3: []int{4, 2},
		4: []int{2},
	}},
	{"5-complete", true, [][]int{
		0: []int{0, 1, 2, 3, 4},
		1: []int{0, 1, 2, 3, 4},
		2: []int{0, 1, 2, 3, 4},
		3: []int{0, 1, 2, 3, 4},
		4: []int{0, 1, 2, 3, 4},
	}},

	// regressions
	{"regression-0", true, [][]int{
		0: []int{0, 1, 2, 4, 5},
		1: []int{0, 2, 3},
		2: []int{0, 1, 4, 5, 6},
		3: []int{0, 3, 4},
		4: []int{0, 1, 2, 3, 4, 5},
		5: []int{0, 1, 2},
		6: []int{0, 6},
	}},
	{"regression-1", true, [][]int{
		0: []int{1, 2, 3, 4},
		1: []int{1, 5},
		2: []int{1},
		3: []int{0, 1, 2, 3},
		4: []int{0, 2},
		5: []int{0, 1, 2, 6},
		6: []int{1, 3, 4},
	}},
	{"regression-2", true, [][]int{
		0: []int{1, 2, 3, 4, 5, 6},
		1: []int{0, 1, 2, 3, 4, 5, 6},
		2: []int{1},
		3: []int{0, 3, 4, 5, 6},
		4: []int{0, 1, 2, 3, 4, 5, 6},
		5: []int{0, 1, 2, 5, 6},
		6: []int{0, 1, 2, 3, 4, 5, 6},
	}},
}
