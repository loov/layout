package hier

import (
	"container/heap"
	"math"
)

// RankNetworkSimplex assigns ranks minimizing the total weighted edge length
// Σ weight(e)·(rank(dst) − rank(src)) subject to every edge spanning at least
// its minimum length, using the network simplex method of Gansner et al.
//
// Nodes in a SameRank group are contracted into a single vertex so they end
// up on the same rank; MinRank and MaxRank nodes are contracted with an
// artificial source or sink connected to every other vertex, which other
// vertices may share a rank with, as with Graphviz rank=min and rank=max. Edges that
// would contradict those constraints, including one edge of every cycle
// that contracting the groups closes, are ignored here and end up flat or
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
	find := func(i int) int {
		for rep[i] != i {
			rep[i] = rep[rep[i]]
			i = rep[i]
		}
		return i
	}
	unify := func(group Nodes) {
		if len(group) < 2 {
			return
		}
		for _, node := range group[1:] {
			rep[find(int(node.ID))] = find(int(group[0].ID))
		}
	}
	for _, group := range graph.SameRank {
		unify(group)
	}
	if len(graph.MinRank) > 0 {
		unify(graph.MinRank)
	}
	if len(graph.MaxRank) > 0 {
		unify(graph.MaxRank)
	}
	var (
		verts, source, sink int
		index               []int // contracted vertex index by node id
		edges               []rankEdge
	)
	contract := func() {
		verts, index = 0, make([]int, graph.NodeCount())
		for i := range index {
			index[i] = -1
		}
		for _, node := range graph.Nodes {
			r := find(int(node.ID))
			if index[r] < 0 {
				index[r] = verts
				verts++
			}
			index[node.ID] = index[r]
		}
		source, sink = -1, -1
		if len(graph.MinRank) > 0 {
			source = index[graph.MinRank[0].ID]
		}
		if len(graph.MaxRank) > 0 {
			sink = index[graph.MaxRank[0].ID]
		}
		edges = edges[:0]
		for _, src := range graph.Nodes {
			for _, dst := range src.Out {
				u, v := index[src.ID], index[dst.ID]
				if u == v || v == source || u == sink {
					continue // inside a group, or contradicting a min/max pin
				}
				edges = append(edges, rankEdge{int32(u), int32(v), graph.Weight(src, dst), graph.MinLen(src, dst)})
			}
		}
	}
	contract()
	// contracting groups can close cycles; a cycle of edges that may be
	// flat puts its vertices on one rank, any other cycle contradicts the
	// groups and loses an edge
	if comp, count := flatComponents(verts, edges); count < verts {
		first := make([]*Node, count)
		for _, node := range graph.Nodes {
			if c := comp[index[node.ID]]; first[c] == nil {
				first[c] = node
			} else {
				unify(Nodes{first[c], node})
			}
		}
		contract()
	}
	edges = dropBackEdges(verts, edges)

	s := &simplex{n: int32(verts)}
	for _, e := range edges {
		s.addEdge(e.tail, e.head, e.weight, e.minlen)
	}
	// zero weight edges keep the artificial source first and sink last
	for v := range verts {
		if source >= 0 && v != source {
			s.addEdge(int32(source), int32(v), 0, 0)
		}
		if sink >= 0 && v != sink && v != source {
			s.addEdge(int32(v), int32(sink), 0, 0)
		}
	}
	s.run()

	for _, node := range graph.Nodes {
		node.Rank = int(s.rank[index[node.ID]])
	}
}

// rankEdge is an edge between contracted vertices
type rankEdge struct {
	tail, head int32
	weight     float32
	minlen     int32
}

// flatComponents numbers the strongly connected components made by the
// edges that may be flat, returning the component of every vertex and the
// number of components
func flatComponents(n int, edges []rankEdge) (comp []int32, count int) {
	out := make([][]int32, n)
	for _, e := range edges {
		if e.minlen <= 0 {
			out[e.tail] = append(out[e.tail], e.head)
		}
	}
	comp = make([]int32, n)
	order, low := make([]int32, n), make([]int32, n)
	for v := range comp {
		comp[v] = -1
	}
	var stack []int32
	next := int32(0)
	var visit func(v int32)
	visit = func(v int32) {
		next++
		order[v], low[v] = next, next
		stack = append(stack, v)
		for _, w := range out[v] {
			if order[w] == 0 {
				visit(w)
				low[v] = min(low[v], low[w])
			} else if comp[w] < 0 {
				low[v] = min(low[v], order[w])
			}
		}
		if low[v] == order[v] {
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				comp[w] = int32(count)
				if w == v {
					break
				}
			}
			count++
		}
	}
	for v := range n {
		if order[v] == 0 {
			visit(int32(v))
		}
	}
	return comp, count
}

// dropBackEdges removes the back edges of a depth first search, leaving
// the edges acyclic
func dropBackEdges(n int, edges []rankEdge) []rankEdge {
	out := make([][]int32, n)
	for i, e := range edges {
		out[e.tail] = append(out[e.tail], int32(i))
	}
	const (
		unseen = iota
		active
		done
	)
	state := make([]int8, n)
	back := make([]bool, len(edges))
	var visit func(v int32)
	visit = func(v int32) {
		state[v] = active
		for _, i := range out[v] {
			switch w := edges[i].head; state[w] {
			case active:
				back[i] = true
			case unseen:
				visit(w)
			}
		}
		state[v] = done
	}
	for v := range n {
		if state[v] == unseen {
			visit(int32(v))
		}
	}
	kept := edges[:0]
	for i, e := range edges {
		if !back[i] {
			kept = append(kept, e)
		}
	}
	return kept
}

// simplex holds the network simplex state as flat arrays (structure of
// arrays, int32 indexes, CSR adjacency) so that the pivot loops stream
// through memory instead of chasing pointers.
type simplex struct {
	n int32

	// edges
	tail, head []int32
	weight     []float32
	minlen     []int32
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

func (s *simplex) addEdge(tail, head int32, weight float32, minlen int32) {
	s.tail = append(s.tail, tail)
	s.head = append(s.head, head)
	s.weight = append(s.weight, weight)
	s.minlen = append(s.minlen, minlen)
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
func (s *simplex) slack(i int32) int32  { return s.length(i) - s.minlen[i] }

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
			s.rank[h] = max(s.rank[h], s.rank[v]+s.minlen[i])
			indeg[h]--
			if indeg[h] == 0 {
				queue = append(queue, h)
			}
		}
	}
}

// feasibleTree grows a spanning forest of tight edges like Prim's
// algorithm, shifting the ranks of the partial tree to make the closest
// edge leaving it tight when none is. Shifting the tree changes the slack
// of every edge leaving it by the same amount, down for the edges leaving
// from their tail and up for those leaving from their head, so two heaps
// keyed by the slack without the shift find the closest edge.
func (s *simplex) feasibleTree() {
	inTree := make([]bool, s.n)
	var members []int32
	// out holds the edges leaving the tree from their tail, in those
	// leaving from their head
	var out, in edgeHeap
	for root := range s.n {
		if inTree[root] {
			continue
		}
		// the members of the tree sit shift ranks below their s.rank,
		// which is applied once the tree is complete
		shift := int32(0)
		members = members[:0]
		add := func(v int32) {
			inTree[v] = true
			members = append(members, v)
			s.rank[v] -= shift
			for _, i := range s.adjacent(v) {
				t, h := s.tail[i], s.head[i]
				if inTree[t] && inTree[h] {
					continue
				}
				e := edgeKey{s.rank[h] - s.rank[t] - s.minlen[i], i}
				if t == v {
					heap.Push(&out, e)
				} else {
					heap.Push(&in, e)
				}
			}
		}
		add(root)
		for {
			for out.Len() > 0 && inTree[s.head[out[0].edge]] {
				heap.Pop(&out)
			}
			for in.Len() > 0 && inTree[s.tail[in[0].edge]] {
				heap.Pop(&in)
			}
			if out.Len() == 0 && in.Len() == 0 {
				break
			}
			var next int32
			if in.Len() == 0 || (out.Len() > 0 && edgeKey.less(edgeKey{out[0].key - shift, out[0].edge}, edgeKey{in[0].key + shift, in[0].edge})) {
				e := heap.Pop(&out).(edgeKey)
				shift += e.key - shift
				s.tree[e.edge] = true
				next = s.head[e.edge]
			} else {
				e := heap.Pop(&in).(edgeKey)
				shift -= e.key + shift
				s.tree[e.edge] = true
				next = s.tail[e.edge]
			}
			add(next)
		}
		for _, v := range members {
			s.rank[v] += shift
		}
	}
}

// edgeKey orders edges by a key, then by index
type edgeKey struct{ key, edge int32 }

func (a edgeKey) less(b edgeKey) bool { return a.key < b.key || (a.key == b.key && a.edge < b.edge) }

// edgeHeap is a min-heap of edgeKeys, see container/heap
type edgeHeap []edgeKey

func (h edgeHeap) Len() int           { return len(h) }
func (h edgeHeap) Less(i, j int) bool { return h[i].less(h[j]) }
func (h edgeHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *edgeHeap) Push(x any)        { *h = append(*h, x.(edgeKey)) }
func (h *edgeHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
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
