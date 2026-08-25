package hier

import "math"

// RankNetworkSimplex assigns ranks minimizing the total weighted edge length
// Σ weight(e)·(rank(dst) − rank(src)) subject to every edge spanning at least
// one rank, using the network simplex method of Gansner et al.
//
// Nodes in a SameRank group are contracted into a single vertex so they end
// up on the same rank. The graph must be acyclic.
//
// Cut values are recomputed from scratch after every pivot, which is
// O(V·E) per pivot; fine for hundreds of nodes, slow for thousands.
func RankNetworkSimplex(graph *Graph) {
	if len(graph.Nodes) == 0 {
		return
	}

	// contract same-rank groups
	rep := make([]int, graph.NodeCount())
	for i := range rep {
		rep[i] = i
	}
	for _, group := range graph.SameRank {
		for _, node := range group {
			rep[node.ID] = rep[group[0].ID]
		}
	}
	verts := 0
	index := make([]int, graph.NodeCount()) // contracted vertex index by node id
	for i := range index {
		index[i] = -1
	}
	for _, node := range graph.Nodes {
		if index[rep[node.ID]] < 0 {
			index[rep[node.ID]] = verts
			verts++
		}
		index[node.ID] = index[rep[node.ID]]
	}

	s := &simplex{n: verts}
	for _, src := range graph.Nodes {
		for _, dst := range src.Out {
			u, v := index[src.ID], index[dst.ID]
			if u == v {
				continue // edge inside a same-rank group
			}
			s.edges = append(s.edges, simplexEdge{tail: u, head: v, weight: graph.Weight(src, dst)})
		}
	}
	s.run()

	for _, node := range graph.Nodes {
		node.Rank = s.rank[index[node.ID]]
	}
}

type simplexEdge struct {
	tail, head int
	weight     float32
	tree       bool
}

type simplex struct {
	n     int
	edges []simplexEdge
	rank  []int
	adj   [][]int // edge indexes by vertex, both directions
}

func (s *simplex) run() {
	s.adj = make([][]int, s.n)
	for i, e := range s.edges {
		s.adj[e.tail] = append(s.adj[e.tail], i)
		s.adj[e.head] = append(s.adj[e.head], i)
	}

	s.initRank()
	s.feasibleTree()

	for iter := 0; iter < 8*len(s.edges)+8; iter++ {
		leave := s.negativeCutEdge()
		if leave < 0 {
			break
		}
		enter := s.enteringEdge(leave)
		if enter < 0 {
			break // should not happen: cut value negative implies a candidate
		}
		s.edges[leave].tree = false
		s.edges[enter].tree = true
		s.rerank()
	}
	s.normalize()
}

func (s *simplex) length(i int) int { return s.rank[s.edges[i].head] - s.rank[s.edges[i].tail] }
func (s *simplex) slack(i int) int  { return s.length(i) - 1 }

// initRank assigns longest-path ranks, a feasible starting point
func (s *simplex) initRank() {
	s.rank = make([]int, s.n)
	indeg := make([]int, s.n)
	for _, e := range s.edges {
		indeg[e.head]++
	}
	var queue []int
	for v := range s.n {
		if indeg[v] == 0 {
			queue = append(queue, v)
		}
	}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		for _, i := range s.adj[v] {
			e := s.edges[i]
			if e.tail != v {
				continue
			}
			s.rank[e.head] = max(s.rank[e.head], s.rank[v]+1)
			indeg[e.head]--
			if indeg[e.head] == 0 {
				queue = append(queue, e.head)
			}
		}
	}
}

// feasibleTree grows a spanning forest of tight edges, shifting ranks of the
// partial tree to make the closest non-tree edge tight when it gets stuck.
func (s *simplex) feasibleTree() {
	inTree := make([]bool, s.n)
	for root := range s.n {
		if inTree[root] {
			continue
		}
		// one component
		inTree[root] = true
		size := 1
		for {
			// grow along tight edges
			changed := true
			for changed {
				changed = false
				for i := range s.edges {
					e := &s.edges[i]
					if e.tree || inTree[e.tail] == inTree[e.head] || s.slack(i) != 0 {
						continue
					}
					e.tree = true
					inTree[e.tail], inTree[e.head] = true, true
					size++
					changed = true
				}
			}
			// find the incident non-tree edge with minimal slack
			best, bestSlack := -1, math.MaxInt
			for i, e := range s.edges {
				if e.tree || inTree[e.tail] == inTree[e.head] {
					continue
				}
				if sl := s.slack(i); sl < bestSlack {
					best, bestSlack = i, sl
				}
			}
			if best < 0 {
				break // component complete
			}
			// shift the tree side so the edge becomes tight
			delta := bestSlack
			if inTree[s.edges[best].head] {
				delta = -bestSlack
			}
			// nodes reachable from root through tree edges are the tree; they
			// share the component with root, but inTree also marks other
			// finished components, so walk the tree explicitly
			for _, v := range s.treeNodes(root) {
				s.rank[v] += delta
			}
		}
		_ = size
	}
}

// treeNodes returns the vertices connected to root by tree edges
func (s *simplex) treeNodes(root int) []int {
	seen := make([]bool, s.n)
	seen[root] = true
	nodes := []int{root}
	for k := 0; k < len(nodes); k++ {
		v := nodes[k]
		for _, i := range s.adj[v] {
			e := s.edges[i]
			if !e.tree {
				continue
			}
			w := e.tail + e.head - v
			if !seen[w] {
				seen[w] = true
				nodes = append(nodes, w)
			}
		}
	}
	return nodes
}

// tailComponent returns membership of the tail side after removing tree edge i
func (s *simplex) tailComponent(i int) []bool {
	side := make([]bool, s.n)
	e := s.edges[i]
	side[e.tail] = true
	stack := []int{e.tail}
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, k := range s.adj[v] {
			if k == i || !s.edges[k].tree {
				continue
			}
			f := s.edges[k]
			w := f.tail + f.head - v
			if !side[w] {
				side[w] = true
				stack = append(stack, w)
			}
		}
	}
	return side
}

// cutValue of tree edge i: weight of edges from the tail side to the head
// side minus the weight of edges the other way.
func (s *simplex) cutValue(i int) float32 {
	tail := s.tailComponent(i)
	cut := float32(0)
	for _, e := range s.edges {
		switch {
		case tail[e.tail] && !tail[e.head]:
			cut += e.weight
		case !tail[e.tail] && tail[e.head]:
			cut -= e.weight
		}
	}
	return cut
}

// negativeCutEdge returns a tree edge with negative cut value, or -1
func (s *simplex) negativeCutEdge() int {
	best, bestCut := -1, float32(0)
	for i, e := range s.edges {
		if !e.tree {
			continue
		}
		if cut := s.cutValue(i); cut < bestCut {
			best, bestCut = i, cut
		}
	}
	return best
}

// enteringEdge returns the non-tree edge from the head side to the tail side
// of tree edge leave with minimal slack
func (s *simplex) enteringEdge(leave int) int {
	tail := s.tailComponent(leave)
	best, bestSlack := -1, math.MaxInt
	for i, e := range s.edges {
		if e.tree || !(tail[e.head] && !tail[e.tail]) {
			continue
		}
		if sl := s.slack(i); sl < bestSlack {
			best, bestSlack = i, sl
		}
	}
	return best
}

// rerank recomputes ranks from the spanning forest so tree edges are tight
func (s *simplex) rerank() {
	seen := make([]bool, s.n)
	for root := range s.n {
		if seen[root] {
			continue
		}
		seen[root] = true
		queue := []int{root}
		for len(queue) > 0 {
			v := queue[0]
			queue = queue[1:]
			for _, i := range s.adj[v] {
				e := s.edges[i]
				if !e.tree {
					continue
				}
				w := e.tail + e.head - v
				if seen[w] {
					continue
				}
				seen[w] = true
				if e.tail == v {
					s.rank[w] = s.rank[v] + 1
				} else {
					s.rank[w] = s.rank[v] - 1
				}
				queue = append(queue, w)
			}
		}
	}
}

// normalize shifts every component so that its minimum rank is 0
func (s *simplex) normalize() {
	seen := make([]bool, s.n)
	for root := range s.n {
		if seen[root] {
			continue
		}
		nodes := s.treeNodes(root)
		lo := math.MaxInt
		for _, v := range nodes {
			seen[v] = true
			lo = min(lo, s.rank[v])
		}
		for _, v := range nodes {
			s.rank[v] -= lo
		}
	}
}
