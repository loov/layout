package layout

import (
	"fmt"
	"math"
	"slices"

	"github.com/loov/layout/internal/hier"
)

const epsilon = 1e-6

// peripheryGap is the distance between a node's extra outlines
const peripheryGap = 4 * Point

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

		if node.FontSize <= 0 {
			node.FontSize = graph.FontSize
		}

		if node.Radius.X <= 0 {
			node.Radius.X = graph.LineHeight
		}
		if node.Radius.Y <= 0 {
			node.Radius.Y = graph.LineHeight
		}
		if !node.FixedSize {
			labelRadius := graph.textRadius(node.DefaultLabel(), node.FontName, node.FontSize)
			labelRadius.X += node.FontSize * 0.5
			labelRadius.Y += node.FontSize * 0.25
			if node.Shape == Record {
				labelRadius = graph.recordRadius(node)
			}

			if node.Radius.X < labelRadius.X {
				node.Radius.X = labelRadius.X
			}
			if node.Radius.Y < labelRadius.Y {
				node.Radius.Y = labelRadius.Y
			}
		}
		if node.Peripheries > 1 {
			extra := Length(node.Peripheries-1) * peripheryGap
			node.Radius = node.Radius.Add(Vector{extra, extra})
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
			edge.LabelRadius = graph.textRadius(edge.Label, edge.FontName, edge.FontSize)
		}
	}
}

// Hierarchical lays out the graph top-down in ranks (Sugiyama style):
// cycles are broken, nodes are assigned to ranks, ordered within ranks to
// reduce crossings, positioned, and finally edge paths are computed.
//
// It sets Node.Center and Edge.Path. It fails when an edge refers to a
// node that is not part of the graph.
func Hierarchical(graphdef *Graph) error {
	return HierarchicalWith(graphdef, Options{})
}

// Options tunes the hierarchical layout. The zero value gives the defaults.
type Options struct {
	// OrderIterations is the number of crossing reduction sweeps;
	// 0 uses the default of 24. More sweeps can help on large graphs.
	OrderIterations int
	// NoRankBalance keeps nodes that could go on several ranks at the
	// topmost one instead of spreading them over the least crowded ranks.
	NoRankBalance bool
}

// HierarchicalWith is Hierarchical with explicit options.
func HierarchicalWith(graphdef *Graph, opts Options) error {
	if err := graphdef.validate(); err != nil {
		return err
	}
	if opts.OrderIterations <= 0 {
		opts.OrderIterations = hier.DefaultOrderIterations
	}
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
		for _, cluster := range graphdef.Clusters {
			a, b := transform(cluster.TopLeft), transform(cluster.BottomRight)
			cluster.TopLeft = Vector{min(a.X, b.X), min(a.Y, b.Y)}
			cluster.BottomRight = Vector{max(a.X, b.X), max(a.Y, b.Y)}
		}
	}()

	left := Length(0)
	for _, component := range components(graphdef) {
		hierarchicalComponent(component, opts)
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
		for _, cluster := range component.Clusters {
			cluster.TopLeft = cluster.TopLeft.Add(shift)
			cluster.BottomRight = cluster.BottomRight.Add(shift)
		}
		left += size.X + 2*graphdef.NodePadding
	}
	return nil
}

// validate checks that every edge connects nodes of the graph
func (graph *Graph) validate() error {
	known := make(map[*Node]bool, len(graph.Nodes))
	for i, node := range graph.Nodes {
		if node == nil {
			return fmt.Errorf("node %d is nil", i)
		}
		known[node] = true
	}
	for i, edge := range graph.Edges {
		switch {
		case edge == nil:
			return fmt.Errorf("edge %d is nil", i)
		case edge.From == nil || edge.To == nil:
			return fmt.Errorf("edge %d has a nil endpoint", i)
		case !known[edge.From]:
			return fmt.Errorf("edge %v: node %q is not in the graph", edge, edge.From)
		case !known[edge.To]:
			return fmt.Errorf("edge %v: node %q is not in the graph", edge, edge.To)
		}
	}
	return nil
}

// components splits the graph into connected components, each a Graph
// sharing the original nodes, edges and settings. Components are laid out
// independently and then placed side by side.
func components(graphdef *Graph) []*Graph {
	index := make(map[*Node]int, len(graphdef.Nodes))
	for i, node := range graphdef.Nodes {
		index[node] = i
	}
	parent := make([]int, len(graphdef.Nodes))
	for i := range parent {
		parent[i] = i
	}
	var findIndex func(i int) int
	findIndex = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]] // path halving
			i = parent[i]
		}
		return i
	}
	find := func(n *Node) *Node { return graphdef.Nodes[findIndex(index[n])] }
	union := func(a, b *Node) { parent[findIndex(index[a])] = findIndex(index[b]) }
	for _, edge := range graphdef.Edges {
		union(edge.From, edge.To)
	}
	for _, group := range graphdef.SameRank {
		for _, node := range group[1:] {
			union(group[0], node)
		}
	}
	for _, cluster := range graphdef.Clusters {
		for _, node := range cluster.Nodes[1:] {
			union(cluster.Nodes[0], node)
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
			copy.Nodes, copy.Edges, copy.SameRank, copy.MinRank, copy.MaxRank, copy.Clusters = nil, nil, nil, nil, nil, nil
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
	for _, cluster := range graphdef.Clusters {
		if len(cluster.Nodes) == 0 {
			continue
		}
		g := sub(cluster.Nodes[0])
		g.Clusters = append(g.Clusters, cluster)
	}
	return result
}

// hierarchicalComponent lays out one connected graph top to bottom,
// placing it to the right of whatever the previous components occupy.
func hierarchicalComponent(graphdef *Graph, opts Options) {
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
	decycledGraph := graph
	hier.Decycle(decycledGraph)

	// assign nodes to ranks
	rankedGraph := decycledGraph
	hier.RankWith(rankedGraph, !opts.NoRankBalance)

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
	filledGraph := rankedGraph
	hier.AddVirtuals(filledGraph)

	// cluster borders
	clusters := map[*Cluster]*hier.Cluster{}
	for _, clusterdef := range graphdef.Clusters {
		cluster := &hier.Cluster{}
		for _, nodedef := range clusterdef.Nodes {
			cluster.Members.Append(filledGraph.Nodes[nodes[nodedef]])
		}
		clusters[clusterdef] = cluster
		filledGraph.Clusters = append(filledGraph.Clusters, cluster)
	}
	for clusterdef, cluster := range clusters {
		cluster.Parent = clusters[clusterdef.Parent]
	}
	hier.AddClusterBorders(filledGraph)

	// order nodes in ranks
	orderedGraph := filledGraph
	hier.OrderRanksN(orderedGraph, opts.OrderIterations)

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

		nodedef := reverse[hier.ID(id)]
		node.Radius.X = float32(nodedef.Radius.X + graphdef.NodePadding)
		if hasLoop[nodedef] {
			node.Radius.X += float32(loopWidth / 2)
		}
		node.Radius.Y = float32(nodedef.Radius.Y + graphdef.RowPadding)
	}

	// reserve a label strip at the top of every labeled cluster by making
	// its first border nodes taller than the rank; outer clusters first so
	// nested labels stack
	for _, clusterdef := range graphdef.Clusters {
		if clusterdef.Label == "" {
			continue
		}
		cluster := clusters[clusterdef]
		half := float32(0)
		for _, node := range orderedGraph.ByRank[cluster.MinRank] {
			half = max(half, node.Radius.Y)
		}
		height := float32(2 * graphdef.textRadius(clusterdef.Label, "", graphdef.FontSize).Y)
		cluster.Left[0].Radius.Y = half + height
		cluster.Right[0].Radius.Y = half + height
	}

	// position nodes
	positionedGraph := orderedGraph
	hier.Position(positionedGraph)

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

	// cluster boxes span their borders horizontally and their members
	// vertically, with room for the label on top; inner boxes are
	// finished first so that outer boxes can enclose them
	byDepth := slices.Clone(graphdef.Clusters)
	slices.SortStableFunc(byDepth, func(a, b *Cluster) int { return b.depth() - a.depth() })
	for _, clusterdef := range byDepth {
		clusterdef.TopLeft = Vector{Length(math.Inf(1)), Length(math.Inf(1))}
		clusterdef.BottomRight = Vector{Length(math.Inf(-1)), Length(math.Inf(-1))}
	}
	for _, clusterdef := range byDepth {
		cluster := clusters[clusterdef]
		left, right := Length(math.Inf(1)), Length(math.Inf(-1))
		for i := range cluster.Left {
			left = min(left, Length(cluster.Left[i].Center.X-cluster.Left[i].Radius.X))
			right = max(right, Length(cluster.Right[i].Center.X+cluster.Right[i].Radius.X))
		}
		top, bottom := Length(math.Inf(1)), Length(math.Inf(-1))
		for _, nodedef := range clusterdef.Nodes {
			top = min(top, nodedef.Top())
			bottom = max(bottom, nodedef.Bottom())
		}
		top -= graphdef.RowPadding / 2
		bottom += graphdef.RowPadding / 2
		// enclose nested boxes
		left, top = min(left, clusterdef.TopLeft.X), min(top, clusterdef.TopLeft.Y)
		right, bottom = max(right, clusterdef.BottomRight.X), max(bottom, clusterdef.BottomRight.Y)
		if clusterdef.Label != "" {
			top -= 2 * graphdef.textRadius(clusterdef.Label, "", graphdef.FontSize).Y
		}
		clusterdef.TopLeft = Vector{left, top}
		clusterdef.BottomRight = Vector{right, bottom}
		if parent := clusterdef.Parent; parent != nil {
			pad := graphdef.RowPadding / 2
			parent.TopLeft = Vector{min(parent.TopLeft.X, left), min(parent.TopLeft.Y, top-pad)}
			parent.BottomRight = Vector{max(parent.BottomRight.X, right), max(parent.BottomRight.Y, bottom+pad)}
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
			path = routeAroundClusters(path, graphdef.Clusters, sourcedef, targetdef, graphdef.EdgePadding)

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

	nudgeLabels(graphdef.Edges, graphdef.EdgePadding, 2*graphdef.RowPadding)
}

// flattenPath approximates the rounded corners drawn by the writers
// (quadratic curves of the given radius) with two extra points per corner.
func flattenPath(path []Vector, radius Length) []Vector {
	if len(path) < 3 {
		return path
	}
	length := func(a, b Vector) Length {
		return Length(math.Hypot(float64(b.X-a.X), float64(b.Y-a.Y)))
	}
	towards := func(a, b Vector, d Length) Vector {
		l := length(a, b)
		if l == 0 {
			return a
		}
		return Vector{X: a.X + (b.X-a.X)*d/l, Y: a.Y + (b.Y-a.Y)*d/l}
	}
	out := []Vector{path[0]}
	for i := 1; i+1 < len(path); i++ {
		prev, p, next := path[i-1], path[i], path[i+1]
		r := min(radius, length(prev, p)/2, length(p, next)/2)
		in, exit := towards(p, prev, r), towards(p, next, r)
		mid := Vector{X: (in.X + 2*p.X + exit.X) / 4, Y: (in.Y + 2*p.Y + exit.Y) / 4}
		out = append(out, in, mid, exit)
	}
	return append(out, path[len(path)-1])
}

// nudgeLabels slides edge labels sideways along their rank until they
// clear every edge path and the labels placed before them.
func nudgeLabels(edges []*Edge, pad, radius Length) {
	paths := make([][]Vector, len(edges))
	for i, edge := range edges {
		paths[i] = flattenPath(edge.Path, radius)
	}
	var placed []*Edge
	clear := func(edge *Edge, at Vector) bool {
		tl := at.Add(Vector{-edge.LabelRadius.X - pad, -edge.LabelRadius.Y})
		br := at.Add(Vector{edge.LabelRadius.X + pad, edge.LabelRadius.Y})
		for _, path := range paths {
			for i := 0; i+1 < len(path); i++ {
				if segmentHitsRect(path[i], path[i+1], tl, br) {
					return false
				}
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
	for _, edge := range edges {
		if edge.Label == "" || len(edge.Path) < 2 {
			continue
		}
		// alternate right and left in growing steps
		for step := 0; step <= 8; step++ {
			dx := Length((step+1)/2) * 2 * pad
			if step%2 == 0 {
				dx = -dx
			}
			if at := edge.LabelPos.Add(Vector{dx, 0}); clear(edge, at) {
				edge.LabelPos = at
				break
			}
		}
		placed = append(placed, edge)
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
	var lastHit *Node
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
		path = slices.Insert(path, i+1, way)
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
func segmentHitsBox(a, b Vector, node *Node, pad Length) bool {
	return segmentHitsRect(a, b, Vector{node.Left() - pad, node.Top() - pad}, Vector{node.Right() + pad, node.Bottom() + pad})
}

// routeAroundClusters detours segments that cut across a cluster box the
// edge does not belong to, going around the nearest corner.
func routeAroundClusters(path []Vector, clusters []*Cluster, from, to *Node, pad Length) []Vector {
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
