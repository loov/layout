package layout

import (
	"cmp"
	"math"
	"slices"
)

// spreadWaypoints moves interior path points that several edges share
// (detours around the same node) sideways so that the edges don't run on
// top of each other, in the order they head so that they don't cross.
func spreadWaypoints(edges []*ledge, pad Length) {
	type at struct {
		edge  *ledge
		index int
		next  Vector // original following point
	}
	shared := map[Vector][]at{}
	for _, edge := range edges {
		for i := 1; i+1 < len(edge.Path); i++ {
			shared[edge.Path[i]] = append(shared[edge.Path[i]], at{edge, i, edge.Path[i+1]})
		}
	}
	points := make([]Vector, 0, len(shared))
	for point, list := range shared {
		if len(list) > 1 {
			points = append(points, point)
		}
	}
	slices.SortFunc(points, func(a, b Vector) int {
		if a.X != b.X {
			return cmp.Compare(a.X, b.X)
		}
		return cmp.Compare(a.Y, b.Y)
	})
	for _, point := range points {
		list := shared[point]
		// spread along x, so the order of the following points' x keeps
		// the edges from crossing
		slices.SortStableFunc(list, func(a, b at) int { return cmp.Compare(a.next.X, b.next.X) })
		for i, a := range list {
			offset := (Length(i) - Length(len(list)-1)/2) * 2 * pad
			a.edge.Path[a.index] = point.Add(Vector{offset, 0})
		}
	}
}

// spreadEnds keeps the attachment points on every node at least minSep
// apart along the outline so that arrowheads don't stack, pushing the
// crowded ones apart around their mean direction. The ends of edges
// between ranks stay on the side facing the other rank, the top or the
// bottom, see spreadSides. Edges pinned to a port keep their point.
func spreadEnds(graph *lgraph, minSep Length) {
	spreadSides(graph, minSep)
	type end struct {
		edge  *ledge
		angle float64
		start bool
		fixed bool // a loop's attachment, or a lane's end, which stays where it is
	}
	byNode := map[*lnode][]end{}
	pairs := newPairs(graph)
	for _, edge := range graph.Edges {
		if len(edge.Path) < 2 {
			continue
		}
		angle := func(node *lnode, p Vector) float64 {
			return math.Atan2(float64(p.Y-node.Center.Y), float64(p.X-node.Center.X))
		}
		if edge.From == edge.To {
			byNode[edge.From] = append(byNode[edge.From],
				end{edge, angle(edge.From, edge.Path[0]), true, true},
				end{edge, angle(edge.From, edge.Path[len(edge.Path)-1]), false, true})
			continue
		}
		if edge.From.Center.Y != edge.To.Center.Y {
			continue // on a side, see spreadSides
		}
		// edges between neighbors that run side by side keep their lanes
		if len(edge.Path) == 2 && pairs.count[pairs.key(edge)] > 1 {
			if edge.freeStart() {
				byNode[edge.From] = append(byNode[edge.From], end{edge, angle(edge.From, edge.Path[0]), true, true})
			}
			if edge.freeEnd() {
				byNode[edge.To] = append(byNode[edge.To], end{edge, angle(edge.To, edge.Path[1]), false, true})
			}
			continue
		}
		if edge.freeStart() {
			byNode[edge.From] = append(byNode[edge.From], end{edge, angle(edge.From, edge.Path[1]), true, false})
		}
		if edge.freeEnd() {
			byNode[edge.To] = append(byNode[edge.To], end{edge, angle(edge.To, edge.Path[len(edge.Path)-2]), false, false})
		}
	}
	for node, ends := range byNode {
		if len(ends) < 2 {
			continue
		}
		slices.SortFunc(ends, func(a, b end) int { return cmp.Compare(a.angle, b.angle) })
		// start the sequence after the largest gap so that the ±π seam
		// never falls between neighbors
		gap, at := ends[0].angle+2*math.Pi-ends[len(ends)-1].angle, len(ends)-1
		for i := 1; i < len(ends); i++ {
			if d := ends[i].angle - ends[i-1].angle; d > gap {
				gap, at = d, i-1
			}
		}
		for i := range at + 1 {
			ends[i].angle += 2 * math.Pi
		}
		ends = append(ends[at+1:], ends[:at+1]...)
		// angle step from the arc length on the smaller radius, so that
		// it is enough along the flat sides of wide nodes too
		step := float64(minSep) / float64(min(node.Radius.X, node.Radius.Y))
		// as many as go around the node at most, or the push carries the
		// ends of one side around the corners onto the next
		step = min(step, math.Pi/float64(len(ends)))
		for i := 1; i < len(ends); i++ {
			d := ends[i].angle - ends[i-1].angle
			if d >= step {
				continue
			}
			switch {
			case ends[i].fixed && !ends[i-1].fixed:
				ends[i-1].angle -= step - d
			case ends[i-1].fixed && !ends[i].fixed:
				ends[i].angle += step - d
			case !ends[i].fixed:
				// split the push, moving everything before along
				for k := range i {
					ends[k].angle -= (step - d) / 2
				}
				ends[i].angle += (step - d) / 2
			}
		}
		for _, e := range ends {
			if e.fixed {
				continue
			}
			p := node.Boundary(node.Center.Add(Vector{Length(math.Cos(e.angle)), Length(math.Sin(e.angle))}))
			if e.start {
				e.edge.Path[0] = p
			} else {
				e.edge.Path[len(e.edge.Path)-1] = p
			}
		}
	}
}

// spreadSides puts the ends of edges between ranks on the side of their
// node that faces the other rank, the top or the bottom, where they head
// to, at least minSep apart where the side has the room, as the ranks
// they come from are, in the order they head
func spreadSides(graph *lgraph, minSep Length) {
	type end struct {
		edge  *ledge
		start bool
		want  Length // where the line toward the next point crosses the side
	}
	type side struct {
		node   *lnode
		bottom bool
	}
	sides := map[side][]end{}
	add := func(node *lnode, edge *ledge, start bool, next Vector) {
		bottom := next.Y > node.Center.Y
		dy := absLength(next.Y - node.Center.Y)
		want := node.Center.X
		if dy > 0 {
			want += (next.X - node.Center.X) * node.Radius.Y / dy
		}
		k := side{node, bottom}
		sides[k] = append(sides[k], end{edge, start, want})
	}
	for _, edge := range graph.Edges {
		if edge.From == edge.To || len(edge.Path) < 2 || edge.From.Center.Y == edge.To.Center.Y {
			continue
		}
		if edge.freeStart() {
			add(edge.From, edge, true, edge.Path[1])
		}
		if edge.freeEnd() {
			add(edge.To, edge, false, edge.Path[len(edge.Path)-2])
		}
	}
	for k, ends := range sides {
		node := k.node
		slices.SortStableFunc(ends, func(a, b end) int { return cmp.Compare(a.want, b.want) })
		gap := min(minSep, 2*node.Radius.X/Length(len(ends)+1))
		want, weight := make([]Length, len(ends)), make([]float64, len(ends))
		for i, e := range ends {
			want[i], weight[i] = e.want, 1
		}
		reach := node.Radius.X - gap/2
		xs := separate(want, weight, gap, node.Center.X-reach, node.Center.X+reach)
		for i, e := range ends {
			p := Vector{xs[i], node.Center.Y + node.halfHeightAt(xs[i])}
			if !k.bottom {
				p.Y = node.Center.Y - node.halfHeightAt(xs[i])
			}
			if e.start {
				e.edge.Path[0] = p
			} else {
				e.edge.Path[len(e.edge.Path)-1] = p
			}
		}
	}
}

// loopPath draws a self-loop on the right side of the node
// pairs numbers the edges between each pair of nodes, which run side by
// side in pinned and force layouts
type pairs struct {
	order        map[*lnode]int // keeps a pair's direction the same both ways
	count, index map[[2]*lnode]int
}

func newPairs(graph *lgraph) *pairs {
	p := &pairs{order: map[*lnode]int{}, count: map[[2]*lnode]int{}, index: map[[2]*lnode]int{}}
	for i, node := range graph.Nodes {
		p.order[node] = i
	}
	for _, edge := range graph.Edges {
		if edge.From != edge.To {
			p.count[p.key(edge)]++
		}
	}
	return p
}

func (p *pairs) key(edge *ledge) [2]*lnode {
	if p.order[edge.From] > p.order[edge.To] {
		return [2]*lnode{edge.To, edge.From}
	}
	return [2]*lnode{edge.From, edge.To}
}

// shift returns the offset of the next edge between its nodes, spacing
// apart, across the line from the first node of the pair to the second
func (p *pairs) shift(edge *ledge, spacing Length) Vector {
	key := p.key(edge)
	n := p.count[key]
	if n < 2 {
		return Vector{}
	}
	k := p.index[key]
	p.index[key]++
	d := key[1].Center.Sub(key[0].Center)
	length := Length(math.Hypot(float64(d.X), float64(d.Y)))
	if length == 0 {
		return Vector{}
	}
	offset := (Length(k) - Length(n-1)/2) * spacing / length
	return Vector{X: -d.Y * offset, Y: d.X * offset}
}
