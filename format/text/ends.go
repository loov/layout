package text

import (
	"cmp"
	"maps"
	"math"
	"slices"

	"github.com/loov/layout"
	"github.com/loov/layout/internal/draw"
)

// spreadSides gives every edge end on a side of a box a row, or on the
// top or bottom a column, of its own between the corners, as close to
// where it rounded as the others allow. The layout spreads ends apart,
// but rounds them to cells independently of the box, so ends could share
// a cell or land on a corner. An end whose run goes on straight keeps
// its place if it can; a moved end takes the bend before it along.
func (c *canvas) spreadSides(edges []*layout.Edge, paths [][]point) {
	sides := c.endsOnSides(edges, paths)

	// sides in the order of the nodes, as an edge can end on two of them
	// and moving one end can move the other
	keys := slices.Collect(maps.Keys(sides))
	order := map[*layout.Node]int{}
	for i, node := range c.l.Graph.Nodes {
		order[node] = i
	}
	slices.SortFunc(keys, func(a, b boxSide) int {
		return cmp.Or(cmp.Compare(order[a.node], order[b.node]), cmp.Compare(a.along, b.along), cmp.Compare(boolInt(a.after), boolInt(b.after)))
	})

	// the ends alone on their side, until their side is done
	alone := map[*point]bool{}
	for _, ends := range sides {
		if len(ends) == 1 && len(ends[0].also) == 0 {
			alone[&ends[0].path[ends[0].i]] = true
		}
	}
	for _, key := range keys {
		c.spreadSide(key, sides[key], alone)
	}
}

// spreadEnd is an edge end on a side of a box, see spreadSides
type spreadEnd struct {
	path   []point
	i, j   int     // the end and the bend before it
	k      int     // the point past the bend, -1 if none
	fixed  bool    // the run goes on straight past the bend
	toward int     // where the edge heads past the bend, along the side
	group  int     // the merged ends there, see layout.EdgePath.Merged
	exact  float64 // where the layout ends the edge along the side, in cells

	also []spreadEnd // further edges of a shared start, which follow it
}

// end returns the cell of the end
func (e spreadEnd) end() point { return e.path[e.i] }

// bend returns the cell of the bend before the end
func (e spreadEnd) bend() point { return e.path[e.j] }

// boxSide is a side of the box of node
type boxSide struct {
	node  *layout.Node
	along int  // the axis along the side: 1 for left and right
	after bool // right or bottom
}

// endsOnSides returns the ends of the edges, drawn along paths, on every
// side of a box that they come into straight from beyond it, put on the
// side, with the ends that merge with each in its also
func (c *canvas) endsOnSides(edges []*layout.Edge, paths [][]point) map[boxSide][]spreadEnd {
	sides := map[boxSide][]spreadEnd{}

	// merged ends on a side move as one, led by an end that goes on
	// straight when there is one, as that stays; leads holds where each
	// group's lead is in sides
	type group struct {
		side boxSide
		id   int
	}
	leads := map[group]int{}
	for k, path := range paths {
		if len(path) < 2 {
			continue
		}

		// the ends of a straight run share its bend, which they move
		// along the same way, so moving one end drags the other's off its
		// line: each gets one of its own, in the middle
		if last := len(path) - 1; last <= 2 && straight(path[0], path[last]) {
			mid := point{(path[0].X + path[last].X) / 2, (path[0].Y + path[last].Y) / 2}
			if last == 2 {
				mid = path[1]
			}
			path = []point{path[0], mid, mid, path[last]}
			paths[k] = path
		}
		last := len(path) - 1
		for _, e := range []struct {
			i, j, k int
			node    *layout.Node
			field   string
		}{{0, 1, 2, edges[k].From, edges[k].FromField}, {last, last - 1, last - 2, edges[k].To, edges[k].ToField}} {
			if c.hasField(e.node, e.field) {
				continue // the layout put it across from its field
			}
			b := c.boxes[e.node]
			at, bend := path[e.i], path[e.j]
			for along := range 2 {
				across := 1 - along
				after := bend.at(across) > b.Max.at(across)
				lo, hi, x := b.Min.at(along), b.Max.at(along), at.at(along)
				if hi-lo < 2 || x != bend.at(along) || !between(x, lo, hi) ||
					!after && bend.at(across) >= b.Min.at(across) {
					continue // not straight into the side from beyond it
				}

				// onto the side, also where a rounder outline curves inside
				path[e.i].set(across, b.Min.at(across))
				if after {
					path[e.i].set(across, b.Max.at(across))
				}
				end := spreadEnd{path: path, i: e.i, j: e.j, k: -1, fixed: true, toward: at.at(along)}
				if e.k >= 0 && e.k < len(path) {
					end.k = e.k
					end.fixed = path[e.k].at(along) == bend.at(along)
					end.toward = path[e.k].at(along)
				}
				end.group = c.l.Edges[k].Merged.End
				if e.i == 0 {
					end.group = c.l.Edges[k].Merged.Start
				}
				p := c.l.Edges[k].Path[0]
				if e.i != 0 {
					p = c.l.Edges[k].Path[len(c.l.Edges[k].Path)-1]
				}
				end.exact = float64((p.X - c.origin.X) / c.cellW)
				if along == 1 {
					end.exact = float64((p.Y - c.origin.Y) / c.cellH)
				}
				key := boxSide{e.node, along, after}
				if n, ok := leads[group{key, end.group}]; ok {
					lead := &sides[key][n]
					if end.fixed && !lead.fixed {
						end.also, lead.also = lead.also, nil
						*lead, end = end, *lead
					}
					lead.also = append(lead.also, end)
					break
				}
				if end.group != 0 {
					leads[group{key, end.group}] = len(sides[key])
				}
				sides[key] = append(sides[key], end)
				break
			}
		}
	}

	return sides
}

// spreadSide spreads the ends on the side key, see spreadSides; alone
// holds the ends alone on their side, until their side is done
func (c *canvas) spreadSide(key boxSide, ends []spreadEnd, alone map[*point]bool) {
	lo, hi := c.sideCells(key.node, key.along)

	// the other end of an edge may have moved its bend along already,
	// see below: head for where it is now
	for n := range ends {
		if e := &ends[n]; e.k >= 0 {
			e.fixed = e.path[e.k].at(key.along) == e.bend().at(key.along)
			e.toward = e.path[e.k].at(key.along)
		}
	}

	if len(ends) > hi-lo+1 {
		return // no room; keep them as they are
	}
	slices.SortStableFunc(ends, func(a, b spreadEnd) int {
		return cmp.Or(cmp.Compare(a.end().at(key.along), b.end().at(key.along)), cmp.Compare(a.toward, b.toward))
	})
	want := make([]int, len(ends))
	exact := make([]float64, len(ends))
	fixed := make([]bool, len(ends))
	for n, e := range ends {
		want[n], exact[n], fixed[n] = e.end().at(key.along), e.exact, e.fixed || c.boxBeside(e.end(), e.bend(), key.along)
	}
	packEnds(want, exact, fixed)

	// an end that turns toward a place along the side goes straight
	// there instead, past no other end
	for n, e := range ends {
		if fixed[n] || !between(e.toward, lo, hi) || c.boxAcross(e.end(), e.bend(), key.along, e.toward) {
			continue
		}
		past := slices.ContainsFunc(want, func(w int) bool {
			return w != want[n] && between(w, want[n], e.toward)
		})
		if past {
			continue
		}

		// a jog between two ends straightens from either: not from
		// ends balanced on their side when the other end, still to
		// come, is alone on its own
		moved := slices.Clone(want)
		moved[n] = e.toward
		if len(e.path) == 4 && alone[&e.path[3-e.i]] && unbalance(want, lo, hi) == 0 && unbalance(moved, lo, hi) > 0 {
			continue
		}
		want[n], fixed[n] = e.toward, true
	}

	// with spread, a cell between ends where the side has room, as
	// balanced as the ends were
	rows := spreadRows(want, fixed, lo, hi, 1)
	if c.spread && 2*len(want)-1 <= hi-lo+1 {
		wide := spreadRows(want, fixed, lo, hi, 2)
		moved := false
		for n := range want {
			moved = moved || fixed[n] && wide[n] != want[n]
		}
		if !moved {
			rows = wide
		}
	}
	for n, at := range rows {
		e := ends[n]
		delete(alone, &e.path[e.i])
		e.path[e.i].set(key.along, at)
		e.path[e.j].set(key.along, at)
		for _, f := range e.also {
			f.path[f.i].set(key.along, at)
			f.path[f.j].set(key.along, at)
		}
	}
}

// boxBeside reports whether a box lies right beside the run from the end
// at to the bend, along the axis along: moving the end, which moves the
// run, would run it into the box, and text would draw the edge around
// it. Compaction leaves runs beside boxes.
func (c *canvas) boxBeside(at, bend point, along int) bool {
	return c.boxAcross(at, bend, along, at.at(along)-1) || c.boxAcross(at, bend, along, at.at(along)+1)
}

// boxAcross reports whether the run from the end at to the bend, moved
// to to along the axis along, runs across a box
func (c *canvas) boxAcross(at, bend point, along, to int) bool {
	across := 1 - along
	step := sign(bend.at(across) - at.at(across))
	p := at
	p.set(along, to)
	for p.at(across) != bend.at(across) {
		p.set(across, p.at(across)+step)
		if q := c.at(p.X, p.Y); q != nil && q.solid && q.node != 0 {
			return true
		}
	}

	return false
}

// unbalance returns how far the ends at cells are off the middle of the
// side from lo to hi, in half cells
func unbalance(cells []int, lo, hi int) int {
	sum := 0
	for _, at := range cells {
		sum += 2*at - lo - hi
	}
	return abs(sum)
}

// packEnds moves the ends along a side, in order at cells, no further
// from the end before than their distance at the exact cells where the
// layout ends them, so that ends about a cell apart stay next to each
// other however they round. Fixed ends, which go on straight, stay.
func packEnds(cells []int, exact []float64, fixed []bool) {
	for n := 1; n < len(cells); n++ {
		if !fixed[n] {
			cells[n] = min(cells[n], cells[n-1]+max(1, int(math.Round(exact[n]-exact[n-1]))))
		}
	}
}

// spreadRows moves the ordered rows, or columns, want as little as
// possible so that they are at least gap apart within [lo, hi], with fixed
// ones moving only when they must. Shifting row n by n*gap turns this into
// ordering, solved by pooling adjacent violators.
func spreadRows(want []int, fixed []bool, lo, hi, gap int) []int {
	type block struct {
		sum, weight float64
		n           int
	}
	var blocks []block
	for n, row := range want {
		weight := 1.0
		if fixed[n] {
			weight = 1e6
		}
		blocks = append(blocks, block{float64(row-n*gap) * weight, weight, 1})
		for len(blocks) > 1 {
			a, b := blocks[len(blocks)-2], blocks[len(blocks)-1]
			if a.sum/a.weight <= b.sum/b.weight {
				break
			}
			blocks = append(blocks[:len(blocks)-2], block{a.sum + b.sum, a.weight + b.weight, a.n + b.n})
		}
	}

	rows := make([]int, 0, len(want))
	for _, b := range blocks {
		first := min(max(int(math.Round(b.sum/b.weight)), lo), hi-(len(want)-1)*gap)
		for range b.n {
			rows = append(rows, first+len(rows)*gap)
		}
	}

	return rows
}

// boolInt returns 1 for true and 0 for false
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// hasField reports whether node is a record with a field named field,
// see layout.Edge.FromField
func (c *canvas) hasField(node *layout.Node, field string) bool {
	box := c.l.Node(node)
	if field == "" || box.Shape != layout.Record {
		return false
	}
	return draw.ParseRecord(box.Label, c.sideways()).Field(field) != nil
}
