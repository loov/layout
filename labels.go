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
	paths := make([]Path, len(edges), len(edges)+len(clusters))

	// all the flattened paths in one block
	n := 0
	for _, edge := range edges {
		n += flatLen(edge.Path)
	}
	points := make(Path, 0, n)
	for i, edge := range edges {
		start := len(points)
		flat := appendFlatPath(points, edge.Path, radius, pad)
		if len(edge.Path) < 3 {
			paths[i] = flat
			continue
		}
		points = flat
		paths[i] = points[start:len(points):len(points)]
	}

	// the sides of cluster boxes are lines like edges, after the edge paths
	for _, cluster := range clusters {
		box := cluster.rect()
		if box.empty() {
			continue // nothing inside
		}
		corners := box.corners()
		paths = append(paths, append(corners, corners[0]))
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
		box := rect{at.Add(Vector{-edge.LabelRadius.X - pad + 0.01, -edge.LabelRadius.Y}), at.Add(Vector{edge.LabelRadius.X + pad - 0.01, edge.LabelRadius.Y})}
		tl, br := box.Min, box.Max
		for _, path := range paths {
			if lines && box.hitByPath(path) {
				return false
			}
		}

		// nodes only need to stay clear of the text itself
		for _, node := range nodes {
			if node.Left() < br.X-pad && tl.X+pad < node.Right() && node.Top() < br.Y && tl.Y < node.Bottom() {
				return false
			}
		}
		for _, other := range placed {
			if other.labelRect().overlaps(box, 0) {
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
		if p, ok := own.nearest(edge.LabelPos); ok {
			d := edge.LabelPos.Sub(p)
			if !d.IsZero() {
				u := d.Unit()
				dist := pad + u.X.Abs()*edge.LabelRadius.X + u.Y.Abs()*edge.LabelRadius.Y
				// this side, or the other side of the line when only that
				// one is free (parallel edges of a pair)
				same := p.Add(u.Scale(dist))
				other := p.Add(u.Scale(-dist))
				edge.LabelPos = same
				if !clear(edge, same) && clear(edge, other) {
					edge.LabelPos = other
				}
			}
		}
		near := func(at Vector) bool {
			best := centeredRect(at, edge.LabelRadius).pathDistance(own)
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
			base, _ := own.nearest(edge.LabelPos)
			for a, b := range own.Segments() {
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
					d := float64(p.Sub(base).Len())
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
				if at := edge.LabelPos.Add(dir.Scale(d)); near(at) && clear(edge, at) {
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

// placeEndLabels places the head and tail labels beside the ends of their
// edges, just past the nodes: on the first side, stepping further along
// the edge up to its middle, where a label covers no node, edge or label
// placed before it, or else where it covers no node or label, crossing
// lines, or else right past the end. Layouts keep no room for them, so in
// tight ones they can cover what is there.
func (g *lgraph) placeEndLabels() {
	pad := g.EdgePadding
	var placed []rect
	for _, edge := range g.Edges {
		if edge.Label != "" {
			placed = append(placed, edge.labelRect())
		}
	}

	covers := func(r rect) bool {
		for _, node := range g.Nodes {
			if node.rect().overlaps(r, 0) {
				return true
			}
		}
		for _, b := range placed {
			if b.overlaps(r, 0) {
				return true
			}
		}
		return false
	}

	crosses := func(r rect) bool {
		for _, edge := range g.Edges {
			if r.hitByPath(edge.Path) {
				return true
			}
		}
		return false
	}

	for _, edge := range g.Edges {
		n := len(edge.Path)
		if n < 2 || edge.Invisible {
			continue
		}
		for _, end := range []struct {
			label    *endLabel
			text     string
			at, next Vector
		}{
			{&edge.head, edge.HeadLabel, edge.Path[n-1], edge.Path[n-2]},
			{&edge.tail, edge.TailLabel, edge.Path[0], edge.Path[1]},
		} {
			if end.text == "" {
				continue
			}
			r := end.label.radius

			// along the edge away from the node, and across it
			ux, uy := 0.0, 1.0
			if d := end.next.Sub(end.at); !d.IsZero() {
				u := d.Unit()
				ux, uy = float64(u.X), float64(u.Y)
			}
			along := Length(math.Abs(ux))*r.X + Length(math.Abs(uy))*r.Y
			across := Length(math.Abs(uy))*r.X + Length(math.Abs(ux))*r.Y

			// no further than the middle of the edge, so that the label
			// stays nearer its own end
			length := edge.Path.Length()
			var spots []Vector
			for step := range 6 {
				a := pad + along + Length(step)*pad
				if step > 0 && a > length/2 {
					break
				}
				a = min(a, length/3) // a short edge has its label over it
				for _, side := range []Length{1, -1} {
					c := side * (pad/2 + across)
					spots = append(spots, end.at.Add(Vector{Length(ux)*a - Length(uy)*c, Length(uy)*a + Length(ux)*c}))
				}
			}

			at, found := spots[0], false
			for _, lines := range []bool{true, false} {
				for _, spot := range spots {
					box := centeredRect(spot, r)
					if !covers(box) && (!lines || !crosses(box)) {
						at, found = spot, true
						break
					}
				}
				if found {
					break
				}
			}

			end.label.pos = at
			placed = append(placed, centeredRect(at, r))
		}
	}
}
