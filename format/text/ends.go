package text

import (
	"cmp"
	"github.com/loov/layout"
	"github.com/loov/layout/internal/draw"
	"maps"
	"math"
	"slices"
)

// spreadSides gives every edge end on a side of a box a row, or on the
// top or bottom a column, of its own between the corners, as close to
// where it rounded as the others allow. The layout spreads ends apart,
// but rounds them to cells independently of the box, so ends could share
// a cell or land on a corner. An end whose run goes on straight keeps
// its place if it can; a moved end takes the bend before it along.
func (c *canvas) spreadSides(edges []*layout.Edge, paths [][][2]int) {
	type end struct {
		path   [][2]int
		i, j   int     // the end and the bend before it
		fixed  bool    // the run goes on straight past the bend
		toward int     // where the edge heads past the bend, along the side
		group  int     // the merged ends there, see layout.EdgePath.Merged
		exact  float64 // where the layout ends the edge along the side, in cells

		also []end // further edges of a shared start, which follow it
	}
	type side struct {
		node  *layout.Node
		along int  // the axis along the side: 1 for left and right
		after bool // right or bottom
	}
	sides := map[side][]end{}
	// merged ends on a side move as one, led by an end that goes on
	// straight when there is one, as that stays; leads holds where each
	// group's lead is in sides
	type group struct {
		side side
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
		if last := len(path) - 1; last <= 2 && (path[0][0] == path[last][0] || path[0][1] == path[last][1]) {
			mid := [2]int{(path[0][0] + path[last][0]) / 2, (path[0][1] + path[last][1]) / 2}
			if last == 2 {
				mid = path[1]
			}
			path = [][2]int{path[0], mid, mid, path[last]}
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
				after := bend[across] > b[across+2]
				if b[along+2]-b[along] < 2 || at[along] != bend[along] || at[along] < b[along] || at[along] > b[along+2] ||
					!after && bend[across] >= b[across] {
					continue // not straight into the side from beyond it
				}
				// onto the side, also where a rounder outline curves inside
				path[e.i][across] = b[across]
				if after {
					path[e.i][across] = b[across+2]
				}
				end := end{path: path, i: e.i, j: e.j, fixed: true, toward: at[along]}
				if e.k >= 0 && e.k < len(path) {
					end.fixed = path[e.k][along] == bend[along]
					end.toward = path[e.k][along]
				}
				end.group = c.l.Edges[k].Merged[min(e.i, 1)] // the start, else the end
				p := c.l.Edges[k].Path[0]
				if e.i != 0 {
					p = c.l.Edges[k].Path[len(c.l.Edges[k].Path)-1]
				}
				end.exact = float64((p.X - c.origin.X) / c.cellW)
				if along == 1 {
					end.exact = float64((p.Y - c.origin.Y) / c.cellH)
				}
				key := side{e.node, along, after}
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
	// sides in the order of the nodes, as an edge can end on two of them
	// and moving one end can move the other
	keys := slices.Collect(maps.Keys(sides))
	order := map[*layout.Node]int{}
	for i, node := range c.l.Graph.Nodes {
		order[node] = i
	}
	slices.SortFunc(keys, func(a, b side) int {
		return cmp.Or(cmp.Compare(order[a.node], order[b.node]), cmp.Compare(a.along, b.along), cmp.Compare(boolInt(a.after), boolInt(b.after)))
	})
	for _, key := range keys {
		ends := sides[key]
		lo, hi := c.sideCells(key.node, key.along)
		if len(ends) > hi-lo+1 {
			continue // no room; keep them as they are
		}
		slices.SortStableFunc(ends, func(a, b end) int {
			return cmp.Or(cmp.Compare(a.path[a.i][key.along], b.path[b.i][key.along]), cmp.Compare(a.toward, b.toward))
		})
		want := make([]int, len(ends))
		exact := make([]float64, len(ends))
		fixed := make([]bool, len(ends))
		for n, e := range ends {
			want[n], exact[n], fixed[n] = e.path[e.i][key.along], e.exact, e.fixed
		}
		packEnds(want, exact, fixed)
		// an end that turns toward a place along the side goes straight
		// there instead, past no other end
		for n, e := range ends {
			if e.fixed || e.toward < lo || e.toward > hi {
				continue
			}
			past := slices.ContainsFunc(want, func(w int) bool {
				return w != want[n] && (w-want[n])*(w-e.toward) <= 0
			})
			if !past {
				want[n], fixed[n] = e.toward, true
			}
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
			e.path[e.i][key.along], e.path[e.j][key.along] = at, at
			for _, f := range e.also {
				f.path[f.i][key.along], f.path[f.j][key.along] = at, at
			}
		}
	}
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
