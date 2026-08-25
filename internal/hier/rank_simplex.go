package hier

import "math"

// RankNetworkSimplex assigns ranks minimizing the total weighted edge length
// Σ weight(e)·(rank(dst) − rank(src)) subject to every edge spanning at least
// one rank, using the network simplex method of Gansner et al.
//
// Nodes in a SameRank group are contracted into a single vertex so they end
// up on the same rank; MinRank and MaxRank nodes are contracted with an
// artificial source or sink connected to every other vertex. Edges that
// would contradict those constraints are ignored here and end up flat or
// backwards; see Rank. The graph must be acyclic.
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
	unify := func(group Nodes) {
		for _, node := range group[1:] {
			r := rep[node.ID]
			for i := range rep {
				if rep[i] == r {
					rep[i] = rep[group[0].ID]
				}
			}
		}
	}
	if len(graph.MinRank) > 0 {
		unify(graph.MinRank)
	}
	if len(graph.MaxRank) > 0 {
		unify(graph.MaxRank)
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

	source, sink := -1, -1
	if len(graph.MinRank) > 0 {
		source = index[graph.MinRank[0].ID]
	}
	if len(graph.MaxRank) > 0 {
		sink = index[graph.MaxRank[0].ID]
	}

	s := &simplex{n: verts}
	for _, src := range graph.Nodes {
		for _, dst := range src.Out {
			u, v := index[src.ID], index[dst.ID]
			if u == v || v == source || u == sink {
				continue // inside a group, or contradicting a min/max pin
			}
			s.edges = append(s.edges, simplexEdge{tail: u, head: v, weight: graph.Weight(src, dst)})
		}
	}
	// zero weight edges keep the artificial source first and sink last
	for v := range verts {
		if source >= 0 && v != source {
			s.edges = append(s.edges, simplexEdge{tail: source, head: v})
		}
		if sink >= 0 && v != sink && v != source {
			s.edges = append(s.edges, simplexEdge{tail: v, head: sink})
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

	// spanning forest numbering, see number; buffers reused across pivots
	low, lim, parent, cursor []int
	order                    []int
	agg, cut                 []float32
	stack                    []int
	mark                     []int // visited marks by epoch, see subtree
	epoch                    int
	nodes                    []int // subtree result buffer
}

func (s *simplex) run() {
	s.adj = make([][]int, s.n)
	for i, e := range s.edges {
		s.adj[e.tail] = append(s.adj[e.tail], i)
		s.adj[e.head] = append(s.adj[e.head], i)
	}

	s.initRank()
	s.feasibleTree()

	s.low = make([]int, s.n)
	s.lim = make([]int, s.n)
	s.parent = make([]int, s.n)
	s.cursor = make([]int, s.n)
	s.order = make([]int, s.n)
	s.agg = make([]float32, s.n)
	s.cut = make([]float32, len(s.edges))
	s.stack = make([]int, 0, s.n)
	s.mark = make([]int, s.n)
	s.cutValues()

	for iter := 0; iter < 8*len(s.edges)+8; iter++ {
		leave := s.negativeCutEdge()
		if leave < 0 {
			break
		}
		side, sideIsTail := s.smallerSide(leave)
		enter := s.enteringEdge(side, sideIsTail)
		if enter < 0 {
			break // should not happen: cut value negative implies a candidate
		}
		s.exchange(leave, enter, side, sideIsTail)
	}
	s.normalize()
}

// smallerSide returns the vertices on the smaller side of the cut made by
// tree edge leave, and whether that side contains the edge's tail.
func (s *simplex) smallerSide(leave int) (side []int, sideIsTail bool) {
	child, childIsTail := s.subtreeSide(leave)
	e := s.edges[leave]
	if 2*(s.lim[child]-s.low[child]+1) <= s.n {
		return s.subtree(child, leave), childIsTail
	}
	return s.subtree(e.tail+e.head-child, leave), !childIsTail
}

// exchange replaces tree edge leave with non-tree edge enter, updating
// ranks, cut values and the subtree numbering incrementally. side is the
// smaller side of the cut, see smallerSide.
func (s *simplex) exchange(leave, enter int, side []int, sideIsTail bool) {
	f := s.edges[enter]

	// shift the smaller side so that enter becomes tight: enter goes from
	// the head side to the tail side, so the tail side moves up (lower
	// ranks) or equivalently the head side moves down
	delta := s.slack(enter)
	if sideIsTail {
		delta = -delta
	}
	if delta != 0 {
		for _, v := range side {
			s.rank[v] += delta
		}
	}

	// cut values change only on the tree path between enter's endpoints;
	// walking up from each endpoint to their lowest common ancestor
	cutLeave := s.cut[leave]
	lca := s.treeUpdate(f.tail, f.head, cutLeave, true)
	if other := s.treeUpdate(f.head, f.tail, cutLeave, false); other != lca {
		panic("hier: network simplex tree update mismatch")
	}
	s.cut[enter] = -cutLeave
	s.cut[leave] = 0
	s.edges[leave].tree = false
	s.edges[enter].tree = true

	// renumber the lca subtree; its number range does not change
	s.renumber(lca, s.low[lca])
}

// treeUpdate walks from v towards the root until w is inside v's subtree,
// adjusting the cut value of every parent edge on the way, and returns the
// vertex where it stopped (the lowest common ancestor of v and w).
func (s *simplex) treeUpdate(v, w int, cutvalue float32, dir bool) int {
	for !s.inSubtree(w, v) {
		p := s.parent[v]
		if p < 0 {
			panic("hier: network simplex endpoints in different trees")
		}
		e := s.edges[p]
		d := dir
		if v != e.tail {
			d = !d
		}
		if d {
			s.cut[p] += cutvalue
		} else {
			s.cut[p] -= cutvalue
		}
		v = e.tail + e.head - v
	}
	return v
}

// subtree returns the vertices reachable from root over tree edges without
// crossing edge skip
func (s *simplex) subtree(root, skip int) []int {
	s.epoch++
	s.mark[root] = s.epoch
	s.stack = append(s.stack[:0], root)
	nodes := append(s.nodes[:0], root)
	for len(s.stack) > 0 {
		v := s.stack[len(s.stack)-1]
		s.stack = s.stack[:len(s.stack)-1]
		for _, i := range s.adj[v] {
			e := s.edges[i]
			if !e.tree || i == skip {
				continue
			}
			w := e.tail + e.head - v
			if s.mark[w] == s.epoch {
				continue
			}
			s.mark[w] = s.epoch
			nodes = append(nodes, w)
			s.stack = append(s.stack, w)
		}
	}
	s.nodes = nodes
	return nodes
}

// renumber assigns low/lim/parent within the subtree of root, starting the
// numbering at next; used after an exchange to fix the lca subtree
func (s *simplex) renumber(root int, next int) {
	parentEdge := s.parent[root]
	s.epoch++
	s.mark[root] = s.epoch
	s.cursor[root] = 0
	s.stack = append(s.stack[:0], root)
	s.low[root] = next
	for len(s.stack) > 0 {
		v := s.stack[len(s.stack)-1]
		adj := s.adj[v]
		descended := false
		for s.cursor[v] < len(adj) {
			i := adj[s.cursor[v]]
			s.cursor[v]++
			e := s.edges[i]
			if !e.tree || i == parentEdge {
				continue
			}
			w := e.tail + e.head - v
			if s.mark[w] != s.epoch {
				s.mark[w] = s.epoch
				s.cursor[w] = 0
				s.low[w] = next
				s.parent[w] = i
				s.stack = append(s.stack, w)
				descended = true
				break
			}
		}
		if !descended {
			s.lim[v] = next
			next++
			s.stack = s.stack[:len(s.stack)-1]
		}
	}
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

// number assigns a postorder number (lim) and the smallest number in the
// subtree (low) to every vertex of the spanning forest; parent[v] is the
// tree edge to v's parent. Iterative, with a cursor per vertex so each
// adjacency list is scanned once.
func (s *simplex) number() {
	for i := range s.parent {
		s.parent[i] = -1
		s.low[i] = 0
		s.cursor[i] = 0
	}
	next := 1
	for root := range s.n {
		if s.low[root] != 0 {
			continue
		}
		s.stack = append(s.stack[:0], root)
		s.low[root] = next
		for len(s.stack) > 0 {
			v := s.stack[len(s.stack)-1]
			adj := s.adj[v]
			descended := false
			for s.cursor[v] < len(adj) {
				e := s.edges[adj[s.cursor[v]]]
				s.cursor[v]++
				if !e.tree {
					continue
				}
				w := e.tail + e.head - v
				if s.low[w] == 0 {
					s.low[w] = next
					s.parent[w] = adj[s.cursor[v]-1]
					s.stack = append(s.stack, w)
					descended = true
					break
				}
			}
			if !descended {
				s.lim[v] = next
				next++
				s.stack = s.stack[:len(s.stack)-1]
			}
		}
	}
}

// inSubtree reports whether w is in the subtree rooted at v
func (s *simplex) inSubtree(w, v int) bool { return s.low[v] <= s.lim[w] && s.lim[w] <= s.lim[v] }

// subtreeSide returns the child vertex of tree edge i, whose subtree is one
// side of the cut, and whether that child is the edge's tail.
func (s *simplex) subtreeSide(i int) (child int, childIsTail bool) {
	e := s.edges[i]
	if s.parent[e.head] == i {
		return e.head, false
	}
	return e.tail, true
}

// cutValues computes the cut value of every tree edge: weight of edges
// from the tail side to the head side minus the reverse. Summing ±weight
// of all edges incident to a subtree cancels edges inside it, so one
// postorder pass over the forest gives every value in O(V+E).
func (s *simplex) cutValues() []float32 {
	s.number()
	order, agg, cut := s.order, s.agg, s.cut
	for v := range s.n {
		order[s.lim[v]-1] = v // postorder: children before parents
		agg[v] = 0
	}
	for i := range cut {
		cut[i] = 0
	}
	for _, v := range order {
		for _, i := range s.adj[v] {
			e := s.edges[i]
			if e.tail == v {
				agg[v] += e.weight
			} else {
				agg[v] -= e.weight
			}
		}
		if p := s.parent[v]; p >= 0 {
			e := s.edges[p]
			parent := e.tail + e.head - v
			agg[parent] += agg[v]
			if e.tail == v {
				cut[p] = agg[v] // subtree is the tail side
			} else {
				cut[p] = -agg[v]
			}
		}
	}
	return cut
}

// negativeCutEdge returns the tree edge with the most negative cut value, or -1
func (s *simplex) negativeCutEdge() int {
	best, bestCut := -1, float32(0)
	for i, c := range s.cut {
		if c < bestCut && s.edges[i].tree {
			best, bestCut = i, c
		}
	}
	return best
}

// enteringEdge returns the non-tree edge with minimal slack that goes from
// the head side of the leaving edge to its tail side, scanning only the
// adjacency of the smaller side (marked by the last subtree call).
func (s *simplex) enteringEdge(side []int, sideIsTail bool) int {
	best, bestSlack := -1, math.MaxInt
	for _, v := range side {
		for _, i := range s.adj[v] {
			e := s.edges[i]
			if e.tree {
				continue
			}
			w := e.tail + e.head - v
			if s.mark[w] == s.epoch {
				continue // both ends on this side
			}
			// v is the tail exactly when this side is the tail side
			if (e.tail == v) == sideIsTail {
				continue
			}
			if sl := s.slack(i); sl < bestSlack {
				best, bestSlack = i, sl
			}
		}
	}
	return best
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
