package hier

import (
	"math"
	"slices"
)

// Align picks how Position spreads nodes along their ranks.
type Align int

const (
	// Balanced centers nodes among their neighbors, keeping long edges
	// straight and the layout narrow, see positionSimplex
	Balanced Align = iota
	// Left lines nodes up with their first neighbor in the rank above
	// and packs them to the left
	Left
	// Right lines nodes up with their last neighbor in the rank above
	// and packs them to the right
	Right
)

// Position assigns node centers: rows by rank height, columns by network
// simplex when balanced and by Brandes-Köpf when aligned left or right.
// With straighten, virtual nodes of long edges move so that the edges run
// diagonally; orthogonal routing wants them where positioning aligns
// them, in line with an end, to save bends. With fans, a balanced node
// goes over the middle of its children, for edges that merge into a fork.
func Position(graph *Graph, straighten, fans bool, align Align) {
	PositionInitial(graph)
	if len(graph.Nodes) == 0 {
		return
	}

	switch align {
	case Left, Right:
		xs := brandesKoepf(graph, true, align == Left, true)
		for _, node := range graph.Nodes {
			node.Center.X = xs[node.ID]
		}
	default:
		positionSimplex(graph, fans)
	}
	if straighten {
		StraightenChains(graph)
	}
	flushLeft(graph)
	AlignClusterBorders(graph)
}

// balancedKoepf returns the average of the two median x of the four
// Brandes-Köpf layouts, after lining them up with the narrowest one
func balancedKoepf(graph *Graph) []float32 {
	// four alignments: up/down x left/right
	var xs [4][]float32
	for i := range xs {
		up, left := i < 2, i%2 == 0
		xs[i] = brandesKoepf(graph, up, left, false)
	}

	// align layouts to the narrowest one, then take the average of the two medians
	width := func(x []float32) float32 { return slices.Max(x) - slices.Min(x) }
	narrow := 0
	for i := range xs {
		if width(xs[i]) < width(xs[narrow]) {
			narrow = i
		}
	}
	for i := range xs {
		var shift float32
		if i%2 == 0 {
			shift = slices.Min(xs[narrow]) - slices.Min(xs[i])
		} else {
			shift = slices.Max(xs[narrow]) - slices.Max(xs[i])
		}
		for k := range xs[i] {
			xs[i][k] += shift
		}
	}

	x := make([]float32, len(graph.Nodes))
	for _, node := range graph.Nodes {
		v := []float32{xs[0][node.ID], xs[1][node.ID], xs[2][node.ID], xs[3][node.ID]}
		slices.Sort(v)
		x[node.ID] = (v[1] + v[2]) / 2
	}
	return x
}

// StraightenChains moves virtual nodes towards the midpoint of their
// neighbors along the edge, as far as the room between the nodes beside
// them allows, so that long edges run diagonally instead of hooking at
// the end. Order within the ranks is kept, so crossings don't change.
func StraightenChains(graph *Graph) {
	graph.assignPos()
	for range 10 {
		moved := false
		for _, layer := range graph.ByRank {
			for i, node := range layer {
				if !node.Virtual || node.BorderLeft || node.BorderRight || len(node.In) != 1 || len(node.Out) != 1 {
					continue
				}
				// align the anchors, where the edges actually pass
				in, out := node.In[0], node.Out[0]
				want := (in.Center.X+in.Anchor+out.Center.X+out.Anchor)/2 - node.Anchor
				if i > 0 {
					left := layer[i-1]
					want = max(want, left.Center.X+left.Radius.X+node.Radius.X)
				}
				if i+1 < len(layer) {
					right := layer[i+1]
					want = min(want, right.Center.X-right.Radius.X-node.Radius.X)
				}
				if d := want - node.Center.X; d > 0.5 || d < -0.5 {
					node.Center.X = want
					moved = true
				}
			}
		}
		if !moved {
			break
		}
	}
}

// PositionInitial assigns rows and packs nodes left to right
func PositionInitial(graph *Graph) {
	top := float32(0)
	for _, nodes := range graph.ByRank {
		left := float32(0)

		halfrow := float32(0)
		for _, node := range nodes {
			halfrow = max(halfrow, node.Radius.Y)
		}

		top += halfrow
		for _, node := range nodes {
			node.Center.X = left + node.Radius.X
			node.Center.Y = top
			left = node.Center.X + node.Radius.X
		}
		top += halfrow
	}
}

// brandesKoepf computes x coordinates for one of the four alignment directions.
// up: align to neighbors in the rank above (otherwise below);
// left: compact towards the left (otherwise right);
// first: align to the first neighbor towards that side, not the median.
func brandesKoepf(graph *Graph, up, left, first bool) []float32 {
	n := graph.NodeCount()

	// orient the problem so that we always align "up" and compact "left"
	layers := make([]Nodes, len(graph.ByRank))
	for i, layer := range graph.ByRank {
		layers[i] = slices.Clone(layer)
		if !left {
			slices.Reverse(layers[i])
		}
	}
	if !up {
		slices.Reverse(layers)
	}
	upper := func(node *Node) Nodes {
		if up {
			return node.In
		}
		return node.Out
	}

	pos := make([]int, n)
	layerOf := make([]Nodes, n)
	for _, layer := range layers {
		for i, node := range layer {
			pos[node.ID] = i
			layerOf[node.ID] = layer
		}
	}
	// upper neighbors sorted by position
	nbrs := make([]Nodes, n)
	for _, node := range graph.Nodes {
		nbrs[node.ID] = slices.Clone(upper(node))
		slices.SortFunc(nbrs[node.ID], func(a, b *Node) int { return pos[a.ID] - pos[b.ID] })
	}

	// type 1 conflicts: non-inner segments crossing inner (virtual-virtual) segments
	conflict := map[[2]ID]bool{}
	inner := func(v *Node) *Node {
		if v.Virtual && len(nbrs[v.ID]) == 1 && nbrs[v.ID][0].Virtual {
			return nbrs[v.ID][0]
		}
		return nil
	}
	for i := 1; i < len(layers); i++ {
		layer, above := layers[i], layers[i-1]
		k0, l := 0, 0
		for l1, v := range layer {
			u := inner(v)
			if l1 != len(layer)-1 && u == nil {
				continue
			}
			k1 := len(above) - 1
			if u != nil {
				k1 = pos[u.ID]
			}
			for ; l <= l1; l++ {
				for _, w := range nbrs[layer[l].ID] {
					if k := pos[w.ID]; k < k0 || k > k1 {
						conflict[[2]ID{w.ID, layer[l].ID}] = true
					}
				}
			}
			k0 = k1
		}
	}

	// vertical alignment
	root := make([]*Node, n)
	align := make([]*Node, n)
	for _, node := range graph.Nodes {
		root[node.ID], align[node.ID] = node, node
	}
	for i := 1; i < len(layers); i++ {
		r := -1
		for _, v := range layers[i] {
			d := len(nbrs[v.ID])
			if d == 0 {
				continue
			}
			// the median neighbors, or the first one towards the side
			// the layout packs to
			lo, hi := (d-1)/2, d/2
			if first {
				lo, hi = 0, 0
			}
			for m := lo; m <= hi; m++ {
				if align[v.ID] != v {
					break
				}
				u := nbrs[v.ID][m]
				if !conflict[[2]ID{u.ID, v.ID}] && r < pos[u.ID] {
					align[u.ID] = v
					root[v.ID] = root[u.ID]
					align[v.ID] = root[v.ID]
					r = pos[u.ID]
				}
			}
		}
	}

	// horizontal compaction: longest path over the block graph
	// (blocks = roots, edges between horizontally adjacent blocks). Blocks
	// line up the nodes' anchors, so a node reaches Radius.X past its
	// center, which is Anchor before the anchor; mirrored when compacting
	// right
	side := float32(1)
	if !left {
		side = -1
	}
	before := func(node *Node) float32 { return node.Radius.X + side*node.Anchor }
	after := func(node *Node) float32 { return node.Radius.X - side*node.Anchor }
	x := make([]float32, n)
	placed := make([]bool, n)
	var place func(v *Node)
	place = func(v *Node) {
		if placed[v.ID] {
			return
		}
		placed[v.ID] = true
		for w := v; ; w = align[w.ID] {
			if p := pos[w.ID]; p > 0 {
				prev := layerOf[w.ID][p-1]
				u := root[prev.ID]
				place(u)
				x[v.ID] = max(x[v.ID], x[u.ID]+after(prev)+before(w))
			}
			if align[w.ID] == v {
				break
			}
		}
	}
	for _, node := range graph.Nodes {
		place(root[node.ID])
	}
	// node centers from the anchors
	final := make([]float32, n)
	for _, node := range graph.Nodes {
		final[node.ID] = side*x[root[node.ID].ID] - node.Anchor
	}
	return final
}

// flushLeft moves the graph so that the leftmost node touches x = 0
func flushLeft(graph *Graph) {
	if len(graph.Nodes) == 0 {
		return
	}
	minleft := float32(math.Inf(1))
	for _, node := range graph.Nodes {
		minleft = min(minleft, node.Center.X-node.Radius.X)
	}
	for _, node := range graph.Nodes {
		node.Center.X -= minleft
	}
}

// widthWeight is what the width of the layout costs against the length of
// edges along the ranks, and settleWeight what it costs a node to be away
// from where balanced Brandes-Köpf puts it, see positionSimplex
const (
	widthWeight  = 1
	settleWeight = 1.0 / 64
)

// positionSimplex sets x by network simplex on an auxiliary graph, as
// Gansner et al. section 4.2 do: every edge gets a vertex below both of its
// ends, so that the cost of the edge is how far apart its ends are, and
// neighbors in a rank stay apart by their widths. Edges between virtual
// nodes weigh more, so that long edges run straight, and so do the edges
// of labels, so that a label stays beside its edge. The width of the
// layout costs too, so that a long edge bends rather than holding a gap
// open across every rank.
//
// Many layouts can cost the same, such as with a node anywhere between
// its two children; a slight pull towards where balanced Brandes-Köpf
// puts each node picks the one that centers nodes among their neighbors.
func positionSimplex(graph *Graph, fans bool) {
	n := graph.NodeCount()
	edges := 0
	for _, node := range graph.Nodes {
		edges += len(node.Out)
	}
	// two more vertices bound the ranks on the left and the right, and
	// the edge between them costs the width of the drawing; the left one
	// is also where the pulls are measured from; edges along a rank, see
	// Flat, get a vertex each after the pulls
	flats := graph.Flat
	if !graph.PullFlat {
		flats = nil
	}
	s := &simplex{n: int32(n + edges + 2 + n + len(flats))}
	lft, rgt := int32(n+edges), int32(n+edges+1)
	s.addEdge(lft, rgt, widthWeight, 0)
	settle := balancedKoepf(graph)
	low := float32(math.Inf(1)) // the left side, where nodes reach to
	for _, node := range graph.Nodes {
		low = min(low, settle[node.ID]-node.Radius.X)
	}
	for _, node := range graph.Nodes {
		if node.Virtual {
			continue
		}
		at := settle[node.ID] - low + node.Anchor
		pull := int32(n+edges+2) + int32(node.ID)
		// costs how far the node is from at past the left side
		s.addEdge(pull, int32(node.ID), settleWeight, int32(math.Round(float64(at))))
		s.addEdge(pull, lft, settleWeight, 0)
	}
	for _, layer := range graph.ByRank {
		if len(layer) > 0 {
			first, last := layer[0], layer[len(layer)-1]
			s.addEdge(lft, int32(first.ID), 0, int32(math.Ceil(float64(first.Radius.X+first.Anchor))))
			s.addEdge(int32(last.ID), rgt, 0, int32(math.Ceil(float64(last.Radius.X-last.Anchor))))
		}
		for i := 1; i < len(layer); i++ {
			a, b := layer[i-1], layer[i]
			// anchors line up, a node reaches Radius.X past its center
			gap := a.Radius.X - a.Anchor + b.Radius.X + b.Anchor
			// the fans of different nodes keep apart: children of one
			// parent each, which has others
			fan := func(node *Node) bool { return !node.Virtual && len(node.In) == 1 && len(node.In[0].Out) > 1 }
			if fan(a) && fan(b) && a.In[0] != b.In[0] {
				gap += graph.FamilyGap
			}
			s.addEdge(int32(a.ID), int32(b.ID), 0, int32(math.Ceil(float64(gap))))
		}
	}
	// a virtual node with an anchor carries a label beside its edge, which
	// keeps straighter than a long edge leaving a node, the label beside
	// it rather than between its bends
	label := func(node *Node) bool { return node.Virtual && node.Anchor != 0 }
	weight := func(src, dst *Node) float32 {
		omega := float32(1)
		switch {
		case src.Virtual && dst.Virtual:
			omega = 8
		case label(src) || label(dst):
			omega = 4
		case src.Virtual || dst.Virtual:
			omega = 2
		}
		return omega * graph.Weight(src, dst)
	}
	// packed ends of edges on a side of a node line up from its anchor
	// on, in the order of the nodes they head to, see Graph.EndGap; an
	// edge measures from where its ends attach
	from, to := map[[2]ID]float32{}, map[[2]ID]float32{}
	spread := func(node *Node, ends Nodes, at map[[2]ID]float32, key func(other *Node) [2]ID) {
		if graph.EndGap == 0 || node.Virtual || len(ends) < 2 {
			return
		}
		ends = slices.Clone(ends)
		slices.SortStableFunc(ends, func(a, b *Node) int { return a.Pos - b.Pos })
		gap := min(graph.EndGap, node.EndRoom/float32(len(ends)-1))
		for i, other := range ends {
			at[key(other)] = float32(i) * gap
		}
	}
	for _, node := range graph.Nodes {
		spread(node, node.Out, from, func(dst *Node) [2]ID { return [2]ID{node.ID, dst.ID} })
		spread(node, node.In, to, func(src *Node) [2]ID { return [2]ID{src.ID, node.ID} })
	}
	v := int32(n)
	for _, src := range graph.Nodes {
		for _, dst := range src.Out {
			w := weight(src, dst)
			// |x(src) + from - x(dst) - to| at the least cost
			k := [2]ID{src.ID, dst.ID}
			d := int32(math.Round(float64(from[k] - to[k])))
			s.addEdge(v, int32(src.ID), w, max(0, -d))
			s.addEdge(v, int32(dst.ID), w, max(0, d))
			v++
		}
	}
	// an edge along a rank costs how far apart its ends are too, with
	// PullFlat, which keeps a node with only such edges beside the ones it
	// has them to
	v = int32(n + edges + 2 + n)
	for _, flat := range flats {
		w := graph.Weight(flat[0], flat[1])
		s.addEdge(v, int32(flat[0].ID), w, 0)
		s.addEdge(v, int32(flat[1].ID), w, 0)
		v++
	}
	s.run()

	// fans: a node with several children goes over the middle of them,
	// from the bottom up, as far as its neighbors in the rank let it; the
	// children are the nodes right below, or below ranks of labels only,
	// along edges that merge into the fork: not long edges past other
	// nodes, nor edges with labels
	real := make([]bool, len(graph.ByRank))
	for r, layer := range graph.ByRank {
		real[r] = slices.ContainsFunc(layer, func(node *Node) bool { return !node.Virtual })
	}
	child := func(c *Node) *Node {
		for c.Virtual && len(c.Out) == 1 {
			if real[c.Rank] || label(c) {
				return nil
			}
			c = c.Out[0]
		}
		return c
	}
	x := make([]float32, n)
	for _, node := range graph.Nodes {
		x[node.ID] = float32(s.rank[node.ID])
	}
	lo, hi := float32(s.rank[lft]), float32(s.rank[rgt])
	for r := len(graph.ByRank) - 1; r >= 0 && fans; r-- {
		layer := graph.ByRank[r]
		for i, node := range layer {
			if node.Virtual {
				continue
			}
			first, last, children := float32(math.Inf(1)), float32(math.Inf(-1)), 0
			for _, out := range node.Out {
				if c := child(out); c != nil && !c.Virtual {
					first, last = min(first, x[c.ID]), max(last, x[c.ID])
					children++
				}
			}
			if children < 2 {
				continue
			}
			left, right := lo+node.Radius.X+node.Anchor, hi-node.Radius.X+node.Anchor
			if i > 0 {
				a := layer[i-1]
				left = x[a.ID] + float32(math.Ceil(float64(a.Radius.X-a.Anchor+node.Radius.X+node.Anchor)))
			}
			if i+1 < len(layer) {
				b := layer[i+1]
				right = x[b.ID] - float32(math.Ceil(float64(node.Radius.X-node.Anchor+b.Radius.X+b.Anchor)))
			}
			if left <= right {
				x[node.ID] = min(max((first+last)/2, left), right)
			}
		}
	}
	// the edges stretched over ranks of labels only run on in line with
	// the node they end at, so that forks fork right below their node
	for r, layer := range graph.ByRank {
		if real[r] || !fans {
			continue
		}
		for i, node := range layer {
			if !node.Virtual || label(node) || len(node.Out) != 1 {
				continue
			}
			end := child(node.Out[0])
			if end == nil {
				continue
			}
			want := x[end.ID]
			if i > 0 {
				a := layer[i-1]
				want = max(want, x[a.ID]+float32(math.Ceil(float64(a.Radius.X-a.Anchor+node.Radius.X+node.Anchor))))
			}
			if i+1 < len(layer) {
				b := layer[i+1]
				want = min(want, x[b.ID]-float32(math.Ceil(float64(node.Radius.X-node.Anchor+b.Radius.X+b.Anchor))))
			}
			x[node.ID] = want
		}
	}
	for _, node := range graph.Nodes {
		node.Center.X = x[node.ID] - node.Anchor
	}
}
