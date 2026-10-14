package layout

import (
	"math"

	"github.com/loov/layout/internal/hier"
)

// constraint is d[b] - d[a] >= min; elastic marks the run of an edge
// between a and b, whose length shortenRuns shortens
type constraint struct {
	a, b    int
	min     int
	elastic bool
}

// leastSolution returns the least d with d[v] >= lower[v] that satisfies
// cons, by longest paths: every variable starts at its bound and is
// raised along the constraints until none is violated. The least
// solution exists and the queue drains as long as some solution does,
// which d = 0 is when every bound and minimum is at most 0, as the
// constraints compactAxis makes are.
func leastSolution(n int, cons []constraint, lower []int) []int {
	out := make([][]int, n)
	for i, c := range cons {
		out[c.a] = append(out[c.a], i)
	}
	d := append([]int(nil), lower...)
	queued := make([]bool, n)
	queue := make([]int, 0, n)
	for v := range n {
		queue = append(queue, v)
		queued[v] = true
	}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		queued[v] = false
		for _, i := range out[v] {
			c := cons[i]
			if d[c.b] < d[v]+c.min {
				d[c.b] = d[v] + c.min
				if !queued[c.b] {
					queued[c.b] = true
					queue = append(queue, c.b)
				}
			}
		}
	}
	return d
}

// shortenRuns shortens the elastic runs of edges as much as it can
// without widening the drawing past the least solution d, by network
// simplex: two more variables bound the drawing, where every variable
// reaches from lower[v] to reach[v] past where it moves to.
func shortenRuns(n int, cons []constraint, lower, reach, d []int) []int {
	left, right := int32(n), int32(n+1)
	x := make([]int32, n+2)
	width := math.MinInt32
	for v := range n {
		x[v] = int32(d[v])
		width = max(width, d[v]+reach[v])
	}
	x[right] = int32(width)
	var tail, head, minlen []int32
	var weight []float32
	add := func(a, b int32, min int, w float32) {
		tail, head, minlen, weight = append(tail, a), append(head, b), append(minlen, int32(min)), append(weight, w)
	}
	for _, c := range cons {
		w := float32(0)
		if c.elastic {
			w = 1
		}
		add(int32(c.a), int32(c.b), c.min, w)
	}
	for v := range n {
		add(left, int32(v), lower[v], 0)
		add(int32(v), right, reach[v], 0)
	}
	add(right, left, -width, 0)
	hier.NetworkSimplex(x, tail, head, weight, minlen)
	out := make([]int, n)
	for v := range out {
		out[v] = int(x[v] - x[left])
	}
	return out
}

// transposeLayout swaps x and y of everything in l
func transposeLayout(l *Layout) {
	swap := func(v *Vector) { v.X, v.Y = v.Y, v.X }
	for i := range l.Nodes {
		swap(&l.Nodes[i].Center)
		swap(&l.Nodes[i].Size)
	}
	for i := range l.Edges {
		e := &l.Edges[i]
		for j := range e.Path {
			swap(&e.Path[j])
		}
		for _, v := range []*Vector{&e.LabelCenter, &e.LabelSize, &e.HeadLabelCenter, &e.HeadLabelSize, &e.TailLabelCenter, &e.TailLabelSize} {
			swap(v)
		}
	}
	for i := range l.Clusters {
		swap(&l.Clusters[i].TopLeft)
		swap(&l.Clusters[i].BottomRight)
	}
}

// translateLayout moves everything in l by d
func translateLayout(l *Layout, d Vector) {
	for i := range l.Nodes {
		l.Nodes[i].Center = l.Nodes[i].Center.Add(d)
	}
	for i := range l.Edges {
		e := &l.Edges[i]
		for j := range e.Path {
			e.Path[j] = e.Path[j].Add(d)
		}
		for _, v := range []*Vector{&e.LabelCenter, &e.HeadLabelCenter, &e.TailLabelCenter} {
			*v = v.Add(d)
		}
	}
	for i := range l.Clusters {
		l.Clusters[i].TopLeft = l.Clusters[i].TopLeft.Add(d)
		l.Clusters[i].BottomRight = l.Clusters[i].BottomRight.Add(d)
	}
}

// simplifyPath drops repeated points of a path, and points in the middle
// of a straight run, which compaction leaves where it lined runs up
func simplifyPath(path Path) Path {
	out := path[:0]
	for i, p := range path {
		if i > 0 && p == out[len(out)-1] && i < len(path)-1 {
			continue
		}
		if n := len(out); n >= 2 && i < len(path) {
			a, b := out[n-2], out[n-1]
			if (a.X == b.X && b.X == p.X) || (a.Y == b.Y && b.Y == p.Y) {
				out[n-1] = p
				continue
			}
		}
		out = append(out, p)
	}
	return out
}
