package layout

import (
	"math"
	"math/rand"
	"sort"
)

// Force lays out the graph with a Fruchterman-Reingold force simulation:
// nodes repel each other, edges pull their ends together. It suits
// undirected and cyclic graphs where ranks make no sense. Edges are drawn
// as straight segments between node boundaries.
//
// It sets Node.Center and Edge.Path and fails when an edge refers to a
// node that is not part of the graph.
func Force(graph *Graph) error {
	if err := graph.validate(); err != nil {
		return err
	}
	graph.AssignMissingValues()
	if graph.Pinned {
		layoutPinned(graph)
		return nil
	}
	n := len(graph.Nodes)
	if n == 0 {
		return nil
	}

	// ideal edge length: the largest node plus padding
	k := 0.0
	for _, node := range graph.Nodes {
		k = math.Max(k, float64(2*max(node.Radius.X, node.Radius.Y)+graph.NodePadding))
	}
	area := k * k * float64(n)
	side := math.Sqrt(area)

	index := make(map[*Node]int, n)
	pos := make([][2]float64, n)
	rng := rand.New(rand.NewSource(1)) // deterministic
	for i, node := range graph.Nodes {
		index[node] = i
		pos[i] = [2]float64{rng.Float64() * side, rng.Float64() * side}
	}
	// edges between the same nodes pull as one, so that their labels
	// have room; a labeled edge rests longer by its label
	pairs := newPairs(graph)
	rest := make([]float64, len(graph.Edges))
	for i, edge := range graph.Edges {
		rest[i] = k
		if edge.Label != "" {
			rest[i] += float64(2 * max(edge.LabelRadius.X, edge.LabelRadius.Y))
		}
	}

	// float64 conversions below block FMA fusion, so the layout is
	// identical across architectures (goldens are compared byte for byte)

	// O(n²) repulsion; add a grid or Barnes-Hut past a few thousand nodes
	const iterations = 300
	disp := make([][2]float64, n)
	temperature := side / 10
	for range iterations {
		for i := range disp {
			disp[i] = [2]float64{}
		}
		for i := range n {
			for j := i + 1; j < n; j++ {
				dx, dy := pos[i][0]-pos[j][0], pos[i][1]-pos[j][1]
				d := hypot(dx, dy)
				if d < 1e-3 {
					dx, dy, d = 1e-3*float64(i-j), 1e-3, 1e-3
				}
				f := k * k / d / d // repulsion k²/d, normalized by d
				disp[i][0] += float64(dx * f)
				disp[i][1] += float64(dy * f)
				disp[j][0] -= float64(dx * f)
				disp[j][1] -= float64(dy * f)
			}
		}
		for e, edge := range graph.Edges {
			i, j := index[edge.From], index[edge.To]
			if i == j {
				continue
			}
			dx, dy := pos[i][0]-pos[j][0], pos[i][1]-pos[j][1]
			d := hypot(dx, dy)
			if d < 1e-3 {
				continue
			}
			// attraction d²/rest, normalized by d
			f := d * edge.Weight / rest[e] / float64(pairs.count[pairs.key(edge)])
			disp[i][0] -= float64(dx * f)
			disp[i][1] -= float64(dy * f)
			disp[j][0] += float64(dx * f)
			disp[j][1] += float64(dy * f)
		}
		for i := range pos {
			d := hypot(disp[i][0], disp[i][1])
			if d > temperature {
				disp[i][0] *= temperature / d
				disp[i][1] *= temperature / d
			}
			pos[i][0] += disp[i][0]
			pos[i][1] += disp[i][1]
		}
		temperature *= 0.98
	}

	// an outlier stretches the whole picture: shrink every gap between
	// neighbors along an axis to at most twice the median gap
	for axis := range 2 {
		order := make([]int, n)
		for i := range order {
			order[i] = i
		}
		sort.Slice(order, func(a, b int) bool { return pos[order[a]][axis] < pos[order[b]][axis] })
		gaps := make([]float64, 0, n)
		for i := 1; i < n; i++ {
			gaps = append(gaps, pos[order[i]][axis]-pos[order[i-1]][axis])
		}
		if len(gaps) < 2 {
			continue
		}
		sort.Float64s(gaps)
		limit := math.Max(2*gaps[len(gaps)/2], k)
		shift, prev := 0.0, pos[order[0]][axis]
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
		edge.Path, edge.LabelPos = nil, Vector{}
	}
	layoutPinned(graph)
	return nil
}

// hypot is math.Hypot without its internal FMA fusion: the explicit
// float64 conversions round every product, so results match across
// architectures. Fine here, the coordinates are far from overflow.
func hypot(x, y float64) float64 {
	return math.Sqrt(float64(x*x) + float64(y*y))
}
