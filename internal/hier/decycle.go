package hier

import (
	"cmp"
	"slices"
)

// DecycleDefault runs recommended decycling algorithm
func DefaultDecycle(graph *Graph) *Graph {
	decycle := NewDecycle(graph)
	decycle.Recurse = true
	decycle.Reorder = true
	decycle.Run()
	return graph
}

// Decycle implements process for removing cycles from a Graph
type Decycle struct {
	*Graph

	Recurse bool
	Reorder bool

	SkipUpdate bool

	info  []DecycleNodeInfo
	edges [][2]ID
}

// NewDecycle creates new decycle process
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

// Run runs the default decycling process
func (graph *Decycle) Run() {
	if !graph.IsCyclic() {
		return
	}

	graph.info = make([]DecycleNodeInfo, graph.NodeCount())
	for _, node := range graph.Nodes {
		graph.info[node.ID].In = node.InDegree()
		graph.info[node.ID].Out = node.OutDegree()
	}

	graph.processNodes(*graph.Nodes.Clone())

	if !graph.SkipUpdate {
		graph.updateEdges()
		graph.updateWeights()
	}
}

// processNodes processes list of nodes
func (graph *Decycle) processNodes(nodes Nodes) {
	if !graph.Reorder {
		for _, node := range nodes {
			graph.process(node)
		}
	} else {
		var node *Node
		for len(nodes) > 0 {
			graph.sortAscending(nodes)
			node, nodes = nodes[len(nodes)-1], nodes[:len(nodes)-1]
			graph.process(node)
		}
	}
}

// process flips unprocessed incoming edges in dst
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

// addEdge adds edge from src to dest
func (graph *Decycle) addEdge(src, dst *Node) {
	graph.edges = append(graph.edges, [2]ID{src.ID, dst.ID})
}

// addFlippedEdge adds edge and flips it
func (graph *Decycle) addFlippedEdge(src, dst *Node) {
	graph.info[src.ID].Out--
	graph.info[src.ID].In++

	graph.info[dst.ID].In--
	graph.info[dst.ID].Out++

	graph.addEdge(dst, src)
}

// updateEdges, updates graph with new edge information
func (graph *Decycle) updateEdges() {
	// recreate inbound links from outbound
	for _, node := range graph.Nodes {
		node.In.Clear()
		node.Out.Clear()
	}

	for _, edge := range graph.edges {
		graph.AddEdge(graph.Nodes[edge[0]], graph.Nodes[edge[1]])
	}

	for _, node := range graph.Nodes {
		node.In.Normalize()
		node.Out.Normalize()
	}
}

// updateWeights carries edge weights over to flipped edges
func (graph *Decycle) updateWeights() {
	old := graph.weights
	graph.weights = nil
	for _, edge := range graph.edges {
		src, dst := graph.Nodes[edge[0]], graph.Nodes[edge[1]]
		if w, ok := old[[2]ID{src.ID, dst.ID}]; ok {
			graph.SetWeight(src, dst, w)
		} else if w, ok := old[[2]ID{dst.ID, src.ID}]; ok {
			graph.SetWeight(src, dst, w)
		}
	}
}

// sortAscending sorts nodes such that the last node is most beneficial to process
func (graph *Decycle) sortAscending(nodes Nodes) {
	slices.SortStableFunc(nodes, func(a, b *Node) int {
		ai, bi := graph.info[a.ID], graph.info[b.ID]
		if ai.Out == bi.Out {
			return cmp.Compare(bi.In, ai.In)
		}
		return cmp.Compare(ai.Out, bi.Out)
	})
}
