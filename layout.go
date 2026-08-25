package layout

import (
	"math"
	"slices"

	"github.com/loov/layout/internal/hier"
)

const epsilon = 1e-6

// AssignMissingValues fills in unset padding, font and size values on the
// graph, its nodes and edges from the graph defaults. Node sizes are
// estimated from their labels.
func (graph *Graph) AssignMissingValues() {
	if graph.FontSize <= 0 {
		graph.FontSize = graph.LineHeight * 14 / 16
	}
	if graph.NodePadding <= 0 {
		graph.NodePadding = graph.LineHeight
	}
	if graph.RowPadding <= 0 {
		graph.RowPadding = graph.LineHeight
	}
	if graph.EdgePadding <= 0 {
		graph.EdgePadding = 6 * Point
	}

	for _, node := range graph.Nodes {
		if node.Shape == "" {
			node.Shape = graph.Shape
		}

		if node.Weight < epsilon {
			node.Weight = epsilon
		}

		if node.FontSize <= 0 {
			node.FontSize = graph.FontSize
		}

		if node.Radius.X <= 0 || node.Radius.Y <= 0 {
			node.Radius.X = graph.LineHeight
			node.Radius.Y = graph.LineHeight

			labelRadius := node.approxLabelRadius(graph.LineHeight)
			labelRadius.X += node.FontSize * 0.5
			labelRadius.Y += node.FontSize * 0.25

			if node.Radius.X < labelRadius.X {
				node.Radius.X = labelRadius.X
			}
			if node.Radius.Y < labelRadius.Y {
				node.Radius.Y = labelRadius.Y
			}
		}
	}

	for _, edge := range graph.Edges {
		if edge.Weight < epsilon {
			edge.Weight = epsilon
		}
	}
}

// Hierarchical lays out the graph top-down in ranks (Sugiyama style):
// cycles are broken, nodes are assigned to ranks, ordered within ranks to
// reduce crossings, positioned, and finally edge paths are computed.
//
// It sets Node.Center and Edge.Path.
func Hierarchical(graphdef *Graph) {
	graphdef.AssignMissingValues()

	nodes := map[*Node]hier.ID{}
	reverse := map[hier.ID]*Node{}

	// nodes with self-loops need room on their right for the loop
	loopWidth := 2 * graphdef.NodePadding
	hasLoop := map[*Node]bool{}
	for _, edge := range graphdef.Edges {
		if edge.From == edge.To {
			hasLoop[edge.From] = true
		}
	}

	// construct hierarchical graph
	graph := &hier.Graph{}
	for _, nodedef := range graphdef.Nodes {
		node := graph.AddNode()
		nodes[nodedef] = node.ID
		reverse[node.ID] = nodedef
		node.Label = nodedef.ID
	}
	for _, edge := range graphdef.Edges {
		from, to := nodes[edge.From], nodes[edge.To]
		graph.AddWeightedEdge(graph.Nodes[from], graph.Nodes[to], float32(edge.Weight))
	}

	for _, group := range graphdef.SameRank {
		var members hier.Nodes
		for _, nodedef := range group {
			members = append(members, graph.Nodes[nodes[nodedef]])
		}
		graph.SameRank = append(graph.SameRank, members)
	}

	// remove cycles
	decycledGraph := hier.DefaultDecycle(graph)

	// assign nodes to ranks
	rankedGraph := hier.DefaultRank(decycledGraph)

	// create virtual nodes
	filledGraph := hier.DefaultAddVirtuals(rankedGraph)

	// order nodes in ranks
	orderedGraph := hier.DefaultOrderRanks(filledGraph)

	// assign node sizes
	for id, node := range orderedGraph.Nodes {
		if node.Virtual {
			node.Radius.X = float32(graphdef.EdgePadding)
			node.Radius.Y = float32(graphdef.EdgePadding)
			continue
		}

		nodedef, ok := reverse[hier.ID(id)]
		if !ok {
			// TODO: handle missing node
			continue
		}
		node.Radius.X = float32(nodedef.Radius.X + graphdef.NodePadding)
		if hasLoop[nodedef] {
			node.Radius.X += float32(loopWidth / 2)
		}
		node.Radius.Y = float32(nodedef.Radius.Y + graphdef.RowPadding)
	}

	// position nodes
	positionedGraph := hier.DefaultPosition(orderedGraph)

	// assign final positions; loop nodes were widened symmetrically,
	// shift them left so the extra room is on the right
	for nodedef, id := range nodes {
		node := positionedGraph.Nodes[id]
		nodedef.Center.X = Length(node.Center.X)
		nodedef.Center.Y = Length(node.Center.Y)
		if hasLoop[nodedef] {
			nodedef.Center.X -= loopWidth / 2
		}
	}

	// real nodes per rank, obstacles for edge routing
	byRank := make([][]*Node, len(positionedGraph.ByRank))
	for _, node := range positionedGraph.Nodes {
		if !node.Virtual {
			byRank[node.Rank] = append(byRank[node.Rank], reverse[node.ID])
		}
	}

	// calculate edges
	edgePaths := map[[2]hier.ID][]Vector{}
	for _, source := range positionedGraph.Nodes {
		if source.Virtual {
			continue
		}

		sourcedef := reverse[source.ID]
		for _, out := range source.Out {
			path := []Vector{}
			path = append(path, sourcedef.BottomCenter())

			target := out
			for target != nil && target.Virtual {
				if len(target.Out) < 1 { // should never happen
					target = nil
					break
				}

				path = append(path, Vector{
					Length(target.Center.X),
					Length(target.Center.Y),
				})

				target = target.Out[0]
			}
			if target == nil {
				continue
			}

			targetdef := reverse[target.ID]
			path = append(path, targetdef.TopCenter())

			// clip ends to node outlines so fan-ins don't converge on one point
			path[0] = sourcedef.Boundary(path[1])
			path[len(path)-1] = targetdef.Boundary(path[len(path)-2])

			path = routeAround(path, byRank, source.Rank, sourcedef, targetdef, graphdef.EdgePadding)

			edgePaths[[2]hier.ID{source.ID, target.ID}] = path
		}
	}

	// flat edges run sideways along the rank, arcing over nodes in between
	for _, flat := range positionedGraph.Flat {
		sourcedef, targetdef := reverse[flat[0].ID], reverse[flat[1].ID]
		path := []Vector{sourcedef.Boundary(targetdef.Center), targetdef.Boundary(sourcedef.Center)}
		if flat[1].Pos-flat[0].Pos > 1 {
			top := min(sourcedef.Top(), targetdef.Top())
			for _, node := range byRank[flat[0].Rank] {
				if node.Center.X > sourcedef.Center.X && node.Center.X < targetdef.Center.X {
					top = min(top, node.Top())
				}
			}
			y := top - 2*graphdef.EdgePadding
			path = []Vector{
				sourcedef.TopCenter(),
				{sourcedef.Center.X, y},
				{targetdef.Center.X, y},
				targetdef.TopCenter(),
			}
		}
		edgePaths[[2]hier.ID{flat[0].ID, flat[1].ID}] = path
	}

	for _, edge := range graphdef.Edges {
		sourceid := nodes[edge.From]
		targetid := nodes[edge.To]

		if sourceid == targetid {
			edge.Path = loopPath(edge.From, loopWidth)
			continue
		}

		path, ok := edgePaths[[2]hier.ID{sourceid, targetid}]
		if ok {
			edge.Path = path
			continue
		}

		// some paths may have been reversed
		revpath, ok := edgePaths[[2]hier.ID{targetid, sourceid}]
		if ok {
			edge.Path = reversePath(revpath)
			continue
		}
	}
}

// reversePath returns the path in reverse order
func reversePath(path []Vector) []Vector {
	rs := make([]Vector, 0, len(path))
	for _, p := range slices.Backward(path) {
		rs = append(rs, p)
	}
	return rs
}

// loopPath draws a self-loop on the right side of the node
func loopPath(node *Node, width Length) []Vector {
	right := node.Right() + width
	up := Vector{X: node.Right(), Y: node.Center.Y - node.Radius.Y/2}
	down := Vector{X: node.Right(), Y: node.Center.Y + node.Radius.Y/2}
	return []Vector{
		node.Boundary(up),
		{X: right, Y: up.Y},
		{X: right, Y: down.Y},
		node.Boundary(down),
	}
}

// routeAround inserts waypoints so that no segment of path passes through a
// node. Segment i connects rank firstRank+i to firstRank+i+1; only nodes on
// those two ranks can be hit. Obstacles on the lower rank are passed above,
// on the upper rank below.
func routeAround(path []Vector, byRank [][]*Node, firstRank int, from, to *Node, pad Length) []Vector {
	for i := 0; i+1 < len(path); i++ {
		a, b := path[i], path[i+1]
		var hit *Node
		hitDist := Length(math.Inf(1))
		for rank := firstRank + i; rank <= firstRank+i+1 && rank < len(byRank); rank++ {
			for _, node := range byRank[rank] {
				if node == from || node == to || !segmentHitsBox(a, b, node, pad) {
					continue
				}
				// nearest obstacle along the segment first
				if d := absLength(node.Center.X - a.X); d < hitDist {
					hit, hitDist = node, d
				}
			}
		}
		if hit == nil {
			continue
		}
		way := Vector{X: hit.Center.X, Y: hit.Bottom() + pad}
		if hit.Center.Y > (a.Y+b.Y)/2 {
			way.Y = hit.Top() - pad
		}
		path = slices.Insert(path, i+1, way)
		// the new waypoint may itself need routing, so segment i is re-checked;
		// cap the number of detours per edge
		if len(path) > 64 {
			break
		}
		i-- // re-check segment a→way
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
func segmentHitsBox(a, b Vector, node *Node, pad Length) bool {
	x0, y0 := float64(node.Left()-pad), float64(node.Top()-pad)
	x1, y1 := float64(node.Right()+pad), float64(node.Bottom()+pad)
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
