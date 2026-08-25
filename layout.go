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
		if edge.FontSize <= 0 {
			edge.FontSize = graph.FontSize
		}
		if edge.Label != "" {
			edge.LabelRadius = approxTextRadius(edge.Label, edge.FontSize, graph.LineHeight)
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

	// lay out top to bottom in a transposed/flipped frame, then map back
	sideways := graphdef.RankDir == LeftToRight || graphdef.RankDir == RightToLeft
	swapRadii := func() {
		for _, node := range graphdef.Nodes {
			node.Radius.X, node.Radius.Y = node.Radius.Y, node.Radius.X
		}
		for _, edge := range graphdef.Edges {
			edge.LabelRadius.X, edge.LabelRadius.Y = edge.LabelRadius.Y, edge.LabelRadius.X
		}
	}
	if sideways {
		swapRadii()
	}
	defer func() {
		if sideways {
			swapRadii()
		}
		_, size := graphdef.Bounds()
		transform := func(p Vector) Vector {
			switch graphdef.RankDir {
			case LeftToRight:
				return Vector{X: p.Y, Y: p.X}
			case RightToLeft:
				return Vector{X: size.Y - p.Y, Y: p.X}
			case BottomToTop:
				return Vector{X: p.X, Y: size.Y - p.Y}
			}
			return p
		}
		for _, node := range graphdef.Nodes {
			node.Center = transform(node.Center)
		}
		for _, edge := range graphdef.Edges {
			for i, p := range edge.Path {
				edge.Path[i] = transform(p)
			}
			edge.LabelPos = transform(edge.LabelPos)
		}
	}()

	left := Length(0)
	for _, component := range components(graphdef) {
		hierarchicalComponent(component)
		_, size := component.Bounds()
		shift := Vector{X: left}
		for _, node := range component.Nodes {
			node.Center = node.Center.Add(shift)
		}
		for _, edge := range component.Edges {
			for i, p := range edge.Path {
				edge.Path[i] = p.Add(shift)
			}
			edge.LabelPos = edge.LabelPos.Add(shift)
		}
		left += size.X + 2*graphdef.NodePadding
	}
}

// components splits the graph into connected components, each a Graph
// sharing the original nodes, edges and settings. Components are laid out
// independently and then placed side by side.
func components(graphdef *Graph) []*Graph {
	parent := map[*Node]*Node{}
	var find func(n *Node) *Node
	find = func(n *Node) *Node {
		for parent[n] != nil && parent[n] != n {
			n = parent[n]
		}
		return n
	}
	union := func(a, b *Node) { parent[find(a)] = find(b) }
	for _, node := range graphdef.Nodes {
		parent[node] = node
	}
	for _, edge := range graphdef.Edges {
		union(edge.From, edge.To)
	}
	for _, group := range graphdef.SameRank {
		for _, node := range group[1:] {
			union(group[0], node)
		}
	}
	// min/max pinned nodes share a rank, treat them as connected
	for _, pinned := range [][]*Node{graphdef.MinRank, graphdef.MaxRank} {
		for _, node := range pinned {
			union(pinned[0], node)
		}
	}

	byRoot := map[*Node]*Graph{}
	var result []*Graph
	sub := func(node *Node) *Graph {
		root := find(node)
		graph := byRoot[root]
		if graph == nil {
			copy := *graphdef
			copy.Nodes, copy.Edges, copy.SameRank, copy.MinRank, copy.MaxRank = nil, nil, nil, nil, nil
			graph = &copy
			byRoot[root] = graph
			result = append(result, graph)
		}
		return graph
	}
	for _, node := range graphdef.Nodes {
		g := sub(node)
		g.Nodes = append(g.Nodes, node)
	}
	for _, edge := range graphdef.Edges {
		g := sub(edge.From)
		g.Edges = append(g.Edges, edge)
	}
	for _, group := range graphdef.SameRank {
		g := sub(group[0])
		g.SameRank = append(g.SameRank, group)
	}
	for _, node := range graphdef.MinRank {
		g := sub(node)
		g.MinRank = append(g.MinRank, node)
	}
	for _, node := range graphdef.MaxRank {
		g := sub(node)
		g.MaxRank = append(g.MaxRank, node)
	}
	return result
}

// hierarchicalComponent lays out one connected graph top to bottom,
// placing it to the right of whatever the previous components occupy.
func hierarchicalComponent(graphdef *Graph) {
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

	for _, nodedef := range graphdef.MinRank {
		graph.MinRank.Append(graph.Nodes[nodes[nodedef]])
	}
	for _, nodedef := range graphdef.MaxRank {
		graph.MaxRank.Append(graph.Nodes[nodes[nodedef]])
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

	// labeled edges need a virtual node to hang the label on; doubling the
	// ranks guarantees every edge has one in the middle
	labels := map[[2]hier.ID][]*Edge{} // by unordered node pair
	pair := func(a, b hier.ID) [2]hier.ID {
		if a > b {
			a, b = b, a
		}
		return [2]hier.ID{a, b}
	}
	for _, edge := range graphdef.Edges {
		if edge.Label != "" && edge.From != edge.To {
			key := pair(nodes[edge.From], nodes[edge.To])
			labels[key] = append(labels[key], edge)
		}
	}
	if len(labels) > 0 {
		hier.DoubleRanks(rankedGraph)
	}

	// create virtual nodes
	filledGraph := hier.DefaultAddVirtuals(rankedGraph)

	// order nodes in ranks
	orderedGraph := hier.DefaultOrderRanks(filledGraph)

	// the middle virtual node of every labeled edge carries the labels
	labelNode := map[*hier.Node][]*Edge{}
	for _, source := range orderedGraph.Nodes {
		if source.Virtual {
			continue
		}
		for _, out := range source.Out {
			var chain []*hier.Node
			target := out
			for target.Virtual {
				chain = append(chain, target)
				target = target.Out[0]
			}
			if len(chain) == 0 {
				continue
			}
			if edges := labels[pair(source.ID, target.ID)]; len(edges) > 0 {
				labelNode[chain[len(chain)/2]] = edges
			}
		}
	}

	// assign node sizes
	for id, node := range orderedGraph.Nodes {
		if node.Virtual {
			node.Radius.X = float32(graphdef.EdgePadding)
			node.Radius.Y = float32(graphdef.EdgePadding)
			if edges, ok := labelNode[node]; ok {
				var width, height Length
				for _, edge := range edges {
					width = max(width, edge.LabelRadius.X)
					height += edge.LabelRadius.Y
				}
				node.Radius.X += float32(width + graphdef.EdgePadding)
				node.Radius.Y = float32(height + graphdef.EdgePadding)
			}
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

				point := Vector{Length(target.Center.X), Length(target.Center.Y)}
				if edges, ok := labelNode[target]; ok {
					// the edge passes on the left, the labels stack on the right
					point.X = Length(target.Center.X-target.Radius.X) + graphdef.EdgePadding
					x := point.X + (Length(target.Center.X+target.Radius.X)-point.X)/2
					y := point.Y - Length(target.Radius.Y) + graphdef.EdgePadding
					for _, edge := range edges {
						edge.LabelPos = Vector{X: x, Y: y + edge.LabelRadius.Y}
						y += 2 * edge.LabelRadius.Y
					}
				}
				path = append(path, point)

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

	// edges between the same pair of nodes share one route; spread them out
	pairKey := func(edge *Edge) [2]hier.ID {
		a, b := nodes[edge.From], nodes[edge.To]
		if a > b {
			a, b = b, a
		}
		return [2]hier.ID{a, b}
	}
	pairCount := map[[2]hier.ID]int{}
	for _, edge := range graphdef.Edges {
		pairCount[pairKey(edge)]++
	}
	pairIndex := map[[2]hier.ID]int{}

	for _, edge := range graphdef.Edges {
		sourceid := nodes[edge.From]
		targetid := nodes[edge.To]

		if sourceid == targetid {
			edge.Path = loopPath(edge.From, loopWidth)
			edge.LabelPos = Vector{X: edge.From.Right() + loopWidth + graphdef.EdgePadding + edge.LabelRadius.X, Y: edge.From.Center.Y}
			continue
		}

		var path []Vector
		if p, ok := edgePaths[[2]hier.ID{sourceid, targetid}]; ok {
			path = p
		} else if p, ok := edgePaths[[2]hier.ID{targetid, sourceid}]; ok {
			path = reversePath(p) // the edge was reversed to break a cycle
		} else {
			continue
		}

		// pinned ports override the automatic attachment points
		if edge.FromPort != CompassAuto {
			path = append([]Vector{edge.From.CompassPoint(edge.FromPort)}, path[1:]...)
		}
		if edge.ToPort != CompassAuto {
			path = append(path[:len(path)-1:len(path)-1], edge.To.CompassPoint(edge.ToPort))
		}

		key := pairKey(edge)
		if n := pairCount[key]; n > 1 {
			k := pairIndex[key]
			pairIndex[key]++
			spacing := 2 * graphdef.EdgePadding
			offset := (Length(k) - Length(n-1)/2) * spacing
			path = offsetPath(path, offset, edge.From, edge.To)
		}

		// pinned ports override the automatic attachment points
		path = slices.Clone(path)
		if edge.FromPort != CompassAuto {
			path[0] = edge.From.CompassPoint(edge.FromPort)
		}
		if edge.ToPort != CompassAuto {
			path[len(path)-1] = edge.To.CompassPoint(edge.ToPort)
		}
		edge.Path = path
	}
}

// offsetPath shifts the path sideways by dx and re-clips the ends to the nodes
func offsetPath(path []Vector, dx Length, from, to *Node) []Vector {
	out := make([]Vector, len(path))
	for i, p := range path {
		out[i] = Vector{X: p.X + dx, Y: p.Y}
	}
	if len(out) >= 2 {
		out[0] = from.Boundary(out[1])
		out[len(out)-1] = to.Boundary(out[len(out)-2])
	}
	return out
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
