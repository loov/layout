package layout

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"sort"

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

		node.Radius = node.Radius.Sub(node.peripheryPad)
		node.peripheryPad = Vector{}
		if node.Radius.X <= 0 {
			node.Radius.X = graph.LineHeight
		}
		if node.Radius.Y <= 0 {
			node.Radius.Y = graph.LineHeight
		}
		if !node.FixedSize {
			labelRadius := graph.textRadius(node.DefaultLabel(), node.FontName, node.FontSize)
			if IsHTMLLabel(node.DefaultLabel()) {
				labelRadius = graph.htmlLabelRadius(node.DefaultLabel(), node.FontName, node.FontSize)
			}
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
		if node.Shape == Circle || node.Shape == Square {
			// drawn with the larger radius on both axes
			r := max(node.Radius.X, node.Radius.Y)
			node.Radius = Vector{r, r}
		}
		if node.Peripheries > 1 {
			extra := Length(node.Peripheries-1) * peripheryGap
			node.peripheryPad = Vector{extra, extra}
			node.Radius = node.Radius.Add(node.peripheryPad)
		}
	}

	for _, edge := range graph.Edges {
		if edge.Weight < epsilon {
			edge.Weight = epsilon
		}
		if edge.MinLen < 1 {
			edge.MinLen = 1
		}
		if edge.FontSize <= 0 {
			edge.FontSize = graph.FontSize
		}
		if IsHTMLLabel(edge.Label) {
			edge.LabelRadius = graph.htmlLabelRadius(edge.Label, edge.FontName, edge.FontSize)
		} else if edge.Label != "" {
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

// Presets for Options: Fast trades crossings for speed on large graphs,
// Quality spends more sweeps looking for a better order.
var (
	Fast    = Options{OrderIterations: 4}
	Quality = Options{OrderIterations: 96}
)

// compassInRankFrame maps a physical port into the top-to-bottom frame.
func compassInRankFrame(port Compass, dir RankDir) Compass {
	compass := [...]Compass{North, NorthEast, East, SouthEast, South, SouthWest, West, NorthWest}
	i := slices.Index(compass[:], port)
	if i < 0 {
		return port // automatic and center ports do not change
	}
	switch dir {
	case LeftToRight:
		return compass[(14-i)%8]
	case RightToLeft:
		return compass[(i+6)%8]
	case BottomToTop:
		return compass[(12-i)%8]
	default:
		return port
	}
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
	if graphdef.Pinned {
		layoutPinned(graphdef)
		return nil
	}

	// Compass directions are physical directions, so map them into the
	// temporary rank frame and restore the caller's values afterward.
	// Keyed by edge so that an edge listed twice is mapped only once.
	ports := make(map[*Edge][2]Compass, len(graphdef.Edges))
	for _, edge := range graphdef.Edges {
		if _, ok := ports[edge]; ok {
			continue
		}
		ports[edge] = [2]Compass{edge.FromPort, edge.ToPort}
		edge.FromPort = compassInRankFrame(edge.FromPort, graphdef.RankDir)
		edge.ToPort = compassInRankFrame(edge.ToPort, graphdef.RankDir)
	}
	defer func() {
		for edge, port := range ports {
			edge.FromPort, edge.ToPort = port[0], port[1]
		}
	}()

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
	group := func(what string, nodes []*Node) error {
		if len(nodes) == 0 {
			return fmt.Errorf("%s has no nodes", what)
		}
		for _, node := range nodes {
			if node == nil || !known[node] {
				return fmt.Errorf("%s: node %v is not in the graph", what, node)
			}
		}
		return nil
	}
	for i, nodes := range graph.SameRank {
		if err := group(fmt.Sprintf("same rank group %d", i), nodes); err != nil {
			return err
		}
	}
	for _, pinned := range []struct {
		what  string
		nodes []*Node
	}{{"min rank", graph.MinRank}, {"max rank", graph.MaxRank}} {
		if len(pinned.nodes) == 0 {
			continue
		}
		if err := group(pinned.what, pinned.nodes); err != nil {
			return err
		}
	}
	clusters := make(map[*Cluster]bool, len(graph.Clusters))
	for i, cluster := range graph.Clusters {
		if cluster == nil {
			return fmt.Errorf("cluster %d is nil", i)
		}
		clusters[cluster] = true
	}
	for _, cluster := range graph.Clusters {
		if err := group(fmt.Sprintf("cluster %q", cluster.ID), cluster.Nodes); err != nil {
			return err
		}
		depth := 0
		for parent := cluster.Parent; parent != nil; parent = parent.Parent {
			if !clusters[parent] {
				return fmt.Errorf("cluster %q: parent %q is not in the graph", cluster.ID, parent.ID)
			}
			if depth++; depth > len(graph.Clusters) {
				return fmt.Errorf("cluster %q: parent chain is a cycle", cluster.ID)
			}
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

	// nodes with self-loops need room on their right for the loop and
	// its label
	loopWidth := 2 * graphdef.NodePadding
	loopHeight := min(loopWidth, graphdef.RowPadding)
	loopExtra := map[*Node]Length{}
	loopLeft := map[*Node]Length{} // loops with ports may also pass the left side
	for _, edge := range graphdef.Edges {
		if edge.From == edge.To {
			extra := loopWidth
			if edge.Label != "" {
				extra += graphdef.EdgePadding + 2*edge.LabelRadius.X
			}
			loopExtra[edge.From] = max(loopExtra[edge.From], extra)
			if edge.FromPort != CompassAuto || edge.ToPort != CompassAuto {
				loopLeft[edge.From] = max(loopLeft[edge.From], extra)
			}
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
		from, to := graph.Nodes[nodes[edge.From]], graph.Nodes[nodes[edge.To]]
		graph.AddWeightedEdge(from, to, float32(edge.Weight))
		// parallel edges share one hierarchical edge, the longest wins
		graph.SetMinLen(from, to, max(int32(edge.MinLen), graph.MinLen(from, to)))
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
				if graphdef.Splines != SplinesOrtho {
					// the edge passes on the left, the labels stack on the right
					node.Anchor = -node.Radius.X + float32(graphdef.EdgePadding)
				}
			}
			continue
		}

		nodedef := reverse[hier.ID(id)]
		node.Radius.X = float32(nodedef.Radius.X + graphdef.NodePadding)
		node.Radius.X += float32((loopExtra[nodedef] + loopLeft[nodedef]) / 2)
		node.Radius.Y = float32(nodedef.Radius.Y + graphdef.RowPadding)
	}

	// reserve room for the cluster box lines and the label strip by making
	// the cluster's first and last border nodes taller than their ranks;
	// outer clusters first so nested boxes stack
	grow := func(nodes []*hier.Node, rank int, extra float32) {
		half := float32(0)
		for _, node := range orderedGraph.ByRank[rank] {
			half = max(half, node.Radius.Y)
		}
		for _, node := range nodes {
			node.Radius.Y = half + extra
		}
	}
	for _, clusterdef := range graphdef.Clusters {
		cluster := clusters[clusterdef]
		pad := float32(graphdef.RowPadding / 2)
		top := pad
		if clusterdef.Label != "" {
			top += float32(2 * graphdef.textRadius(clusterdef.Label, "", graphdef.FontSize).Y)
		}
		last := len(cluster.Left) - 1
		grow([]*hier.Node{cluster.Left[0], cluster.Right[0]}, cluster.MinRank, top)
		grow([]*hier.Node{cluster.Left[last], cluster.Right[last]}, cluster.MaxRank, pad)
	}

	// position nodes
	positionedGraph := orderedGraph
	hier.Position(positionedGraph, graphdef.Splines != SplinesOrtho)

	// assign final positions; loop nodes were widened symmetrically,
	// shift them left so the extra room is on the right
	for nodedef, id := range nodes {
		node := positionedGraph.Nodes[id]
		nodedef.Center.X = Length(node.Center.X)
		nodedef.Center.Y = Length(node.Center.Y)
		nodedef.Center.X -= (loopExtra[nodedef] - loopLeft[nodedef]) / 2
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
	obstacles := newObstacles(byRank, graphdef.EdgePadding)

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
					point.X = Length(target.Center.X + target.Anchor)
					// labels sit right beside the line, stacked vertically
					// around the node's center
					var height Length
					for _, edge := range edges {
						height += 2 * edge.LabelRadius.Y
					}
					y := point.Y - height/2
					for _, edge := range edges {
						edge.LabelPos = Vector{X: point.X + graphdef.EdgePadding + edge.LabelRadius.X, Y: y + edge.LabelRadius.Y}
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

			if graphdef.Splines != SplinesOrtho {
				// orthogonal edges run on virtual node columns and rank
				// channels, which are free of nodes by construction
				path = routeAround(path, obstacles, sourcedef, targetdef, graphdef.EdgePadding)
				path = routeAroundClusters(path, graphdef.Clusters, sourcedef, targetdef, graphdef.EdgePadding)
			}

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
			edge.Path = loopPath(edge, loopWidth, loopHeight)
			edge.LabelPos = Vector{X: edge.From.Right() + loopWidth + graphdef.EdgePadding + edge.LabelRadius.X, Y: edge.From.Center.Y}
			if edge.FromPort != CompassAuto || edge.ToPort != CompassAuto {
				// ported loops can sit on any side; label the middle of the loop
				p, q := edge.Path[(len(edge.Path)-1)/2], edge.Path[len(edge.Path)/2]
				mid := Vector{X: (p.X + q.X) / 2, Y: (p.Y + q.Y) / 2}
				d := mid.Sub(edge.From.Center)
				if n := Length(math.Hypot(float64(d.X), float64(d.Y))); n > 0 {
					d = Vector{X: d.X / n, Y: d.Y / n}
					gap := graphdef.EdgePadding + Length(math.Abs(float64(d.X)))*edge.LabelRadius.X + Length(math.Abs(float64(d.Y)))*edge.LabelRadius.Y
					edge.LabelPos = mid.Add(Vector{X: d.X * gap, Y: d.Y * gap})
				}
			}
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

	if graphdef.Splines == SplinesOrtho {
		// row extents per rank: real node boxes, virtual nodes are flat
		rows := make([][2]Length, len(positionedGraph.ByRank))
		for r, layer := range positionedGraph.ByRank {
			rows[r] = [2]Length{Length(math.Inf(1)), Length(math.Inf(-1))}
			for _, node := range layer {
				top, bottom := Length(node.Center.Y), Length(node.Center.Y)
				if !node.Virtual {
					top, bottom = reverse[node.ID].Top(), reverse[node.ID].Bottom()
				}
				rows[r][0], rows[r][1] = min(rows[r][0], top), max(rows[r][1], bottom)
			}
		}
		orthoEdges(graphdef, rows, graphdef.EdgePadding)
	}
	if graphdef.Splines != SplinesOrtho {
		spreadWaypoints(graphdef.Edges, graphdef.EdgePadding)
		spreadEnds(graphdef, 9*Point)
	}
	if graphdef.Splines == SplinesLine {
		for _, edge := range graphdef.Edges {
			if edge.From != edge.To && len(edge.Path) > 2 {
				from, to := edge.Path[0], edge.Path[len(edge.Path)-1]
				if edge.FromPort == CompassAuto {
					from = edge.From.Boundary(edge.To.Center)
				}
				if edge.ToPort == CompassAuto {
					to = edge.To.Boundary(edge.From.Center)
				}
				edge.Path = []Vector{from, to}
			}
		}
	}
	nudgeLabels(graphdef.Edges, graphdef.Nodes, graphdef.EdgePadding, 2*graphdef.RowPadding)
}

// flattenPath approximates the rounded corners drawn by the writers
// (quadratic curves of the given radius) with two extra points per corner.
func flattenPath(path []Vector, radius, maxDeviation Length) []Vector {
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
		r := CornerRadius(prev, p, next, radius, maxDeviation)
		in, exit := towards(p, prev, r), towards(p, next, r)
		mid := Vector{X: (in.X + 2*p.X + exit.X) / 4, Y: (in.Y + 2*p.Y + exit.Y) / 4}
		out = append(out, in, mid, exit)
	}
	return append(out, path[len(path)-1])
}

// spreadWaypoints moves interior path points that several edges share
// (detours around the same node) sideways so that the edges don't run on
// top of each other, in the order they head so that they don't cross.
func spreadWaypoints(edges []*Edge, pad Length) {
	type at struct {
		edge  *Edge
		index int
		next  Vector // original following point
	}
	shared := map[Vector][]at{}
	for _, edge := range edges {
		for i := 1; i+1 < len(edge.Path); i++ {
			shared[edge.Path[i]] = append(shared[edge.Path[i]], at{edge, i, edge.Path[i+1]})
		}
	}
	points := make([]Vector, 0, len(shared))
	for point, list := range shared {
		if len(list) > 1 {
			points = append(points, point)
		}
	}
	sort.Slice(points, func(i, k int) bool {
		if points[i].X != points[k].X {
			return points[i].X < points[k].X
		}
		return points[i].Y < points[k].Y
	})
	for _, point := range points {
		list := shared[point]
		// spread along x, so the order of the following points' x keeps
		// the edges from crossing
		sort.SliceStable(list, func(i, k int) bool { return list[i].next.X < list[k].next.X })
		for i, a := range list {
			offset := (Length(i) - Length(len(list)-1)/2) * 2 * pad
			a.edge.Path[a.index] = point.Add(Vector{offset, 0})
		}
	}
}

// spreadEnds keeps the attachment points on every node at least minSep
// apart along the outline so that arrowheads don't stack, pushing the
// crowded ones apart around their mean direction. Edges pinned to a port
// keep their point.
func spreadEnds(graph *Graph, minSep Length) {
	type end struct {
		edge  *Edge
		start bool
		angle float64
		fixed bool // a loop's attachment, which stays where it is
	}
	byNode := map[*Node][]end{}
	for _, edge := range graph.Edges {
		if len(edge.Path) < 2 {
			continue
		}
		angle := func(node *Node, p Vector) float64 {
			return math.Atan2(float64(p.Y-node.Center.Y), float64(p.X-node.Center.X))
		}
		if edge.From == edge.To {
			byNode[edge.From] = append(byNode[edge.From],
				end{edge, true, angle(edge.From, edge.Path[0]), true},
				end{edge, false, angle(edge.From, edge.Path[len(edge.Path)-1]), true})
			continue
		}
		if edge.FromPort == CompassAuto {
			byNode[edge.From] = append(byNode[edge.From], end{edge, true, angle(edge.From, edge.Path[1]), false})
		}
		if edge.ToPort == CompassAuto {
			byNode[edge.To] = append(byNode[edge.To], end{edge, false, angle(edge.To, edge.Path[len(edge.Path)-2]), false})
		}
	}
	for node, ends := range byNode {
		if len(ends) < 2 {
			continue
		}
		sort.Slice(ends, func(i, k int) bool { return ends[i].angle < ends[k].angle })
		// start the sequence after the largest gap so that the ±π seam
		// never falls between neighbors
		gap, at := ends[0].angle+2*math.Pi-ends[len(ends)-1].angle, len(ends)-1
		for i := 1; i < len(ends); i++ {
			if d := ends[i].angle - ends[i-1].angle; d > gap {
				gap, at = d, i-1
			}
		}
		for i := range at + 1 {
			ends[i].angle += 2 * math.Pi
		}
		ends = append(ends[at+1:], ends[:at+1]...)
		// angle step from the arc length on the smaller radius, so that
		// it is enough along the flat sides of wide nodes too
		step := float64(minSep) / float64(min(node.Radius.X, node.Radius.Y))
		for i := 1; i < len(ends); i++ {
			d := ends[i].angle - ends[i-1].angle
			if d >= step {
				continue
			}
			switch {
			case ends[i].fixed && !ends[i-1].fixed:
				ends[i-1].angle -= step - d
			case ends[i-1].fixed && !ends[i].fixed:
				ends[i].angle += step - d
			case !ends[i].fixed:
				// split the push, moving everything before along
				for k := range i {
					ends[k].angle -= (step - d) / 2
				}
				ends[i].angle += (step - d) / 2
			}
		}
		for _, e := range ends {
			if e.fixed {
				continue
			}
			p := node.Boundary(node.Center.Add(Vector{Length(math.Cos(e.angle)), Length(math.Sin(e.angle))}))
			if e.start {
				e.edge.Path[0] = p
			} else {
				e.edge.Path[len(e.edge.Path)-1] = p
			}
		}
	}
}

// layoutPinned keeps node positions and gives edges without a path a
// straight line, labels without a position the middle of their path.
func layoutPinned(graph *Graph) {
	for _, edge := range graph.Edges {
		if len(edge.Path) < 2 {
			if edge.From == edge.To {
				edge.Path = loopPath(edge, edge.From.Radius.X, edge.From.Radius.X)
			} else {
				from, to := edge.From.Boundary(edge.To.Center), edge.To.Boundary(edge.From.Center)
				if edge.FromPort != CompassAuto {
					from = edge.From.CompassPoint(edge.FromPort)
				}
				if edge.ToPort != CompassAuto {
					to = edge.To.CompassPoint(edge.ToPort)
				}
				edge.Path = []Vector{from, to}
			}
		}
		if edge.Label != "" && edge.LabelPos == (Vector{}) {
			mid := edge.Path[len(edge.Path)/2]
			if len(edge.Path)%2 == 0 {
				a, b := edge.Path[len(edge.Path)/2-1], edge.Path[len(edge.Path)/2]
				mid = Vector{(a.X + b.X) / 2, (a.Y + b.Y) / 2}
			}
			edge.LabelPos = mid.Add(Vector{edge.LabelRadius.X, 0})
		}
	}
}

// nudgeLabels slides edge labels sideways along their rank until they
// clear every edge path and the labels placed before them.
func nudgeLabels(edges []*Edge, nodes []*Node, pad, radius Length) {
	paths := make([][]Vector, len(edges))
	for i, edge := range edges {
		paths[i] = flattenPath(edge.Path, radius, pad)
	}
	var placed []*Edge
	clear := func(edge *Edge, at Vector) bool {
		// a hair inside the padding, so that a line at exactly pad
		// distance (the label's own edge) does not count as a hit
		tl := at.Add(Vector{-edge.LabelRadius.X - pad + 0.01, -edge.LabelRadius.Y})
		br := at.Add(Vector{edge.LabelRadius.X + pad - 0.01, edge.LabelRadius.Y})
		for _, path := range paths {
			for i := 0; i+1 < len(path); i++ {
				if segmentHitsRect(path[i], path[i+1], tl, br) {
					return false
				}
			}
		}
		// nodes only need to stay clear of the text itself
		for _, node := range nodes {
			if node.Left() < br.X-pad && tl.X+pad < node.Right() && node.Top() < br.Y && tl.Y < node.Bottom() {
				return false
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
		// candidates in growing rings: along the rank first, then across
		// it, then diagonally; never further from the edge than the text
		// height, so the label stays attributable
		own := paths[slices.Index(edges, edge)]
		// snap to the drawn path first: the label was placed against a
		// waypoint, but rounding and multi-edge offsets move the line
		if p, ok := nearestOnPath(own, edge.LabelPos); ok {
			d := edge.LabelPos.Sub(p)
			if l := math.Hypot(float64(d.X), float64(d.Y)); l > 0 {
				ux, uy := float64(d.X)/l, float64(d.Y)/l
				support := math.Abs(ux)*float64(edge.LabelRadius.X) + math.Abs(uy)*float64(edge.LabelRadius.Y)
				dist := float64(pad) + support
				// this side, or the other side of the line when only that
				// one is free (parallel edges of a pair)
				same := p.Add(Vector{Length(ux * dist), Length(uy * dist)})
				other := p.Add(Vector{Length(-ux * dist), Length(-uy * dist)})
				edge.LabelPos = same
				if !clear(edge, same) && clear(edge, other) {
					edge.LabelPos = other
				}
			}
		}
		near := func(at Vector) bool {
			tl, br := at.Sub(edge.LabelRadius), at.Add(edge.LabelRadius)
			best := math.Inf(1)
			for i := 0; i+1 < len(own); i++ {
				best = math.Min(best, rectSegmentDistance(tl, br, own[i], own[i+1]))
			}
			// the text height is the smaller extent (radii are swapped
			// for sideways layouts), minus a margin
			return best <= float64(2*min(edge.LabelRadius.X, edge.LabelRadius.Y)-pad)
		}
		// slide along the own path, nearest spot first, trying both sides
		found := false
		if !clear(edge, edge.LabelPos) {
			type spot struct {
				at   Vector
				dist float64
			}
			var spots []spot
			base, _ := nearestOnPath(own, edge.LabelPos)
			for i := 0; i+1 < len(own) && !found; i++ {
				a, b := own[i], own[i+1]
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
					d := math.Hypot(float64(p.X-base.X), float64(p.Y-base.Y))
					spots = append(spots,
						spot{p.Add(Vector{Length(nx * off), Length(ny * off)}), d},
						spot{p.Add(Vector{Length(-nx * off), Length(-ny * off)}), d})
				}
			}
			sort.SliceStable(spots, func(i, k int) bool { return spots[i].dist < spots[k].dist })
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
				if at := edge.LabelPos.Add(Vector{dir.X * d, dir.Y * d}); near(at) && clear(edge, at) {
					edge.LabelPos, found = at, true
					break
				}
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
func loopPath(edge *Edge, width, height Length) []Vector {
	node := edge.From
	right := node.Right() + width
	up := Vector{X: node.Right(), Y: node.Center.Y - node.Radius.Y/2}
	down := Vector{X: node.Right(), Y: node.Center.Y + node.Radius.Y/2}
	if edge.FromPort == CompassAuto && edge.ToPort == CompassAuto {
		return []Vector{
			node.Boundary(up),
			{X: right, Y: up.Y},
			{X: right, Y: down.Y},
			node.Boundary(down),
		}
	}

	// With ports, leave each end straight out to the node box grown by
	// width and height, then go around it on the shorter side.
	from, to := node.Boundary(up), node.Boundary(down)
	if edge.FromPort != CompassAuto {
		from = node.CompassPoint(edge.FromPort)
	}
	if edge.ToPort != CompassAuto {
		to = node.CompassPoint(edge.ToPort)
	}
	outward := func(p Vector) Vector {
		d := Vector{X: 1} // the center port leaves to the right
		if p != node.Center {
			v := p.Sub(node.Center)
			n := Length(math.Hypot(float64(v.X), float64(v.Y)))
			d = Vector{X: v.X / n, Y: v.Y / n}
		}
		// where the ray along d leaves the grown box
		t := Length(math.Inf(1))
		if d.X != 0 {
			t = (node.Radius.X + width) / Length(math.Abs(float64(d.X)))
		}
		if d.Y != 0 {
			t = min(t, (node.Radius.Y+height)/Length(math.Abs(float64(d.Y))))
		}
		return node.Center.Add(Vector{X: d.X * t, Y: d.Y * t})
	}
	a, b := outward(from), outward(to)
	if from == to {
		// both ends on one port: split the loop sideways
		d := a.Sub(from)
		side := Vector{X: -d.Y / 2, Y: d.X / 2}
		return []Vector{from, a.Add(side), a.Sub(side), to}
	}
	angle := func(p Vector) float64 {
		return math.Atan2(float64(p.Y-node.Center.Y), float64(p.X-node.Center.X))
	}
	start := angle(a)
	sweep := math.Remainder(angle(b)-start, 2*math.Pi)
	type corner struct {
		at float64
		p  Vector
	}
	var corners []corner
	for _, p := range []Vector{
		{X: node.Left() - width, Y: node.Top() - height},
		{X: node.Right() + width, Y: node.Top() - height},
		{X: node.Right() + width, Y: node.Bottom() + height},
		{X: node.Left() - width, Y: node.Bottom() + height},
	} {
		at := math.Remainder(angle(p)-start, 2*math.Pi)
		if (sweep > 0 && at > 0 && at < sweep) || (sweep < 0 && at < 0 && at > sweep) {
			corners = append(corners, corner{math.Abs(at), p})
		}
	}
	slices.SortFunc(corners, func(x, y corner) int { return cmp.Compare(x.at, y.at) })
	path := []Vector{from, a}
	for _, c := range corners {
		path = append(path, c.p)
	}
	return append(path, b, to)
}

// obstacles indexes the real nodes per rank, sorted by x, with the
// largest padded radius per rank for quick rejection.
type obstacles struct {
	byRank               [][]*Node
	rowRadius, colRadius []Length
}

func newObstacles(byRank [][]*Node, pad Length) *obstacles {
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
func routeAround(path []Vector, obs *obstacles, from, to *Node, pad Length) []Vector {
	var lastHit *Node
	inserted := 0
	for i := 0; i+1 < len(path) && inserted < 16; i++ {
		a, b := path[i], path[i+1]
		var hit *Node
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

// nearestOnPath returns the point of the polyline closest to p
func nearestOnPath(path []Vector, p Vector) (Vector, bool) {
	best, bestDist := Vector{}, math.Inf(1)
	for i := 0; i+1 < len(path); i++ {
		a, b := path[i], path[i+1]
		dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
		t := 0.0
		if l := dx*dx + dy*dy; l > 0 {
			t = math.Max(0, math.Min(1, (float64(p.X-a.X)*dx+float64(p.Y-a.Y)*dy)/l))
		}
		q := Vector{a.X + Length(t*dx), a.Y + Length(t*dy)}
		if d := math.Hypot(float64(p.X-q.X), float64(p.Y-q.Y)); d < bestDist {
			best, bestDist = q, d
		}
	}
	return best, bestDist < math.Inf(1)
}
