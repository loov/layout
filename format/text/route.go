package text

import (
	"container/heap"
	"iter"
	"math/bits"

	"github.com/loov/layout"
)

// route returns the cells of an edge, see edgeCells, along which it runs
// on no run of another edge drawn so far, see canvas.line: its own when
// they share none, or else the cheapest way around them, in steps and
// turns. Its ends can move along the sides of the boxes they are on,
// away from the ends of others. Lines still cross other edges, and
// merged edges, which are one, share runs. When there is no way around,
// the edge keeps its cells.
func (c *canvas) route(edge *layout.Edge, cells []point) []point {
	if len(cells) < 2 {
		return cells
	}
	id, merged := c.edgeOwner(edge, c.ids+1) // see drawEdge
	last := len(cells) - 1
	if c.clear(cells, id, merged) && c.onSide(cells[0], cells[1], edge.From) && c.onSide(cells[last], cells[last-1], edge.To) {
		return cells
	}
	starts := c.endCells(cells[0], cells[1], edge.From, edge.FromField)
	goals := c.endCells(cells[last], cells[last-1], edge.To, edge.ToField)
	if route := c.search(starts, goals, id); route != nil {
		return route
	}
	return cells
}

// edgeOwner returns the id that edge draws with: that of its merged
// group, else own; and whether it is merged
func (c *canvas) edgeOwner(edge *layout.Edge, own edgeID) (edgeID, bool) {
	if id, ok := c.merged[edge]; ok {
		return id, true
	}
	return own, false
}

// onSide reports whether end, heading to next along the edge, is where
// an edge can end on the box of node, see sideCells, or off the box, where
// it stays
func (c *canvas) onSide(end, next point, node *layout.Node) bool {
	if _, ok := c.boxes[node]; !ok || c.l.Node(node).Shape == layout.PointShape {
		return true
	}
	out := c.side(end, next, node)
	if out == 0 {
		return true // off the box
	}
	for p := range c.sideSpots(node, out) {
		if p == end {
			return true
		}
	}
	return false
}

// sideSpots yields the cells on side of the box of node where an edge can
// end, see sideCells, in order along the side
func (c *canvas) sideSpots(node *layout.Node, side uint8) iter.Seq[point] {
	return func(yield func(point) bool) {
		b := c.boxes[node]
		along, at := 0, b.Min.Y
		switch side {
		case down:
			at = b.Max.Y
		case left:
			along, at = 1, b.Min.X
		case right:
			along, at = 1, b.Max.X
		}

		lo, hi := c.sideCells(node, along)
		for i := lo; i <= hi; i++ {
			p := point{i, at}
			if along == 1 {
				p = point{at, i}
			}
			if !yield(p) {
				return
			}
		}
	}
}

// side returns the side of the box of node that end is on, heading to
// next along the edge: by where it is, or on a corner, or on a diamond
// of only a few rows, by where it heads; 0 off the box
func (c *canvas) side(end, next point, node *layout.Node) uint8 {
	b := c.boxes[node]
	if !b.contains(end) {
		return 0
	}
	inX, inY := b.insideX(end.X), b.insideY(end.Y)
	if c.l.Node(node).Shape != layout.Diamond {
		switch {
		case end.Y == b.Min.Y && inX:
			return up
		case end.Y == b.Max.Y && inX:
			return down
		case end.X == b.Min.X && inY:
			return left
		case end.X == b.Max.X && inY:
			return right
		}
	}
	switch d := next.sub(end); {
	case abs(d.X) >= abs(d.Y) && d.X > 0:
		return right
	case abs(d.X) >= abs(d.Y) && d.X < 0:
		return left
	case d.Y < 0:
		return up
	case d.Y > 0:
		return down
	}

	return 0
}

// endCell is a cell an edge can end at, with the direction from the box
// out of it, and the cost of ending there instead of where it was
type endCell struct {
	at   point
	out  uint8
	cost int
}

// endCells returns the cells on the sides of the box of node where an
// edge can end instead of at end, toward next along the edge, the side it
// is on first; only end itself for dots and fields
func (c *canvas) endCells(end, next point, node *layout.Node, field string) []endCell {
	_, ok := c.boxes[node]
	out := uint8(0)
	if ok {
		out = c.side(end, next, node)
	}
	if !ok || c.hasField(node, field) || c.l.Node(node).Shape == layout.PointShape {
		var cells []endCell
		for _, dir := range []uint8{up, down, left, right} {
			if out == 0 || dir == out {
				cells = append(cells, endCell{end, dir, 0})
			}
		}
		return cells
	}

	// the cells of the side it is on, see side, then of the other sides,
	// which an end takes only when it can't leave the side it is on
	var cells []endCell
	for _, side := range []uint8{up, down, left, right} {
		cost := 0
		if out != 0 && side != out {
			cost = 50
		}
		for cell := range c.sideSpots(node, side) {
			d := cell.sub(end)
			cells = append(cells, endCell{cell, side, cost + 2*max(abs(d.X), abs(d.Y))})
		}
	}

	return cells
}

// clear reports whether an edge with id can run along cells, its ends at
// the first and last, without sharing a run, an arrowhead or a box;
// merged edges share their ends, and the arrowheads before them
func (c *canvas) clear(cells []point, id edgeID, merged bool) bool {
	ends := map[point]bool{cells[0]: true, cells[len(cells)-1]: true}
	if merged {
		all := c.pathCells(nil, cells)
		if len(all) >= 2 {
			ends[all[1]], ends[all[len(all)-2]] = true, true
		}
	}
	ok := true
	c.pathSteps(cells, func(from, to point, dir uint8) {
		ok = ok && c.passes(from, to, dir, id, ends[from], ends[to])
	})
	return ok
}

// steps calls step for every cell to cell step from a to b, as walk
// draws them: along the row first, then the column
func (c *canvas) steps(a, b point, step func(from, to point, dir uint8)) {
	for x := a.X; x != b.X; x += sign(b.X - x) {
		dir := uint8(right)
		if b.X < x {
			dir = left
		}
		step(point{x, a.Y}, point{x + dx(dir), a.Y}, dir)
	}

	for y := a.Y; y != b.Y; y += sign(b.Y - y) {
		dir := uint8(down)
		if b.Y < y {
			dir = up
		}
		step(point{b.X, y}, point{b.X, y + dy(dir)}, dir)
	}
}

// pathSteps calls step for every cell to cell step along path, see steps
func (c *canvas) pathSteps(path []point, step func(from, to point, dir uint8)) {
	for i := 0; i+1 < len(path); i++ {
		c.steps(path[i], path[i+1], step)
	}
}

// pathCells appends the cells along path, from its first to its last, to
// cells; none when path takes no step
func (c *canvas) pathCells(cells []point, path []point) []point {
	first := true
	c.pathSteps(path, func(from, to point, _ uint8) {
		if first {
			cells, first = append(cells, from), false
		}
		cells = append(cells, to)
	})
	return cells
}

// free reports whether an edge with id can draw the arm toward dir in
// the cell at p: one that no other edge draws nor ends at, see reserve,
// off boxes and arrowheads unless p ends the edge
func (c *canvas) free(p point, dir uint8, id edgeID, end bool) bool {
	q := c.at(p.X, p.Y)
	if q == nil {
		return false
	}
	if owner, ok := c.reserved[p]; ok && owner != id {
		return false
	}
	if q.solid {
		return end
	}
	return q.lines&dir == 0 || q.ownerOn(dir) == id
}

// passes reports whether an edge with id can draw the step from from to
// to toward dir, see free; fromEnd and toEnd tell whether the cells end
// the edge
func (c *canvas) passes(from, to point, dir uint8, id edgeID, fromEnd, toEnd bool) bool {
	return c.free(from, dir, id, fromEnd) && c.free(to, opposite(dir), id, toEnd)
}

// crosses reports whether a line of an edge with id that enters the
// cell at p toward dir crosses the line of another edge there
func (c *canvas) crosses(p point, dir uint8, id edgeID) bool {
	q := c.at(p.X, p.Y)
	if q == nil || q.solid {
		return false
	}
	across := []uint8{left, right}
	if dir == left || dir == right {
		across = []uint8{up, down}
	}
	for _, d := range across {
		if q.lines&d != 0 && q.ownerOn(d) != id {
			return true
		}
	}
	return false
}

// search returns the cheapest cells from one of starts to one of goals,
// as corners, for an edge with id to run along without sharing a run, see
// route, or nil when there is no way
func (c *canvas) search(starts, goals []endCell, id edgeID) []point {
	const turn = 4 // a turn costs as much as this many steps

	// and crossing the cell of another edge's marker as this many
	const marker = 8

	// and crossing the line of another edge as this many
	const crossing = 8
	if len(c.rows) == 0 {
		return nil
	}
	w, h := len(c.rows[0]), len(c.rows)
	dirs := []uint8{up, down, left, right}
	state := func(p point, d int) int { return (p.Y*w+p.X)*4 + d }
	cost := map[int]int{}
	prev := map[int]int{}
	goal := map[point]endCell{}
	for _, g := range goals {
		goal[g.at] = g
	}
	var queue costQueue
	for _, s := range starts {
		d := bits.TrailingZeros8(s.out)
		k := state(s.at, d)
		if old, ok := cost[k]; !ok || s.cost < old {
			cost[k], prev[k] = s.cost, -1
			heap.Push(&queue, costItem{k, s.cost})
		}
	}

	end := -1
	for queue.Len() > 0 {
		item := heap.Pop(&queue).(costItem)
		if item.cost > cost[item.state] {
			continue
		}
		k, d := item.state/4, item.state%4
		p := point{k % w, k / w}
		if _, ok := goal[p]; ok && prev[item.state] >= 0 {
			end = item.state
			break
		}
		start := prev[item.state] < 0
		// a straight cell past the start and before the goal, where the
		// markers of the ends go, see marker
		second := !start && prev[prev[item.state]] < 0
		for nd, dir := range dirs {
			if dir == opposite(dirs[d]) || (start || second) && nd != d {
				continue
			}
			to := p.step(dir)
			if to.X < 0 || to.Y < 0 || to.X >= w || to.Y >= h {
				continue
			}
			g, isGoal := goal[to]
			isGoal = isGoal && dir == opposite(g.out) && nd == d && !start
			if !c.passes(p, to, dir, id, start, isGoal) {
				continue
			}
			next := item.cost + 1
			if nd != d {
				next += turn
			}
			if c.marked(to, id) {
				next += marker
			}
			if c.crosses(to, dir, id) {
				next += crossing
			}
			if isGoal {
				next += g.cost
			}
			nk := state(to, nd)
			if old, ok := cost[nk]; ok && old <= next {
				continue
			}
			cost[nk], prev[nk] = next, item.state
			heap.Push(&queue, costItem{nk, next})
		}
	}

	if end < 0 {
		return nil
	}
	var cells []point
	for k := end; k >= 0; k = prev[k] {
		cell := k / 4
		cells = append(cells, point{cell % w, cell / w})
	}

	// back to front, keeping only the corners
	var corners []point
	for i := len(cells) - 1; i >= 0; i-- {
		if n := len(corners); n >= 2 {
			a, b := corners[n-2], corners[n-1]
			if inLine(a, b, cells[i]) {
				corners[n-1] = cells[i]
				continue
			}
		}
		corners = append(corners, cells[i])
	}

	return corners
}

type costItem struct{ state, cost int }

// costQueue is a heap of search states by cost
type costQueue []costItem

func (q costQueue) Len() int           { return len(q) }
func (q costQueue) Less(i, j int) bool { return q[i].cost < q[j].cost }
func (q costQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *costQueue) Push(x any)        { *q = append(*q, x.(costItem)) }
func (q *costQueue) Pop() any {
	old := *q
	item := old[len(old)-1]
	*q = old[:len(old)-1]
	return item
}

// reserve keeps the cells where edges end at fields, which they can't
// move off, and the two cells out from them, for them, see edgeID, so
// that the edges drawn before them keep off. Edges that end at one such
// cell draw as one, merged. It marks the cells of other markers too, see
// markCell.
func (c *canvas) reserve(edges []*layout.Edge, paths [][]point) {
	c.reserved = map[point]edgeID{}
	c.markers = map[point]edgeID{}
	by := map[point]*layout.Edge{}
	id := c.ids
	for i, edge := range edges {
		if edge.Invisible || len(paths[i]) < 2 {
			continue
		}
		id++ // see drawEdge
		path, last := paths[i], len(paths[i])-1
		ends := []bool{c.hasField(edge.From, edge.FromField), c.hasField(edge.To, edge.ToField)}
		head := edge.ArrowHead
		if head == layout.ArrowDefault && edge.Directed {
			head = layout.ArrowNormal
		}
		markers := []bool{edge.ArrowTail != layout.ArrowDefault && edge.ArrowTail != layout.ArrowNone,
			head != layout.ArrowDefault && head != layout.ArrowNone}
		for k, end := range []point{path[0], path[last]} {
			next, node := path[1], edge.From
			if k == 1 {
				next, node = path[last-1], edge.To
			}
			own, _ := c.edgeOwner(edge, id)
			if !ends[k] {
				c.markCell(end, next, node, own, markers[k])
				continue
			}
			if other, ok := c.reserved[end]; ok && other != own {
				c.merged[by[end]], c.merged[edge] = other, other
				continue
			}
			c.reserved[end], by[end] = own, edge
			// and the two cells out from it, where its line leaves straight
			// and a marker goes, see search
			if out := c.side(end, next, node); out != 0 {
				cell := end
				for range 2 {
					cell = cell.step(out)
					if _, ok := c.reserved[cell]; !ok {
						c.reserved[cell] = own
					}
				}
			}
		}
	}
}

// markCell keeps the cell out from an end with a marker, where the marker
// goes when the line reaches it straight, see marker, for the edge with
// id: a run of another edge across it puts the marker on the border.
// Unlike reserved cells, others can still cross it when there is no
// other way, at a cost, see search.
func (c *canvas) markCell(end, next point, node *layout.Node, id edgeID, marker bool) {
	out := c.side(end, next, node)
	if !marker || out == 0 {
		return
	}

	// only when the line runs straight through it
	if d := (next.X-end.X)*dx(out) + (next.Y-end.Y)*dy(out); d < 2 || !straight(end, next) {
		return
	}
	cell := end.step(out)
	if _, ok := c.markers[cell]; !ok {
		c.markers[cell] = id
	}
}

// marked reports whether p is the marker cell of an edge other than id,
// see markCell
func (c *canvas) marked(p point, id edgeID) bool {
	owner, ok := c.markers[p]
	return ok && owner != id
}
