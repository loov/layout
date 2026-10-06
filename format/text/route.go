package text

import (
	"container/heap"
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
func (c *canvas) route(edge *layout.Edge, cells [][2]int) [][2]int {
	if len(cells) < 2 {
		return cells
	}
	id, merged := c.merged[edge]
	if !merged {
		id = c.ids + 1 // see drawEdge
	}
	if c.clear(cells, id, merged) {
		return cells
	}
	last := len(cells) - 1
	starts := c.endCells(cells[0], edge.From, edge.FromField)
	goals := c.endCells(cells[last], edge.To, edge.ToField)
	if route := c.search(starts, goals, id); route != nil {
		return route
	}
	return cells
}

// endCell is a cell an edge can end at, with the direction from the box
// out of it, and the cost of ending there instead of where it was
type endCell struct {
	at   [2]int
	out  uint8
	cost int
}

// endCells returns the cells on the sides of the box of node where an
// edge can end instead of at end; only end itself for dots, fields and
// ends not on a side
func (c *canvas) endCells(end [2]int, node *layout.Node, field string) []endCell {
	b, ok := c.boxes[node]
	out := uint8(0)
	switch {
	case !ok:
	case end[1] == b[1] && end[0] > b[0] && end[0] < b[2]:
		out = up
	case end[1] == b[3] && end[0] > b[0] && end[0] < b[2]:
		out = down
	case end[0] == b[0] && end[1] > b[1] && end[1] < b[3]:
		out = left
	case end[0] == b[2] && end[1] > b[1] && end[1] < b[3]:
		out = right
	}
	if out == 0 || c.hasField(node, field) || c.l.Node(node).Shape == layout.PointShape {
		var cells []endCell
		for _, dir := range []uint8{up, down, left, right} {
			if out == 0 || dir == out {
				cells = append(cells, endCell{end, dir, 0})
			}
		}
		return cells
	}
	// the cells of the side it is on, then of the other sides, which an
	// end takes only when it can't leave the side it is on
	var cells []endCell
	for _, side := range []uint8{out, up, down, left, right} {
		if side == out && len(cells) > 0 {
			continue
		}
		cost := 0
		if side != out {
			cost = 50
		}
		along, at := 0, b[1]
		switch side {
		case down:
			at = b[3]
		case left:
			along, at = 1, b[0]
		case right:
			along, at = 1, b[2]
		}
		for i := b[along] + 1; i < b[along+2]; i++ {
			cell := [2]int{i, at}
			if along == 1 {
				cell = [2]int{at, i}
			}
			cells = append(cells, endCell{cell, side, cost + 2*max(cell[0]-end[0], end[0]-cell[0], cell[1]-end[1], end[1]-cell[1])})
		}
	}
	return cells
}

// clear reports whether an edge with id can run along cells, its ends at
// the first and last, without sharing a run, an arrowhead or a box;
// merged edges share their ends, and the arrowheads before them
func (c *canvas) clear(cells [][2]int, id edgeID, merged bool) bool {
	ends := map[[2]int]bool{cells[0]: true, cells[len(cells)-1]: true}
	if merged {
		var all [][2]int
		for i := 0; i+1 < len(cells); i++ {
			c.steps(cells[i], cells[i+1], func(from, to [2]int, dir uint8) {
				if len(all) == 0 {
					all = append(all, from)
				}
				all = append(all, to)
			})
		}
		if len(all) >= 2 {
			ends[all[1]], ends[all[len(all)-2]] = true, true
		}
	}
	ok := true
	for i := 0; ok && i+1 < len(cells); i++ {
		c.steps(cells[i], cells[i+1], func(from, to [2]int, dir uint8) {
			ok = ok && c.free(from, dir, id, ends[from]) && c.free(to, opposite(dir), id, ends[to])
		})
	}
	return ok
}

// steps calls step for every cell to cell step from a to b, as walk
// draws them: along the row first, then the column
func (c *canvas) steps(a, b [2]int, step func(from, to [2]int, dir uint8)) {
	for x := a[0]; x != b[0]; x += sign(b[0] - x) {
		dir := uint8(right)
		if b[0] < x {
			dir = left
		}
		step([2]int{x, a[1]}, [2]int{x + dx(dir), a[1]}, dir)
	}
	for y := a[1]; y != b[1]; y += sign(b[1] - y) {
		dir := uint8(down)
		if b[1] < y {
			dir = up
		}
		step([2]int{b[0], y}, [2]int{b[0], y + dy(dir)}, dir)
	}
}

// free reports whether an edge with id can draw the arm toward dir in
// the cell at p: one that no other edge draws nor ends at, see reserve,
// off boxes and arrowheads unless p ends the edge
func (c *canvas) free(p [2]int, dir uint8, id edgeID, end bool) bool {
	q := c.at(p[0], p[1])
	if q == nil {
		return false
	}
	if owner, ok := c.reserved[p]; ok && owner != id {
		return false
	}
	if q.solid {
		return end
	}
	return q.lines&dir == 0 || q.owner[bits.TrailingZeros8(dir)] == id
}

// search returns the cheapest cells from one of starts to one of goals,
// as corners, for an edge with id to run along without sharing a run, see
// route, or nil when there is no way
func (c *canvas) search(starts, goals []endCell, id edgeID) [][2]int {
	const turn = 4 // a turn costs as much as this many steps
	if len(c.rows) == 0 {
		return nil
	}
	w, h := len(c.rows[0]), len(c.rows)
	dirs := []uint8{up, down, left, right}
	state := func(p [2]int, d int) int { return (p[1]*w+p[0])*4 + d }
	cost := map[int]int{}
	prev := map[int]int{}
	goal := map[[2]int]endCell{}
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
		p := [2]int{k % w, k / w}
		if _, ok := goal[p]; ok && prev[item.state] >= 0 {
			end = item.state
			break
		}
		start := prev[item.state] < 0
		for nd, dir := range dirs {
			if dir == opposite(dirs[d]) || start && nd != d {
				continue
			}
			to := [2]int{p[0] + dx(dir), p[1] + dy(dir)}
			if to[0] < 0 || to[1] < 0 || to[0] >= w || to[1] >= h {
				continue
			}
			g, isGoal := goal[to]
			if !c.free(p, dir, id, start) || !c.free(to, opposite(dir), id, isGoal && dir == opposite(g.out)) {
				continue
			}
			next := item.cost + 1
			if nd != d {
				next += turn
			}
			if isGoal && dir == opposite(g.out) {
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
	var cells [][2]int
	for k := end; k >= 0; k = prev[k] {
		cell := k / 4
		cells = append(cells, [2]int{cell % w, cell / w})
	}
	// back to front, keeping only the corners
	var corners [][2]int
	for i := len(cells) - 1; i >= 0; i-- {
		if n := len(corners); n >= 2 {
			a, b := corners[n-2], corners[n-1]
			if (a[0] == b[0]) == (b[0] == cells[i][0]) && (a[1] == b[1]) == (b[1] == cells[i][1]) {
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
// move off, for them, see edgeID, so that the edges drawn before them
// keep off. Edges that end at one such cell draw as one, merged.
func (c *canvas) reserve(edges []*layout.Edge, paths [][][2]int) {
	c.reserved = map[[2]int]edgeID{}
	by := map[[2]int]*layout.Edge{}
	id := c.ids
	for i, edge := range edges {
		if edge.Invisible || len(paths[i]) < 2 {
			continue
		}
		id++ // see drawEdge
		ends := []bool{c.hasField(edge.From, edge.FromField), c.hasField(edge.To, edge.ToField)}
		for k, end := range [][2]int{paths[i][0], paths[i][len(paths[i])-1]} {
			if !ends[k] {
				continue
			}
			own, merged := c.merged[edge]
			if !merged {
				own = id
			}
			if other, ok := c.reserved[end]; ok && other != own {
				c.merged[by[end]], c.merged[edge] = other, other
				continue
			}
			c.reserved[end], by[end] = own, edge
		}
	}
}
