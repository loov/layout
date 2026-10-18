package layout

import (
	"cmp"
	"container/heap"
	"math"
	"math/rand"
	"slices"
	"sort"
)

// Force lays out the graph by stress: every pair of nodes is placed about
// as far apart as the shortest path between them, then nodes too close
// for their size are pushed apart. It suits undirected and cyclic graphs
// where ranks make no sense. Edges are drawn as straight segments between
// node boundaries.
//
// The graph is not modified. It fails when an edge refers to a node that
// is not part of the graph.
func Force(graph *Graph, opts ForceOptions) (*Layout, error) {
	if err := graph.validate(); err != nil {
		return nil, err
	}
	work := newWorkGraph(graph)
	work.ForText = opts.ForText
	force(work)
	work.placeEndLabels()
	return work.result(), nil
}

// ForceOptions tunes the force-directed layout.
type ForceOptions struct {
	// ForText lays the graph out for format/text, see Options.ForText.
	ForText bool
}

// force lays out the working copy of a validated graph.
func force(graph *lgraph) {
	if graph.ForText {
		graph.prepareText()
	}
	graph.assignDefaults()
	if graph.Pinned {
		layoutPinned(graph)
		return
	}
	n := len(graph.Nodes)
	if n == 0 {
		return
	}

	// ideal edge length: the largest node plus padding
	k := 0.0
	for _, node := range graph.Nodes {
		k = math.Max(k, float64(2*max(node.Radius.X, node.Radius.Y)+graph.NodePadding))
	}
	area := k * k * float64(n)
	side := math.Sqrt(area)

	index := make(map[*lnode]int, n)
	pos := make([][2]float64, n)
	rng := rand.New(rand.NewSource(1)) // deterministic
	for i, node := range graph.Nodes {
		index[node] = i
		pos[i] = [2]float64{rng.Float64() * side, rng.Float64() * side}
	}

	// edges between the same nodes pull as one, a labeled edge rests
	// longer by its label so that the label has room
	type link struct {
		to             int
		rest, strength float64
	}
	links := make([][]link, n)
	join := func(i, j int, rest, weight float64) {
		for l := range links[i] {
			if links[i][l].to == j {
				links[i][l].rest = math.Max(links[i][l].rest, rest)
				links[i][l].strength = math.Max(links[i][l].strength, weight)
				return
			}
		}
		links[i] = append(links[i], link{j, rest, weight})
	}
	for _, edge := range graph.Edges {
		i, j := index[edge.From], index[edge.To]
		if i == j {
			continue
		}
		rest := 2 * k // room between the nodes to see the edge
		if edge.Label != "" {
			rest += float64(2 * max(edge.LabelRadius.X, edge.LabelRadius.Y))
		}
		join(i, j, rest, edge.Weight)
		join(j, i, rest, edge.Weight)
	}

	// stress majorization by stochastic gradient descent (Zheng, Pawar and
	// Goodman, "Graph drawing by stochastic gradient descent", 2018): every
	// pair of nodes moves towards its distance along the graph, the nearer
	// pairs more strongly
	type pair struct {
		i, j int32
		d, w float64 // the distance to keep, how strongly
	}
	// O(n²) pairs in memory; sparse stress with pivots past a few
	// thousand nodes
	pairs := make([]pair, 0, n*(n-1)/2)
	dist := make([]float64, n)
	longest, shortest := 0.0, math.Inf(1)
	for i := range n {
		shortestPaths(i, dist, func(i int, visit func(j int, length float64)) {
			for _, l := range links[i] {
				visit(l.to, l.rest)
			}
		})
		for j := i + 1; j < n; j++ {
			if !math.IsInf(dist[j], 1) {
				longest = math.Max(longest, dist[j])
				shortest = math.Min(shortest, dist[j])
			}
			pairs = append(pairs, pair{i: int32(i), j: int32(j), d: dist[j]})
		}
	}
	if math.IsInf(shortest, 1) {
		shortest, longest = k, k // no edges
	}
	strength := func(i, j int) float64 {
		for _, l := range links[i] {
			if l.to == j {
				return l.strength
			}
		}
		return 1
	}
	for p := range pairs {
		pr := &pairs[p]
		if math.IsInf(pr.d, 1) {
			pr.d = longest + k // separate components keep apart
		}
		pr.w = strength(int(pr.i), int(pr.j)) / float64(pr.d*pr.d)
	}

	// the step size falls geometrically from 1/min w to 0.1/max w; over
	// 2⁶ steps, so that six square roots give the rate, which unlike
	// math.Pow rounds the same on every architecture
	const halvings, steps = 6, 1 << 6
	step, end := longest*longest, 0.1*shortest*shortest
	rate := end / step
	for range halvings {
		rate = math.Sqrt(rate)
	}

	for range steps + 1 {
		rng.Shuffle(len(pairs), func(a, b int) { pairs[a], pairs[b] = pairs[b], pairs[a] })
		for _, pr := range pairs {
			i, j := pr.i, pr.j
			dx, dy := pos[i][0]-pos[j][0], pos[i][1]-pos[j][1]
			d := hypot(dx, dy)
			if d < 1e-3 {
				dx, dy = 1e-3*float64(i-j), 1e-3
				d = hypot(dx, dy)
			}
			mu := math.Min(float64(pr.w*step), 1)
			f := float64(float64(d-pr.d)/2*mu) / d
			pos[i][0] -= float64(dx * f)
			pos[i][1] -= float64(dy * f)
			pos[j][0] += float64(dx * f)
			pos[j][1] += float64(dy * f)
		}
		step *= rate
	}

	// stress can't keep every neighbor of a hub an edge apart, so push
	// apart the nodes that end up closer than their sizes allow; O(n²) a
	// round, a sweep line past a few thousand nodes
	reach := make([]float64, n)
	for i, node := range graph.Nodes {
		reach[i] = float64(max(node.Radius.X, node.Radius.Y) + graph.NodePadding/2)
	}
	for range 100 {
		moved := false
		for i := range n {
			for j := i + 1; j < n; j++ {
				dx, dy := pos[i][0]-pos[j][0], pos[i][1]-pos[j][1]
				d, apart := hypot(dx, dy), reach[i]+reach[j]
				if d >= apart-1e-3 {
					continue
				}
				if d < 1e-3 {
					dx, dy = 1e-3*float64(i-j), 1e-3
					d = hypot(dx, dy)
				}
				f := float64(apart-d) / 2 / d
				pos[i][0] += float64(dx * f)
				pos[i][1] += float64(dy * f)
				pos[j][0] -= float64(dx * f)
				pos[j][1] -= float64(dy * f)
				moved = true
			}
		}
		if !moved {
			break
		}
	}

	// an outlier stretches the whole picture: shrink every gap between
	// neighbors along an axis to at most twice the median gap
	for axis := range 2 {
		order := make([]int, n)
		for i := range order {
			order[i] = i
		}
		slices.SortFunc(order, func(a, b int) int { return cmp.Compare(pos[a][axis], pos[b][axis]) })
		gaps := make([]float64, 0, n)
		for i := 1; i < n; i++ {
			gaps = append(gaps, pos[order[i]][axis]-pos[order[i-1]][axis])
		}
		if len(gaps) < 2 {
			continue
		}
		sort.Float64s(gaps)
		limit := math.Max(2*gaps[len(gaps)/2], k)
		shift := 0.0
		prev := pos[order[0]][axis]
		for i := 1; i < n; i++ {
			gap := pos[order[i]][axis] - prev
			prev = pos[order[i]][axis]
			if gap > limit {
				shift += gap - limit
			}
			pos[order[i]][axis] -= shift
		}
	}

	// shift so that the drawing starts at the padding
	minX, minY := math.Inf(1), math.Inf(1)
	for i, node := range graph.Nodes {
		minX = math.Min(minX, pos[i][0]-float64(node.Radius.X))
		minY = math.Min(minY, pos[i][1]-float64(node.Radius.Y))
	}
	for i, node := range graph.Nodes {
		node.Center = Vector{
			X: Length(pos[i][0]-minX) + graph.NodePadding,
			Y: Length(pos[i][1]-minY) + graph.RowPadding,
		}
	}

	for _, edge := range graph.Edges {
		edge.Path = nil
		edge.LabelPos = Vector{}
	}
	layoutPinned(graph)
}

// shortestPaths sets dist to the length of the shortest path from source
// to every node, +Inf where there is none, with Dijkstra's algorithm;
// neighbors visits the nodes next to a node, with the length to them.
func shortestPaths(source int, dist []float64, neighbors func(i int, visit func(j int, length float64))) {
	for i := range dist {
		dist[i] = math.Inf(1)
	}
	dist[source] = 0
	queue := &nearest{{source, 0}}
	for queue.Len() > 0 {
		at := heap.Pop(queue).(reached)
		if at.dist > dist[at.node] {
			continue // reached sooner since
		}
		neighbors(at.node, func(j int, length float64) {
			if d := at.dist + length; d < dist[j] {
				dist[j] = d
				heap.Push(queue, reached{j, d})
			}
		})
	}
}

// reached is a node and the length of a path to it
type reached struct {
	node int
	dist float64
}

// nearest is a heap of reached nodes, the nearest first
type nearest []reached

func (q nearest) Len() int           { return len(q) }
func (q nearest) Less(a, b int) bool { return q[a].dist < q[b].dist }
func (q nearest) Swap(a, b int)      { q[a], q[b] = q[b], q[a] }
func (q *nearest) Push(x any)        { *q = append(*q, x.(reached)) }
func (q *nearest) Pop() any {
	old := *q
	x := old[len(old)-1]
	*q = old[:len(old)-1]
	return x
}

// hypot is math.Hypot without its internal FMA fusion: the explicit
// float64 conversions round every product, so results match across
// architectures. Fine here, the coordinates are far from overflow.
func hypot(x, y float64) float64 {
	return math.Sqrt(float64(x*x) + float64(y*y))
}
