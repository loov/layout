package layout

import (
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
		sort.Slice(nodes, func(i, k int) bool { return nodes[i].Center.X < nodes[k].Center.X })
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
func routeAround(path []Vector, obs *obstacles, from, to *lnode, pad Length) []Vector {
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
				if node == from || node == to || !segmentHitsBox(a, b, node, pad-0.01) {
					continue
				}
				// nearest obstacle along the segment first
				if d := absLength(node.Center.X - a.X); d < hitDist {
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

func absLength(v Length) Length {
	if v < 0 {
		return -v
	}
	return v
}

// segmentHitsBox reports whether segment ab intersects node's box grown by pad
func segmentHitsBox(a, b Vector, node *lnode, pad Length) bool {
	return segmentHitsRect(a, b, Vector{node.Left() - pad, node.Top() - pad}, Vector{node.Right() + pad, node.Bottom() + pad})
}

// routeAroundClusters detours segments that cut across a cluster box the
// edge does not belong to, going around the nearest corner.
func routeAroundClusters(path []Vector, clusters []*lcluster, from, to *lnode, pad Length) []Vector {
	for _, cluster := range clusters {
		if slices.Contains(cluster.Nodes, from) || slices.Contains(cluster.Nodes, to) {
			continue
		}
		// virtual nodes outside the box sit pad away from it; grow by
		// less so that chains running alongside are not hits
		pad := pad / 2
		tl, br := cluster.TopLeft.Add(Vector{-pad, -pad}), cluster.BottomRight.Add(Vector{pad, pad})
		corners := []Vector{tl, {br.X, tl.Y}, br, {tl.X, br.Y}}
		inserted := 0
		for i := 0; i+1 < len(path) && inserted < 8; i++ {
			a, b := path[i], path[i+1]
			if !segmentHitsRect(a, b, tl, br) {
				continue
			}
			// nearest corner to the segment's midpoint
			mid := Vector{(a.X + b.X) / 2, (a.Y + b.Y) / 2}
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

// segmentHitsRect reports whether segment ab intersects the rectangle tl-br
func segmentHitsRect(a, b Vector, tl, br Vector) bool {
	x0, y0 := float64(tl.X), float64(tl.Y)
	x1, y1 := float64(br.X), float64(br.Y)
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
