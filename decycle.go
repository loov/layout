package layout

import "sort"

func DecycleDefault(graph *Graph) {
	decycle := NewDecycle(graph)
	decycle.Recurse = true
	decycle.Reorder = true
	decycle.Run()
}

// Decycle implements process for removing cycles from a Graph
type Decycle struct {
	*Graph

	Recurse bool
	Reorder bool

	info  []DecycleNodeInfo
	edges EdgeSet
}

func NewDecycle(graph *Graph) *Decycle {
	dg := &Decycle{}
	dg.Graph = graph
	dg.Recurse = false
	dg.Reorder = true
	return dg
}

// DecycleNodeInfo contains running info necessary in decycling
type DecycleNodeInfo struct {
	Processed bool
	In, Out   int
}

func (graph *Decycle) Run() {
	if !graph.IsCyclic() {
		return
	}

	graph.edges = NewEdgeSet()
	graph.info = make([]DecycleNodeInfo, graph.NodeCount())
	for _, node := range graph.Nodes {
		graph.info[node.ID].In = node.InDegree()
		graph.info[node.ID].Out = node.OutDegree()
	}

	graph.processNodes(*graph.Nodes.Clone())
	graph.edges.SetTo(graph.Graph)
}

func (graph *Decycle) processNodes(nodes Nodes) {
	if !graph.Reorder {
		for _, node := range nodes {
			graph.process(node)
		}
	} else {
		var node *Node
		for len(nodes) > 0 {
			graph.SortAscending(nodes)
			node, nodes = nodes[len(nodes)-1], nodes[:len(nodes)-1]
			graph.process(node)
		}
	}
}

func (graph *Decycle) process(dst *Node) {
	if graph.info[dst.ID].Processed {
		return
	}
	graph.info[dst.ID].Processed = true

	var recurse Nodes
	for _, src := range dst.In {
		if src == dst {
			continue
		}

		if graph.info[src.ID].Processed {
			graph.addEdge(src, dst)
		} else {
			graph.addFlippedEdge(src, dst)
			if graph.Recurse {
				recurse.Append(src)
			}
		}
	}

	if graph.Recurse {
		graph.processNodes(recurse)
	}
}

func (graph *Decycle) addEdge(src, dst *Node) {
	graph.edges.Include(src, dst)
}

func (graph *Decycle) addFlippedEdge(src, dst *Node) {
	graph.edges.Include(dst, src)

	graph.info[src.ID].Out--
	graph.info[src.ID].In++

	graph.info[dst.ID].In--
	graph.info[dst.ID].Out++
}

func (graph *Decycle) SortAscending(nodes []*Node) {
	sort.Slice(nodes, func(i, k int) bool {
		a, b := nodes[i], nodes[k]
		ai, bi := graph.info[a.ID], graph.info[b.ID]
		if ai.Out == bi.Out {
			return ai.In > bi.In
		}
		return ai.Out < bi.Out
	})
}
