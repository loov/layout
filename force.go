package layout

import (
	"math"
	"math/rand"
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
				d := math.Hypot(dx, dy)
				if d < 1e-3 {
					dx, dy, d = 1e-3*float64(i-j), 1e-3, 1e-3
				}
				f := k * k / d / d // repulsion k²/d, normalized by d
				disp[i][0] += dx * f
				disp[i][1] += dy * f
				disp[j][0] -= dx * f
				disp[j][1] -= dy * f
			}
		}
		for _, edge := range graph.Edges {
			i, j := index[edge.From], index[edge.To]
			if i == j {
				continue
			}
			dx, dy := pos[i][0]-pos[j][0], pos[i][1]-pos[j][1]
			d := math.Hypot(dx, dy)
			if d < 1e-3 {
				continue
			}
			f := d * edge.Weight / k // attraction d²/k, normalized by d
			disp[i][0] -= dx * f
			disp[i][1] -= dy * f
			disp[j][0] += dx * f
			disp[j][1] += dy * f
		}
		for i := range pos {
			d := math.Hypot(disp[i][0], disp[i][1])
			if d > temperature {
				disp[i][0] *= temperature / d
				disp[i][1] *= temperature / d
			}
			pos[i][0] += disp[i][0]
			pos[i][1] += disp[i][1]
		}
		temperature *= 0.98
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
