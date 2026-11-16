package layout

import (
	"cmp"
	"math"
	"slices"
	"sort"
)

// obstacles indexes the real nodes per rank, sorted by x, with the
// largest padded radius per rank for quick rejection.
type obstacles struct {
	byRank               [][]*lnode
	rowRadius, colRadius []Length
}

func newObstacles(byRank [][]*lnode, pad Length) *obstacles {
	obs := &obstacles{byRank: byRank, rowRadius: make([]Length, len(byRank)), colRadius: make([]Length, len(byRank))}
	for r, nodes := range byRank {
		slices.SortFunc(nodes, func(a, b *lnode) int { return cmp.Compare(a.Center.X, b.Center.X) })
		for _, node := range nodes {
			obs.rowRadius[r] = max(obs.rowRadius[r], node.Radius.Y+pad)
			obs.colRadius[r] = max(obs.colRadius[r], node.Radius.X+pad)
		}
	}

	return obs
}

// routeAround inserts waypoints so that no segment of path passes through a
// node. Segment i connects rank firstRank+i to firstRank+i+1; only nodes on
// those two ranks can be hit. Obstacles on the lower rank are passed above,
// on the upper rank below.
func routeAround(path Path, obs *obstacles, from, to *lnode, pad Length) Path {
	var lastHit *lnode
	inserted := 0
	for i := 0; i+1 < len(path) && inserted < 16; i++ {
		a, b := path[i], path[i+1]
		var hit *lnode
		hitDist := Length(math.Inf(1))
		// every rank whose row the segment's y span touches
		top, bottom := min(a.Y, b.Y), max(a.Y, b.Y)
		left, right := min(a.X, b.X), max(a.X, b.X)
		for rank, nodes := range obs.byRank {
			if len(nodes) == 0 {
				continue
			}
			if row := nodes[0].Center; row.Y+obs.rowRadius[rank] < top || row.Y-obs.rowRadius[rank] > bottom {
				continue
			}

			// nodes are sorted by x; only those whose box can reach the
			// segment's x span
			reach := obs.colRadius[rank]
			first := sort.Search(len(nodes), func(k int) bool { return nodes[k].Center.X >= left-reach })
			for _, node := range nodes[first:] {
				if node.Center.X > right+reach {
					break
				}

				// slightly less than pad: waypoints sit on the padded box
				// and touching it is not a hit
				if node == from || node == to || !node.rect().grow(pad-0.01).hitBy(a, b) {
					continue
				}

				// nearest obstacle along the segment first
				if d := (node.Center.X - a.X).Abs(); d < hitDist {
					hit, hitDist = node, d
				}
			}
		}

		if hit == nil {
			lastHit = nil
			continue
		}
		if hit == lastHit {
			// the detour did not clear it (the segment starts beside the
			// node); give up on this obstacle rather than loop
			lastHit = nil
			continue
		}
		lastHit = hit
		way := Vector{X: hit.Center.X, Y: hit.Bottom() + pad}
		if hit.Center.Y > (a.Y+b.Y)/2 {
			way.Y = hit.Top() - pad
		}

		// a segment that starts or ends beside the node goes around the
		// corner on that end's side
		beside := func(p Vector) bool { return p.Y > hit.Top() && p.Y < hit.Bottom() }
		for _, p := range []Vector{a, b} {
			if beside(p) {
				way.X = hit.Right() + pad
				if p.X < hit.Center.X {
					way.X = hit.Left() - pad
				}
			}
		}

		path = slices.Insert(path, i+1, way)
		inserted++
		i-- // re-check segment a→way, which may hit something else
	}

	return path
}

// routeAroundClusters detours segments that cut across a cluster box the
// edge does not belong to, going around the nearest corner.
func routeAroundClusters(path Path, clusters []*lcluster, from, to *lnode, pad Length) Path {
	for _, cluster := range clusters {
		if slices.Contains(cluster.Nodes, from) || slices.Contains(cluster.Nodes, to) {
			continue
		}

		// virtual nodes outside the box sit pad away from it; grow by
		// less so that chains running alongside are not hits
		pad := pad / 2
		box := cluster.rect().grow(pad)
		corners := box.corners()
		inserted := 0
		for i := 0; i+1 < len(path) && inserted < 8; i++ {
			a, b := path[i], path[i+1]
			if !box.hitBy(a, b) {
				continue
			}

			// nearest corner to the segment's midpoint
			mid := midpoint(a, b)
			best, bestDist := corners[0], Length(math.Inf(1))
			for _, c := range corners {
				if d := (c.X-mid.X)*(c.X-mid.X) + (c.Y-mid.Y)*(c.Y-mid.Y); d < bestDist {
					best, bestDist = c, d
				}
			}
			if best == a || best == b {
				continue // already routed via this corner
			}
			path = slices.Insert(path, i+1, best)
			inserted++
			i-- // re-check a→corner
		}
	}

	return path
}

// hitBy reports whether segment ab intersects r
func (r rect) hitBy(a, b Vector) bool {
	x0, y0 := float64(r.Min.X), float64(r.Min.Y)
	x1, y1 := float64(r.Max.X), float64(r.Max.Y)
	dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
	t0, t1 := 0.0, 1.0
	clip := func(p, q float64) bool {
		if p == 0 {
			return q >= 0
		}
		r := q / p
		if p < 0 {
			if r > t1 {
				return false
			}
			t0 = math.Max(t0, r)
		} else {
			if r < t0 {
				return false
			}
			t1 = math.Min(t1, r)
		}
		return true
	}

	ax, ay := float64(a.X), float64(a.Y)
	return clip(-dx, ax-x0) && clip(dx, x1-ax) && clip(-dy, ay-y0) && clip(dy, y1-ay) && t0 <= t1
}

// clearCorners moves the corners whose rounding, of radius, cuts within
// pad of a node out by as much as the curve cuts in, so that the curve runs
// where the corner was; a corner stays when moving it would bring a segment
// within pad of a node or cross more edges. Each corner scans all nodes and
// segments by their bounds; a spatial index would help on huge graphs.
func clearCorners(edges []*ledge, nodes []*lnode, radius, pad Length) {
	hits := func(edge *ledge, a, b Vector, pad Length) bool {
		for _, node := range nodes {
			if max(a.X, b.X) < node.Left()-pad || min(a.X, b.X) > node.Right()+pad ||
				max(a.Y, b.Y) < node.Top()-pad || min(a.Y, b.Y) > node.Bottom()+pad {
				continue
			}
			if !node.Invisible && node != edge.From && node != edge.To && segmentHitsNode(a, b, node, pad) {
				return true
			}
		}
		return false
	}
	// crossings counts the other edges' segments that a→b→c crosses,
	// skipping those outside its bounds
	crossings := func(edge *ledge, a, b, c Vector) (n int) {
		lo := Vector{min(a.X, b.X, c.X), min(a.Y, b.Y, c.Y)}
		hi := Vector{max(a.X, b.X, c.X), max(a.Y, b.Y, c.Y)}
		for _, other := range edges {
			if other == edge {
				continue
			}
			for q, r := range other.Path.Segments() {
				if max(q.X, r.X) < lo.X || min(q.X, r.X) > hi.X || max(q.Y, r.Y) < lo.Y || min(q.Y, r.Y) > hi.Y {
					continue
				}
				for _, s := range [...]segment{{a: a, b: b}, {a: b, b: c}} {
					if crosses, _ := cross(s, segment{a: q, b: r}); crosses {
						n++
					}
				}
			}
		}
		return n
	}
	for _, edge := range edges {
		path := edge.Path
		for i := 1; i+1 < len(path); i++ {
			for range 3 {
				flat := flattenPath(path[i-1:i+2], radius, pad)
				in, mid, exit := flat[1], flat[2], flat[3]
				if !hits(edge, in, mid, pad) && !hits(edge, mid, exit, pad) {
					break
				}
				out := path[i].Sub(mid)
				d := out.Len()
				if d < diagEps {
					break
				}
				p := path[i].Add(out.Scale((d + 0.5) / d))
				if hits(edge, path[i-1], p, pad-0.01) || hits(edge, p, path[i+1], pad-0.01) ||
					crossings(edge, path[i-1], p, path[i+1]) > crossings(edge, path[i-1], path[i], path[i+1]) {
					break
				}
				path[i] = p
			}
		}
	}
}
