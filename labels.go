package layout

import (
	"cmp"
	"math"
	"slices"
)

// nudgeLabels slides edge labels sideways along their rank until they
// clear every edge path, the cluster boxes and the labels placed before
// them. Labels of the edges in keep stay where they are, and the others
// avoid them.
func nudgeLabels(edges []*ledge, nodes []*lnode, clusters []*lcluster, pad, radius Length, keep map[*ledge]bool) {
	paths := make([][]Vector, len(edges), len(edges)+len(clusters))
	for i, edge := range edges {
		paths[i] = flattenPath(edge.Path, radius, pad)
	}
	// the sides of cluster boxes are lines like edges, after the edge paths
	for _, cluster := range clusters {
		tl, br := cluster.TopLeft, cluster.BottomRight
		if tl.X > br.X {
			continue // nothing inside
		}
		paths = append(paths, []Vector{tl, {br.X, tl.Y}, br, {tl.X, br.Y}, tl})
	}
	var placed []*ledge
	for _, edge := range edges {
		if keep[edge] && edge.Label != "" {
			placed = append(placed, edge)
		}
	}
	// clearOf reports whether a label of edge at at stays clear of nodes,
	// placed labels and, with lines, every edge path
	clearOf := func(edge *ledge, at Vector, lines bool) bool {
		// a hair inside the padding, so that a line at exactly pad
		// distance (the label's own edge) does not count as a hit
		tl := at.Add(Vector{-edge.LabelRadius.X - pad + 0.01, -edge.LabelRadius.Y})
		br := at.Add(Vector{edge.LabelRadius.X + pad - 0.01, edge.LabelRadius.Y})
		for _, path := range paths {
			if !lines {
				break
			}
			for i := 0; i+1 < len(path); i++ {
				if segmentHitsRect(path[i], path[i+1], tl, br) {
					return false
				}
			}
		}
		// nodes only need to stay clear of the text itself
		for _, node := range nodes {
			if node.Left() < br.X-pad && tl.X+pad < node.Right() && node.Top() < br.Y && tl.Y < node.Bottom() {
				return false
			}
		}
		for _, other := range placed {
			if other.LabelPos.X-other.LabelRadius.X < br.X && tl.X < other.LabelPos.X+other.LabelRadius.X &&
				other.LabelPos.Y-other.LabelRadius.Y < br.Y && tl.Y < other.LabelPos.Y+other.LabelRadius.Y {
				return false
			}
		}
		return true
	}
	clear := func(edge *ledge, at Vector) bool { return clearOf(edge, at, true) }
	for _, edge := range edges {
		if edge.Label == "" || len(edge.Path) < 2 || keep[edge] {
			continue
		}
		// candidates in growing rings: along the rank first, then across
		// it, then diagonally; never further from the edge than the text
		// height, so the label stays attributable
		own := paths[slices.Index(edges, edge)]
		// snap to the drawn path first: the label was placed against a
		// waypoint, but rounding and multi-edge offsets move the line
		if p, ok := nearestOnPath(own, edge.LabelPos); ok {
			d := edge.LabelPos.Sub(p)
			if l := math.Hypot(float64(d.X), float64(d.Y)); l > 0 {
				ux, uy := float64(d.X)/l, float64(d.Y)/l
				support := math.Abs(ux)*float64(edge.LabelRadius.X) + math.Abs(uy)*float64(edge.LabelRadius.Y)
				dist := float64(pad) + support
				// this side, or the other side of the line when only that
				// one is free (parallel edges of a pair)
				same := p.Add(Vector{Length(ux * dist), Length(uy * dist)})
				other := p.Add(Vector{Length(-ux * dist), Length(-uy * dist)})
				edge.LabelPos = same
				if !clear(edge, same) && clear(edge, other) {
					edge.LabelPos = other
				}
			}
		}
		near := func(at Vector) bool {
			tl, br := at.Sub(edge.LabelRadius), at.Add(edge.LabelRadius)
			best := math.Inf(1)
			for i := 0; i+1 < len(own); i++ {
				best = math.Min(best, rectSegmentDistance(tl, br, own[i], own[i+1]))
			}
			// the text height is the smaller extent (radii are swapped
			// for sideways layouts), minus a margin
			return best <= float64(2*min(edge.LabelRadius.X, edge.LabelRadius.Y)-pad)
		}
		// slide along the own path, nearest spot first, trying both sides
		found := false
		type spot struct {
			at   Vector
			dist float64
		}
		var spots []spot
		if !clear(edge, edge.LabelPos) {
			base, _ := nearestOnPath(own, edge.LabelPos)
			for i := 0; i+1 < len(own) && !found; i++ {
				a, b := own[i], own[i+1]
				dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
				l := math.Hypot(dx, dy)
				if l == 0 {
					continue
				}
				nx, ny := -dy/l, dx/l
				support := math.Abs(nx)*float64(edge.LabelRadius.X) + math.Abs(ny)*float64(edge.LabelRadius.Y)
				off := float64(pad) + support
				for t := 0.0; t <= l; t += float64(pad) {
					p := Vector{a.X + Length(t*dx/l), a.Y + Length(t*dy/l)}
					d := math.Hypot(float64(p.X-base.X), float64(p.Y-base.Y))
					spots = append(spots,
						spot{p.Add(Vector{Length(nx * off), Length(ny * off)}), d},
						spot{p.Add(Vector{Length(-nx * off), Length(-ny * off)}), d})
				}
			}
			slices.SortStableFunc(spots, func(a, b spot) int { return cmp.Compare(a.dist, b.dist) })
			for _, s := range spots {
				if clear(edge, s.at) {
					edge.LabelPos, found = s.at, true
					break
				}
			}
		}
		// otherwise rings: along the rank first, then across it, then
		// diagonally; never further from the edge than the text height
		for ring := 1; ring <= 4 && !found; ring++ {
			d := Length(ring) * 2 * pad
			for _, dir := range []Vector{{-1, 0}, {1, 0}, {0, 1}, {0, -1}, {-1, 1}, {1, 1}, {-1, -1}, {1, -1}} {
				if at := edge.LabelPos.Add(Vector{dir.X * d, dir.Y * d}); near(at) && clear(edge, at) {
					edge.LabelPos, found = at, true
					break
				}
			}
		}
		// nowhere clear: crossing another edge reads better than covering
		// another label or a node
		if !found && !clearOf(edge, edge.LabelPos, false) {
			for _, s := range spots {
				if clearOf(edge, s.at, false) {
					edge.LabelPos = s.at
					break
				}
			}
		}
		placed = append(placed, edge)
	}
}
