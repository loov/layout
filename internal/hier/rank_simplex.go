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
// Pivots are incremental: cut values are updated along the tree path
// between the entering edge's endpoints and only the affected subtree is
// renumbered, following Gansner et al. section 2.4.
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

	s := &simplex{n: int32(verts)}
	for _, src := range graph.Nodes {
		for _, dst := range src.Out {
			u, v := index[src.ID], index[dst.ID]
			if u == v || v == source || u == sink {
				continue // inside a group, or contradicting a min/max pin
			}
			s.addEdge(int32(u), int32(v), graph.Weight(src, dst))
		}
	}
	// zero weight edges keep the artificial source first and sink last
	for v := range verts {
		if source >= 0 && v != source {
			s.addEdge(int32(source), int32(v), 0)
		}
		if sink >= 0 && v != sink && v != source {
			s.addEdge(int32(v), int32(sink), 0)
		}
	}
	s.run()

	for _, node := range graph.Nodes {
		node.Rank = int(s.rank[index[node.ID]])
	}
}

// simplex holds the network simplex state as flat arrays (structure of
// arrays, int32 indexes, CSR adjacency) so that the pivot loops stream
// through memory instead of chasing pointers.
type simplex struct {
	n int32

	// edges
	tail, head []int32
	weight     []float32
	tree       []bool
	cut        []float32

	// vertices
	rank []int32
	// adjacency in CSR form: edges of vertex v are adj[adjStart[v]:adjStart[v+1]]
	adjStart []int32
	adj      []int32

	// spanning forest numbering, see number; buffers reused across pivots
	low, lim, parent, cursor []int32
	order                    []int32
	agg                      []float32
	stack                    []int32
	mark                     []int32 // visited marks by epoch, see subtree
	epoch                    int32
	nodes                    []int32 // subtree result buffer
}

func (s *simplex) addEdge(tail, head int32, weight float32) {
	s.tail = append(s.tail, tail)
	s.head = append(s.head, head)
	s.weight = append(s.weight, weight)
	s.tree = append(s.tree, false)
}

func (s *simplex) edgeCount() int { return len(s.tail) }

// other returns the endpoint of edge i that is not v
func (s *simplex) other(i int32, v int32) int32 { return s.tail[i] + s.head[i] - v }

// adjacent returns the edge indexes incident to v
func (s *simplex) adjacent(v int32) []int32 { return s.adj[s.adjStart[v]:s.adjStart[v+1]] }

func (s *simplex) run() {
	// build CSR adjacency
	m := int32(s.edgeCount())
	s.adjStart = make([]int32, s.n+1)
	for i := range m {
		s.adjStart[s.tail[i]+1]++
		s.adjStart[s.head[i]+1]++
	}
	for v := range s.n {
		s.adjStart[v+1] += s.adjStart[v]
	}
	s.adj = make([]int32, 2*m)
	fill := make([]int32, s.n)
	copy(fill, s.adjStart[:s.n])
	for i := range m {
		for _, v := range [2]int32{s.tail[i], s.head[i]} {
			s.adj[fill[v]] = i
			fill[v]++
		}
	}

	s.initRank()
	s.feasibleTree()

	s.low = make([]int32, s.n)
	s.lim = make([]int32, s.n)
	s.parent = make([]int32, s.n)
	s.cursor = make([]int32, s.n)
	s.order = make([]int32, s.n)
	s.agg = make([]float32, s.n)
	s.cut = make([]float32, m)
	s.stack = make([]int32, 0, s.n)
	s.mark = make([]int32, s.n)
	s.cutValues()

	for iter := 0; iter < 8*int(m)+8; iter++ {
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
func (s *simplex) smallerSide(leave int32) (side []int32, sideIsTail bool) {
	child, childIsTail := s.subtreeSide(leave)
	if 2*(s.lim[child]-s.low[child]+1) <= s.n {
		return s.subtree(child, leave), childIsTail
	}
	return s.subtree(s.other(leave, child), leave), !childIsTail
}

// exchange replaces tree edge leave with non-tree edge enter, updating
// ranks, cut values and the subtree numbering incrementally. side is the
// smaller side of the cut, see smallerSide.
func (s *simplex) exchange(leave, enter int32, side []int32, sideIsTail bool) {
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
	lca := s.treeUpdate(s.tail[enter], s.head[enter], cutLeave, true)
	if other := s.treeUpdate(s.head[enter], s.tail[enter], cutLeave, false); other != lca {
		panic("hier: network simplex tree update mismatch")
	}
	s.cut[enter] = -cutLeave
	s.cut[leave] = 0

	// the subtree below leave moves under enter's outside endpoint
	child, _ := s.subtreeSide(leave)
	s.tree[leave] = false
	s.tree[enter] = true
	s.moveSubtree(child, enter)
}

// moveSubtree fixes the postorder numbering after the subtree of child
// (numbers [a, a+k-1]) has been re-attached through edge enter. The smaller
// of the two parts is walked again; the larger keeps its numbering, fixed
// arithmetically by cutting a range out and inserting it before the new
// parent. Postorder numbers of a component are contiguous, and other
// components are only shifted, so they stay consistent.
func (s *simplex) moveSubtree(child, enter int32) {
	a, k := s.low[child], s.lim[child]-s.low[child]+1
	last := a + k - 1
	inside, outside := s.tail[enter], s.head[enter]
	if !s.inSubtree(inside, child) {
		inside, outside = outside, inside
	}

	// the component of the exchange; its root keeps the largest number
	root := child
	for s.parent[root] >= 0 {
		root = s.other(s.parent[root], root)
	}
	compSize := s.lim[root] - s.low[root] + 1

	if 2*k <= compSize {
		// move the child subtree under outside; everything else keeps its
		// relative order
		p := s.lim[outside] // insertion point, right before the new parent
		if p > last {
			p -= k
		}
		for v := range s.n {
			lim := s.lim[v]
			if a <= lim && lim <= last {
				continue // moved subtree, renumbered below
			}
			if lim > last {
				lim -= k
			}
			if lim >= p {
				lim += k
			}
			s.lim[v] = lim
			low := s.low[v]
			if low > last {
				low -= k
			}
			if low > p { // == p: the new parent (or its ancestors) now starts at the subtree
				low += k
			}
			s.low[v] = low
		}
		s.parent[inside] = enter
		s.renumber(inside, p)
		return
	}

	// the child subtree is the larger part: make child the component's root,
	// keep the subtree's numbering compressed to the start of the component
	// and renumber the rest of the component, rooted at outside, before
	// inside's number
	c0 := s.low[root]
	m := compSize - k
	p := s.lim[inside] - (a - c0)
	for v := range s.n {
		lim := s.lim[v]
		if lim < a || lim > last {
			continue // rest of the component, renumbered below; or another component
		}
		lim -= a - c0
		if lim >= p {
			lim += m
		}
		s.lim[v] = lim
		low := s.low[v] - (a - c0)
		if low > p {
			low += m
		}
		s.low[v] = low
	}
	s.parent[child] = -1
	s.parent[outside] = enter
	s.renumber(outside, p)
}

// treeUpdate walks from v towards the root until w is inside v's subtree,
// adjusting the cut value of every parent edge on the way, and returns the
// vertex where it stopped (the lowest common ancestor of v and w).
func (s *simplex) treeUpdate(v, w int32, cutvalue float32, dir bool) int32 {
	for !s.inSubtree(w, v) {
		p := s.parent[v]
		if p < 0 {
			panic("hier: network simplex endpoints in different trees")
		}
		d := dir
		if v != s.tail[p] {
			d = !d
		}
		if d {
			s.cut[p] += cutvalue
		} else {
			s.cut[p] -= cutvalue
		}
		v = s.other(p, v)
	}
	return v
}

// subtree returns the vertices reachable from root over tree edges without
// crossing edge skip
func (s *simplex) subtree(root, skip int32) []int32 {
	s.epoch++
	s.mark[root] = s.epoch
	s.stack = append(s.stack[:0], root)
	nodes := append(s.nodes[:0], root)
	for len(s.stack) > 0 {
		v := s.stack[len(s.stack)-1]
		s.stack = s.stack[:len(s.stack)-1]
		for _, i := range s.adjacent(v) {
			if !s.tree[i] || i == skip {
				continue
			}
			w := s.other(i, v)
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
func (s *simplex) renumber(root int32, next int32) {
	parentEdge := s.parent[root]
	s.epoch++
	s.mark[root] = s.epoch
	s.cursor[root] = s.adjStart[root]
	s.stack = append(s.stack[:0], root)
	s.low[root] = next
	for len(s.stack) > 0 {
		v := s.stack[len(s.stack)-1]
		end := s.adjStart[v+1]
		descended := false
		for s.cursor[v] < end {
			i := s.adj[s.cursor[v]]
			s.cursor[v]++
			if !s.tree[i] || i == parentEdge {
				continue
			}
			w := s.other(i, v)
			if s.mark[w] != s.epoch {
				s.mark[w] = s.epoch
				s.cursor[w] = s.adjStart[w]
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

func (s *simplex) length(i int32) int32 { return s.rank[s.head[i]] - s.rank[s.tail[i]] }
func (s *simplex) slack(i int32) int32  { return s.length(i) - 1 }

// initRank assigns longest-path ranks, a feasible starting point
func (s *simplex) initRank() {
	s.rank = make([]int32, s.n)
	indeg := make([]int32, s.n)
	for _, h := range s.head {
		indeg[h]++
	}
	var queue []int32
	for v := range s.n {
		if indeg[v] == 0 {
			queue = append(queue, v)
		}
	}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		for _, i := range s.adjacent(v) {
			if s.tail[i] != v {
				continue
			}
			h := s.head[i]
			s.rank[h] = max(s.rank[h], s.rank[v]+1)
			indeg[h]--
			if indeg[h] == 0 {
				queue = append(queue, h)
			}
		}
	}
}

// feasibleTree grows a spanning forest of tight edges, shifting ranks of the
// partial tree to make the closest non-tree edge tight when it gets stuck.
func (s *simplex) feasibleTree() {
	inTree := make([]bool, s.n)
	m := int32(s.edgeCount())
	for root := range s.n {
		if inTree[root] {
			continue
		}
		inTree[root] = true
		for {
			// grow along tight edges
			changed := true
			for changed {
				changed = false
				for i := range m {
					if s.tree[i] || inTree[s.tail[i]] == inTree[s.head[i]] || s.slack(i) != 0 {
						continue
					}
					s.tree[i] = true
					inTree[s.tail[i]], inTree[s.head[i]] = true, true
					changed = true
				}
			}
			// find the incident non-tree edge with minimal slack
			best, bestSlack := int32(-1), int32(math.MaxInt32)
			for i := range m {
				if s.tree[i] || inTree[s.tail[i]] == inTree[s.head[i]] {
					continue
				}
				if sl := s.slack(i); sl < bestSlack {
					best, bestSlack = i, sl
				}
			}
			if best < 0 {
				break // component complete
			}
			// shift the tree side so the edge becomes tight; inTree also marks
			// finished components, so walk this tree explicitly
			delta := bestSlack
			if inTree[s.head[best]] {
				delta = -bestSlack
			}
			for _, v := range s.treeNodes(root) {
				s.rank[v] += delta
			}
		}
	}
}

// treeNodes returns the vertices connected to root by tree edges
func (s *simplex) treeNodes(root int32) []int32 {
	seen := make([]bool, s.n)
	seen[root] = true
	nodes := []int32{root}
	for k := 0; k < len(nodes); k++ {
		v := nodes[k]
		for _, i := range s.adjacent(v) {
			if !s.tree[i] {
				continue
			}
			w := s.other(i, v)
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
	for v := range s.n {
		s.parent[v] = -1
		s.low[v] = 0
		s.cursor[v] = s.adjStart[v]
	}
	next := int32(1)
	for root := range s.n {
		if s.low[root] != 0 {
			continue
		}
		s.stack = append(s.stack[:0], root)
		s.low[root] = next
		for len(s.stack) > 0 {
			v := s.stack[len(s.stack)-1]
			end := s.adjStart[v+1]
			descended := false
			for s.cursor[v] < end {
				i := s.adj[s.cursor[v]]
				s.cursor[v]++
				if !s.tree[i] {
					continue
				}
				w := s.other(i, v)
				if s.low[w] == 0 {
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
}

// inSubtree reports whether w is in the subtree rooted at v
func (s *simplex) inSubtree(w, v int32) bool { return s.low[v] <= s.lim[w] && s.lim[w] <= s.lim[v] }

// subtreeSide returns the child vertex of tree edge i, whose subtree is one
// side of the cut, and whether that child is the edge's tail.
func (s *simplex) subtreeSide(i int32) (child int32, childIsTail bool) {
	if s.parent[s.head[i]] == i {
		return s.head[i], false
	}
	return s.tail[i], true
}

// cutValues computes the cut value of every tree edge: weight of edges
// from the tail side to the head side minus the reverse. Summing ±weight
// of all edges incident to a subtree cancels edges inside it, so one
// postorder pass over the forest gives every value in O(V+E).
func (s *simplex) cutValues() {
	s.number()
	for v := range s.n {
		s.order[s.lim[v]-1] = v // postorder: children before parents
		s.agg[v] = 0
	}
	for i := range s.cut {
		s.cut[i] = 0
	}
	for _, v := range s.order {
		for _, i := range s.adjacent(v) {
			if s.tail[i] == v {
				s.agg[v] += s.weight[i]
			} else {
				s.agg[v] -= s.weight[i]
			}
		}
		if p := s.parent[v]; p >= 0 {
			parent := s.other(p, v)
			s.agg[parent] += s.agg[v]
			if s.tail[p] == v {
				s.cut[p] = s.agg[v] // subtree is the tail side
			} else {
				s.cut[p] = -s.agg[v]
			}
		}
	}
}

// negativeCutEdge returns the tree edge with the most negative cut value, or -1
func (s *simplex) negativeCutEdge() int32 {
	best, bestCut := int32(-1), float32(0)
	for i, c := range s.cut {
		if c < bestCut && s.tree[i] {
			best, bestCut = int32(i), c
		}
	}
	return best
}

// enteringEdge returns the non-tree edge with minimal slack that goes from
// the head side of the leaving edge to its tail side, scanning only the
// adjacency of the smaller side (marked by the last subtree call).
func (s *simplex) enteringEdge(side []int32, sideIsTail bool) int32 {
	best, bestSlack := int32(-1), int32(math.MaxInt32)
	for _, v := range side {
		for _, i := range s.adjacent(v) {
			if s.tree[i] {
				continue
			}
			w := s.other(i, v)
			if s.mark[w] == s.epoch {
				continue // both ends on this side
			}
			// v is the tail exactly when this side is the tail side
			if (s.tail[i] == v) == sideIsTail {
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
		lo := int32(math.MaxInt32)
		for _, v := range nodes {
			seen[v] = true
			lo = min(lo, s.rank[v])
		}
		for _, v := range nodes {
			s.rank[v] -= lo
		}
	}
}
