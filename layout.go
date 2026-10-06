package layout

import (
	"cmp"
	"math"
	"slices"

	"github.com/loov/layout/internal/hier"
)

const epsilon = 1e-6

// peripheryGap is the distance between a node's extra outlines
const peripheryGap = 4 * Point

// pointRadius is the default radius of a PointShape node
const pointRadius = 3 * Point

// Hierarchical lays out the graph top-down in ranks (Sugiyama style):
// cycles are broken, nodes are assigned to ranks, ordered within ranks to
// reduce crossings, positioned, and finally edge paths are computed.
//
// The graph is not modified. It fails when an edge refers to a node that
// is not part of the graph.
func Hierarchical(graph *Graph, opts Options) (*Layout, error) {
	if err := graph.validate(); err != nil {
		return nil, err
	}
	work := newWorkGraph(graph)
	work.ForText = opts.ForText
	hierarchical(work, opts)
	return work.result(), nil
}

// Options tunes the hierarchical layout. The zero value gives the defaults.
type Options struct {
	// OrderIterations is the number of crossing reduction sweeps;
	// 0 uses the default of 24. More sweeps can help on large graphs.
	OrderIterations int
	// NoRankBalance keeps nodes that could go on several ranks at the
	// topmost one instead of spreading them over the least crowded ranks.
	NoRankBalance bool
	// Align shifts nodes along their ranks, see Align constants.
	Align Align
	// ForText lays the graph out for format/text: ortho splines, rows
	// between ranks for the horizontal runs and arrowheads, and nodes
	// sized in character cells. Layout.Graph has the changed settings.
	ForText bool
}

// Align picks how nodes are spread along their ranks. In left-to-right
// and right-to-left layouts, left is the top and right the bottom.
type Align int

const (
	// AlignBalanced centers nodes among their neighbors, keeping long
	// edges straight and the layout narrow.
	AlignBalanced Align = iota
	// AlignLeft puts nodes over their first neighbor in the rank before
	// and packs the layout to the left.
	AlignLeft
	// AlignRight puts nodes over their last neighbor in the rank before
	// and packs the layout to the right.
	AlignRight
)

// Presets for Options: Fast trades crossings for speed on large graphs,
// Quality spends more sweeps looking for a better order.
var (
	Fast    = Options{OrderIterations: 4}
	Quality = Options{OrderIterations: 96}
)

// sideways reports whether ranks run left or right
func sideways(dir RankDir) bool { return dir == LeftToRight || dir == RightToLeft }

// labelSide is the side of an edge, in the top to bottom frame, that its
// label goes on: right of downward edges, and above sideways ones, which
// is the left in the frame
func labelSide(dir RankDir) float32 {
	if sideways(dir) {
		return -1
	}
	return 1
}

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

// hierarchical lays out the working copy of a validated graph.
func hierarchical(graphdef *lgraph, opts Options) {
	if opts.OrderIterations <= 0 {
		opts.OrderIterations = hier.DefaultOrderIterations
	}
	if graphdef.ForText {
		graphdef.prepareText()
	}
	graphdef.assignDefaults()
	if graphdef.Pinned {
		layoutPinned(graphdef)
		return
	}
	graphdef.resolveFields()

	// edges are updated in place below, an edge listed twice only once
	edges := uniqueEdges(graphdef.Edges)

	// Compass directions are physical directions, so map them into the
	// rank frame the layout works in.
	for _, edge := range edges {
		edge.FromPort = compassInRankFrame(edge.FromPort, graphdef.RankDir)
		edge.ToPort = compassInRankFrame(edge.ToPort, graphdef.RankDir)
	}

	// lay out top to bottom in a transposed/flipped frame, then map back
	sideways := graphdef.RankDir == LeftToRight || graphdef.RankDir == RightToLeft
	swapRadii := func() {
		for _, node := range graphdef.Nodes {
			node.Radius.X, node.Radius.Y = node.Radius.Y, node.Radius.X
			node.pad.X, node.pad.Y = node.pad.Y, node.pad.X
		}
		for _, edge := range edges {
			edge.LabelRadius.X, edge.LabelRadius.Y = edge.LabelRadius.Y, edge.LabelRadius.X
		}
	}
	if sideways {
		swapRadii()
	}
	defer func() {
		// mirror within the bounds, measured in the frame, so that flipped
		// layouts keep their margins
		lo, hi := graphdef.Bounds()
		if sideways {
			swapRadii()
		}
		transform := func(p Vector) Vector {
			switch graphdef.RankDir {
			case LeftToRight:
				return Vector{X: p.Y, Y: p.X}
			case RightToLeft:
				return Vector{X: lo.Y + hi.Y - p.Y, Y: p.X}
			case BottomToTop:
				return Vector{X: p.X, Y: lo.Y + hi.Y - p.Y}
			}
			return p
		}
		for _, node := range graphdef.Nodes {
			node.Center = transform(node.Center)
		}
		for _, edge := range edges {
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

	// the components share it, see lgraph.merged
	graphdef.merged = map[*ledge][2]int{}
	left := Length(0)
	for _, component := range components(graphdef) {
		hierarchicalComponent(component, opts)
		_, size := component.Bounds()
		shift := Vector{X: left}
		for _, node := range component.Nodes {
			node.Center = node.Center.Add(shift)
		}
		for _, edge := range uniqueEdges(component.Edges) {
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
}

// uniqueEdges returns edges without repeats of the same edge
func uniqueEdges(edges []*ledge) []*ledge {
	seen := make(map[*ledge]bool, len(edges))
	return slices.DeleteFunc(slices.Clone(edges), func(edge *ledge) bool {
		repeat := seen[edge]
		seen[edge] = true
		return repeat
	})
}

// components splits the graph into connected components, each a Graph
// sharing the original nodes, edges and settings. Components are laid out
// independently and then placed side by side.
func components(graphdef *lgraph) []*lgraph {
	index := make(map[*lnode]int, len(graphdef.Nodes))
	for i, node := range graphdef.Nodes {
		index[node] = i
	}
	parent := make([]int, len(graphdef.Nodes))
	for i := range parent {
		parent[i] = i
	}
	findIndex := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]] // path halving
			i = parent[i]
		}
		return i
	}
	find := func(n *lnode) *lnode { return graphdef.Nodes[findIndex(index[n])] }
	union := func(a, b *lnode) { parent[findIndex(index[a])] = findIndex(index[b]) }
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
	for _, pinned := range [][]*lnode{graphdef.MinRank, graphdef.MaxRank} {
		for _, node := range pinned {
			union(pinned[0], node)
		}
	}

	byRoot := map[*lnode]*lgraph{}
	var result []*lgraph
	sub := func(node *lnode) *lgraph {
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

// hierComponent is the state shared by the stages that lay out one
// connected graph, see hierarchicalComponent
type hierComponent struct {
	graphdef *lgraph
	opts     Options
	graph    *hier.Graph
	nodes    map[*lnode]hier.ID
	reverse  map[hier.ID]*lnode

	// nodes with self-loops need room on their right for the loop and
	// its label
	loopWidth, loopHeight Length
	loopExtra             map[*lnode]Length
	loopLeft              map[*lnode]Length // loops with ports may also pass the left side
	// with packed edge ends, the first end of a node is where its
	// edges line up, see packed
	pack bool

	labels    map[[2]hier.ID][]*ledge // labeled edges by unordered node pair
	clusters  map[*lcluster]*hier.Cluster
	labelNode map[*hier.Node][]*ledge // the labels the middle virtual node of an edge carries
	flatBelow map[*lnode]Length       // room below a node for the labels of edges along its rank
	align     hier.Align
}

// hierarchicalComponent lays out one connected graph top to bottom,
// placing it to the right of whatever the previous components occupy.
func hierarchicalComponent(graphdef *lgraph, opts Options) {
	c := newHierComponent(graphdef, opts)
	c.build()
	c.rank()
	c.addClusters()
	c.fieldPorts()
	c.order()
	c.reserveEnds()
	c.reserveFlatLabels()
	c.sizeNodes()
	c.reserveClusterLabels()
	c.position()
	c.finish()
	if graphdef.ForText && graphdef.Splines == SplinesOrtho && c.align == hier.Balanced {
		straightenNodes(c.graph, graphdef, c.finish)
	}
}

// newHierComponent returns the state for laying out graphdef, with the
// room that self-loops need
func newHierComponent(graphdef *lgraph, opts Options) *hierComponent {
	c := &hierComponent{
		graphdef:  graphdef,
		opts:      opts,
		nodes:     map[*lnode]hier.ID{},
		reverse:   map[hier.ID]*lnode{},
		loopExtra: map[*lnode]Length{},
		loopLeft:  map[*lnode]Length{},
		pack:      graphdef.PackEdgeEnds && graphdef.Splines == SplinesOrtho,
		labels:    map[[2]hier.ID][]*ledge{},
		clusters:  map[*lcluster]*hier.Cluster{},
		labelNode: map[*hier.Node][]*ledge{},
		flatBelow: map[*lnode]Length{},
	}
	c.loopWidth = max(graphdef.NodePadding, 2*graphdef.EdgePadding)
	c.loopHeight = min(c.loopWidth, graphdef.RowPadding)
	for _, edge := range graphdef.Edges {
		if edge.From == edge.To {
			extra := c.loopWidth
			if edge.Label != "" {
				extra += graphdef.EdgePadding + 2*edge.LabelRadius.X
			}
			c.loopExtra[edge.From] = max(c.loopExtra[edge.From], extra)
			if edge.FromPort != CompassAuto || edge.ToPort != CompassAuto {
				c.loopLeft[edge.From] = max(c.loopLeft[edge.From], extra)
			}
		}
	}
	return c
}

// loopShift returns how far left of the center of its widened box a node
// widened for loops sits
func (c *hierComponent) loopShift(node *lnode) Length {
	return -(c.loopExtra[node] - c.loopLeft[node]) / 2
}

// packed returns where the edges of a node line up with packed edge ends,
// this far from its center: an edge padding inside its innermost outline
func (c *hierComponent) packed(node *lnode) Length {
	if !c.pack || node.Shape == PointShape {
		return 0
	}
	return min(0, c.graphdef.EdgePadding+outlines(node)-node.Radius.X)
}

// labelSpan returns the length of the box side along a cluster's label
func (c *hierComponent) labelSpan(cluster *lcluster) Length {
	return 2 * (c.graphdef.textRadius(cluster.Label, "", c.graphdef.FontSize).X + c.graphdef.EdgePadding)
}

// outlines returns the room the extra outlines of a node take
func outlines(node *lnode) Length { return Length(max(node.Peripheries-1, 0)) * peripheryGap }

// unorderedPair returns a and b in increasing order
func unorderedPair(a, b hier.ID) [2]hier.ID {
	if a > b {
		a, b = b, a
	}
	return [2]hier.ID{a, b}
}

// build constructs the hierarchical graph with its rank constraints
func (c *hierComponent) build() {
	c.graph = &hier.Graph{}
	for _, nodedef := range c.graphdef.Nodes {
		node := c.graph.AddNode()
		c.nodes[nodedef] = node.ID
		c.reverse[node.ID] = nodedef
		node.Label = nodedef.ID
	}
	for _, edge := range c.graphdef.Edges {
		from, to := c.graph.Nodes[c.nodes[edge.From]], c.graph.Nodes[c.nodes[edge.To]]
		c.graph.AddWeightedEdge(from, to, float32(edge.Weight))
		// parallel edges share one hierarchical edge, the longest wins
		c.graph.SetMinLen(from, to, max(int32(edge.MinLen), c.graph.MinLen(from, to)))
	}

	for _, nodedef := range c.graphdef.MinRank {
		c.graph.MinRank.Append(c.graph.Nodes[c.nodes[nodedef]])
	}
	for _, nodedef := range c.graphdef.MaxRank {
		c.graph.MaxRank.Append(c.graph.Nodes[c.nodes[nodedef]])
	}
	for _, group := range c.graphdef.SameRank {
		var members hier.Nodes
		for _, nodedef := range group {
			members = append(members, c.graph.Nodes[c.nodes[nodedef]])
		}
		c.graph.SameRank = append(c.graph.SameRank, members)
	}
}

// rank removes cycles, assigns nodes to ranks and adds virtual nodes
// along the edges that cross ranks
func (c *hierComponent) rank() {
	// remove cycles
	hier.Decycle(c.graph)

	// assign nodes to ranks
	hier.RankWith(c.graph, !c.opts.NoRankBalance)

	// labeled edges need a virtual node to hang the label on; doubling the
	// ranks guarantees every edge has one in the middle
	for _, edge := range c.graphdef.Edges {
		if edge.Label != "" && edge.From != edge.To {
			key := unorderedPair(c.nodes[edge.From], c.nodes[edge.To])
			c.labels[key] = append(c.labels[key], edge)
		}
	}
	if len(c.labels) > 0 {
		hier.DoubleRanks(c.graph)
	}

	// create virtual nodes
	hier.AddVirtuals(c.graph)
}

// addClusters adds the borders of the clusters
func (c *hierComponent) addClusters() {
	for _, clusterdef := range c.graphdef.Clusters {
		cluster := &hier.Cluster{}
		if clusterdef.Label != "" && !sideways(c.graphdef.RankDir) {
			// room for the label across the top, with padding beside it;
			// sideways the label runs along the ranks, see labelSpan
			// across the borders, which the box keeps half of
			cluster.MinWidth = float32(c.labelSpan(clusterdef) + c.graphdef.EdgePadding)
		}
		for _, nodedef := range clusterdef.Nodes {
			cluster.Members.Append(c.graph.Nodes[c.nodes[nodedef]])
		}
		c.clusters[clusterdef] = cluster
		c.graph.Clusters = append(c.graph.Clusters, cluster)
	}
	for clusterdef, cluster := range c.clusters {
		cluster.Parent = c.clusters[clusterdef.Parent]
	}
	hier.AddClusterBorders(c.graph)
}

// order orders the nodes in ranks, finds the middle virtual node of every
// labeled edge, which carries the labels, and merges edges
func (c *hierComponent) order() {
	hier.OrderRanksN(c.graph, c.opts.OrderIterations)

	// the middle virtual node of every labeled edge carries the labels
	for _, source := range c.graph.Nodes {
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
			if edges := c.labels[unorderedPair(source.ID, target.ID)]; len(edges) > 0 {
				c.labelNode[chain[len(chain)/2]] = edges
			}
		}
	}

	if c.graphdef.MergeEdges && c.graphdef.Splines == SplinesOrtho {
		mergeEdges(c.graphdef.merged, c.graphdef.Edges, func(node *lnode) int { return c.graph.Nodes[c.nodes[node]].Rank })
	}
}

// reserveEnds widens nodes for packed edge ends, which need room on each
// side of a node for its ends an edge padding apart, from an edge padding
// in; for text, ends spread across a node need a cell each
func (c *hierComponent) reserveEnds() {
	if c.pack || c.graphdef.ForText {
		step := c.graphdef.EdgePadding / 2
		if !c.pack {
			step = c.graphdef.cellWidth() / 2
		}
		ends := map[*lnode][2]int{} // above and below, by rank
		// merged ends take one end a group
		type slot struct {
			node  *lnode
			below bool
			group int
		}
		counted := map[slot]bool{}
		count := func(ends map[*lnode][2]int, node *lnode, below bool, group int) {
			k := slot{node, below, group}
			if group != 0 && counted[k] {
				return
			}
			counted[k] = true
			e := ends[node]
			e[map[bool]int{false: 0, true: 1}[below]]++
			ends[node] = e
		}
		// sideways in text, flat edges leave along the top and bottom of
		// the drawing, which are the sides of the frame here; they need a
		// cell each across the frame, as ends between ranks do along it
		flat := map[*lnode][2]int{} // before and after along the rank
		for _, edge := range c.graphdef.Edges {
			from, to := c.graph.Nodes[c.nodes[edge.From]], c.graph.Nodes[c.nodes[edge.To]]
			if from.Rank == to.Rank {
				if c.graphdef.ForText && c.pack && edge.From != edge.To {
					count(flat, edge.From, from.Pos < to.Pos, c.graphdef.merged[edge][0])
					count(flat, edge.To, to.Pos < from.Pos, c.graphdef.merged[edge][1])
				}
				continue // loops and flat edges leave sideways
			}
			below := from.Rank < to.Rank
			if edge.FromPort == CompassAuto {
				count(ends, edge.From, below, c.graphdef.merged[edge][0])
			}
			if edge.ToPort == CompassAuto {
				count(ends, edge.To, !below, c.graphdef.merged[edge][1])
			}
		}
		for node, e := range flat {
			if node.Shape == PointShape {
				continue
			}
			cell := c.graphdef.cellWidth() / 2
			if grow := Length(max(e[0], e[1])+1)*cell - node.Radius.Y; grow > 0 {
				node.Radius.Y += grow
				node.pad.Y += grow
			}
		}
		for node, e := range ends {
			if node.Shape == PointShape {
				continue // edges meet at the dot
			}
			grow := Length(max(e[0], e[1])+1)*step + outlines(node) - node.Radius.X
			if grow <= 0 {
				continue
			}
			pad := Vector{X: grow}
			if node.Shape == Circle || node.Shape == Square {
				pad.Y = grow
			}
			node.Radius = node.Radius.Add(pad)
			node.pad = node.pad.Add(pad)
		}
	}
}

// reserveFlatLabels makes room for the label of an edge between neighbors
// on a rank: between them, on the right of the left one as for loops, and
// beside the edge within the rank, which it may be taller than, see
// placeFlatLabels
func (c *hierComponent) reserveFlatLabels() {
	for _, edge := range c.graphdef.Edges {
		from, to := c.graph.Nodes[c.nodes[edge.From]], c.graph.Nodes[c.nodes[edge.To]]
		if edge.Label == "" || edge.From == edge.To || from.Rank != to.Rank || max(from.Pos-to.Pos, to.Pos-from.Pos) != 1 {
			continue
		}
		left := edge.From
		if to.Pos < from.Pos {
			left = edge.To
		}
		// neighbors are already two node paddings apart
		c.loopExtra[left] += max(0, 2*(edge.LabelRadius.X+c.graphdef.EdgePadding-c.graphdef.NodePadding))
		below := c.graphdef.EdgePadding + 2*edge.LabelRadius.Y
		c.flatBelow[edge.From] = max(c.flatBelow[edge.From], below)
		c.flatBelow[edge.To] = max(c.flatBelow[edge.To], below)
	}
}

// sizeNodes assigns the sizes of the hierarchical nodes
func (c *hierComponent) sizeNodes() {
	defer c.widenParallel()
	for id, node := range c.graph.Nodes {
		if node.Virtual {
			node.Radius.X = float32(c.graphdef.EdgePadding)
			node.Radius.Y = float32(c.graphdef.EdgePadding)
			if edges, ok := c.labelNode[node]; ok {
				var width, height Length
				for _, edge := range edges {
					width = max(width, edge.LabelRadius.X)
					height += edge.LabelRadius.Y
				}
				node.Radius.X += float32(width + c.graphdef.EdgePadding)
				node.Radius.Y = float32(height + c.graphdef.EdgePadding)
				// the edge passes on one side, the labels stack on the
				// other, see labelSide
				node.Anchor = -labelSide(c.graphdef.RankDir) * (node.Radius.X - float32(c.graphdef.EdgePadding))
			}
			continue
		}

		nodedef := c.reverse[hier.ID(id)]
		node.Radius.X = float32(nodedef.Radius.X + c.graphdef.NodePadding)
		// room for loops on the right, and the left with ports: the box
		// grows by both, and the node sits left of the box center by the
		// difference, where its edges line up with the neighbors
		node.Radius.X += float32((c.loopExtra[nodedef] + c.loopLeft[nodedef]) / 2)
		node.Anchor = float32(c.loopShift(nodedef) + c.packed(nodedef))
		if c.pack && nodedef.Shape != PointShape {
			// see the slots of packed ends in orthoEdges
			node.EndRoom = float32(2 * (nodedef.Radius.X - min(c.graphdef.EdgePadding, nodedef.Radius.X)))
		}
		node.Radius.Y = float32(max(nodedef.Radius.Y, c.flatBelow[nodedef]) + c.graphdef.RowPadding)
	}
}

// widenParallel widens the virtual nodes of the edges between the same
// two nodes, which share them, by as far as assignPaths spreads the edges
// apart, so that positioning keeps the other edges clear of them
func (c *hierComponent) widenParallel() {
	pairs := map[[2]hier.ID]int{}
	for _, edge := range c.graphdef.Edges {
		if edge.From != edge.To {
			pairs[unorderedPair(c.nodes[edge.From], c.nodes[edge.To])]++
		}
	}
	for _, source := range c.graph.Nodes {
		if source.Virtual {
			continue
		}
		for _, out := range source.Out {
			target := out
			for target.Virtual && len(target.Out) > 0 {
				target = target.Out[0]
			}
			n := pairs[unorderedPair(source.ID, target.ID)]
			for v := out; n > 1 && v.Virtual && len(v.Out) > 0; v = v.Out[0] {
				v.Radius.X += float32(Length(n-1) * c.graphdef.EdgePadding)
			}
		}
	}
}

// reserveClusterLabels reserves room for the cluster box lines and the
// label strip by making the cluster's first and last border nodes taller
// than their ranks; outer clusters first so nested boxes stack
func (c *hierComponent) reserveClusterLabels() {
	grow := func(borders []*hier.Node, rank int, extra float32) {
		half := float32(0)
		for _, node := range c.graph.ByRank[rank] {
			half = max(half, node.Radius.Y)
		}
		for _, node := range borders {
			node.Radius.Y = half + extra
		}
	}
	// span returns how far the box of a cluster will reach along the
	// ranks: ranks are stacked their half heights apart, and the box spans
	// its members, see the cluster boxes below
	span := func(clusterdef *lcluster, cluster *hier.Cluster) Length {
		half := func(rank int) float32 {
			h := float32(0)
			for _, node := range c.graph.ByRank[rank] {
				h = max(h, node.Radius.Y)
			}
			return h
		}
		center := map[int]float32{cluster.MinRank: 0}
		for rank := cluster.MinRank + 1; rank <= cluster.MaxRank; rank++ {
			center[rank] = center[rank-1] + half(rank-1) + half(rank)
		}
		top, bottom := float32(math.Inf(1)), float32(math.Inf(-1))
		for _, nodedef := range clusterdef.Nodes {
			rank := c.graph.Nodes[c.nodes[nodedef]].Rank
			top = min(top, center[rank]-float32(nodedef.Radius.Y))
			bottom = max(bottom, center[rank]+float32(nodedef.Radius.Y))
		}
		return Length(bottom-top) + c.graphdef.RowPadding
	}
	for _, clusterdef := range c.graphdef.Clusters {
		cluster := c.clusters[clusterdef]
		pad := float32(c.graphdef.RowPadding / 2)
		top, bottom := pad, pad
		switch {
		case clusterdef.Label == "" && c.graphdef.ForText && !sideways(c.graphdef.RankDir):
			// a line under the top, where text draws the arrowheads of
			// edges coming in, as tall as a label's, see placeClusters
			top += float32(2 * c.graphdef.textRadius(" ", "", c.graphdef.FontSize).Y)
		case clusterdef.Label == "":
		case sideways(c.graphdef.RankDir):
			// the label runs along the ranks; make room for the box to
			// reach as far on both sides
			short := float32(max(0, c.labelSpan(clusterdef)-span(clusterdef, cluster))) / 2
			top, bottom = top+short, bottom+short
		default:
			top += float32(2 * c.graphdef.textRadius(clusterdef.Label, "", c.graphdef.FontSize).Y)
		}
		last := len(cluster.Left) - 1
		grow([]*hier.Node{cluster.Left[0], cluster.Right[0]}, cluster.MinRank, top)
		grow([]*hier.Node{cluster.Left[last], cluster.Right[last]}, cluster.MaxRank, bottom)
	}
}

// position positions the nodes in their ranks
func (c *hierComponent) position() {
	c.align = map[Align]hier.Align{AlignBalanced: hier.Balanced, AlignLeft: hier.Left, AlignRight: hier.Right}[c.opts.Align]
	if c.pack && !c.graphdef.MergeEdges {
		// see the slots of packed ends in orthoEdges; merged ends share one
		c.graph.EndGap = float32(c.graphdef.EdgePadding)
	}
	if c.graphdef.ForText && c.graphdef.MergeEdges {
		// a cell between the fans of different nodes, as merged edges
		// center a node's children on it
		c.graph.FamilyGap = float32(c.graphdef.cellWidth())
		if sideways(c.graphdef.RankDir) {
			c.graph.FamilyGap = float32(c.graphdef.LineHeight)
		}
	}
	hier.Position(c.graph, c.graphdef.Splines != SplinesOrtho, c.graphdef.MergeEdges, c.align)
}

// finish places the nodes, clusters, edges and labels where the
// positions in the hierarchical graph put them
func (c *hierComponent) finish() {
	c.placeNodes()
	c.placeClusters()
	byRank := c.realByRank()
	paths := c.rankPaths(byRank)
	c.flatPaths(paths, byRank)
	c.assignPaths(paths)
	c.crossLabelBands()
	c.shapePaths()
	c.placeFlatLabels()
	nudgeLabels(c.graphdef.Edges, c.graphdef.Nodes, c.graphdef.Clusters, c.graphdef.EdgePadding, 2*c.graphdef.RowPadding, nil)
}

// placeNodes assigns final positions, off the center of nodes widened for
// loops
func (c *hierComponent) placeNodes() {
	for nodedef, id := range c.nodes {
		node := c.graph.Nodes[id]
		nodedef.Center.X = Length(node.Center.X) + c.loopShift(nodedef)
		nodedef.Center.Y = Length(node.Center.Y)
	}
}

// placeClusters boxes the clusters: boxes span their borders
// horizontally and their members vertically, with room for the label on
// top; inner boxes are finished first so that outer boxes can enclose
// them. A box side keeps to the inner half of its border, so that the
// boxes of neighboring borders are apart.
func (c *hierComponent) placeClusters() {
	byDepth := slices.Clone(c.graphdef.Clusters)
	slices.SortStableFunc(byDepth, func(a, b *lcluster) int { return b.depth() - a.depth() })
	for _, clusterdef := range byDepth {
		clusterdef.TopLeft = Vector{Length(math.Inf(1)), Length(math.Inf(1))}
		clusterdef.BottomRight = Vector{Length(math.Inf(-1)), Length(math.Inf(-1))}
	}
	for _, clusterdef := range byDepth {
		cluster := c.clusters[clusterdef]
		left, right := Length(math.Inf(1)), Length(math.Inf(-1))
		for i := range cluster.Left {
			left = min(left, Length(cluster.Left[i].Center.X-cluster.Left[i].Radius.X/2))
			right = max(right, Length(cluster.Right[i].Center.X+cluster.Right[i].Radius.X/2))
		}
		top, bottom := Length(math.Inf(1)), Length(math.Inf(-1))
		for _, nodedef := range clusterdef.Nodes {
			top = min(top, nodedef.Top())
			bottom = max(bottom, nodedef.Bottom())
		}
		top -= c.graphdef.RowPadding / 2
		bottom += c.graphdef.RowPadding / 2
		// enclose nested boxes
		left, top = min(left, clusterdef.TopLeft.X), min(top, clusterdef.TopLeft.Y)
		right, bottom = max(right, clusterdef.BottomRight.X), max(bottom, clusterdef.BottomRight.Y)
		switch {
		case clusterdef.Label == "" && c.graphdef.ForText && !sideways(c.graphdef.RankDir):
			// text has no row for an arrowhead into the first node
			// otherwise, as it draws the top on the line above it; the
			// rows line up as under a label
			top -= 2 * c.graphdef.textRadius(" ", "", c.graphdef.FontSize).Y
		case clusterdef.Label == "":
		case sideways(c.graphdef.RankDir):
			if short := c.labelSpan(clusterdef) - (bottom - top); short > 0 {
				top, bottom = top-short/2, bottom+short/2
			}
		default:
			top -= 2 * c.graphdef.textRadius(clusterdef.Label, "", c.graphdef.FontSize).Y
		}
		clusterdef.TopLeft = Vector{left, top}
		clusterdef.BottomRight = Vector{right, bottom}
		if parent := clusterdef.Parent; parent != nil {
			pad := c.graphdef.RowPadding / 2
			parent.TopLeft = Vector{min(parent.TopLeft.X, left), min(parent.TopLeft.Y, top-pad)}
			parent.BottomRight = Vector{max(parent.BottomRight.X, right), max(parent.BottomRight.Y, bottom+pad)}
		}
	}
}

// realByRank returns the real nodes per rank, obstacles for edge routing
func (c *hierComponent) realByRank() [][]*lnode {
	byRank := make([][]*lnode, len(c.graph.ByRank))
	for _, node := range c.graph.Nodes {
		if !node.Virtual {
			byRank[node.Rank] = append(byRank[node.Rank], c.reverse[node.ID])
		}
	}
	return byRank
}

// rankPaths returns the paths of the edges between ranks, along their
// virtual nodes, by the hierarchical nodes they join; labels go beside the
// middle virtual node
func (c *hierComponent) rankPaths(byRank [][]*lnode) map[[2]hier.ID][]Vector {
	obstacles := newObstacles(byRank, c.graphdef.EdgePadding)
	edgePaths := map[[2]hier.ID][]Vector{}
	for _, source := range c.graph.Nodes {
		if source.Virtual {
			continue
		}

		sourcedef := c.reverse[source.ID]
		for _, out := range source.Out {
			// the ends, a point past a flat label and one for every
			// virtual node, sized at once
			n := 3
			for target := out; target.Virtual && len(target.Out) > 0; target = target.Out[0] {
				n++
			}
			path := make([]Vector, 0, n)
			path = append(path, sourcedef.BottomCenter().Add(Vector{X: c.packed(sourcedef)}))
			if below := sourcedef.Center.Y + c.flatBelow[sourcedef]; c.graphdef.Splines != SplinesOrtho && below > path[0].Y {
				// past the label beside the node first, see reserveFlatLabels
				path = append(path, Vector{path[0].X, below})
			}

			target := out
			for target != nil && target.Virtual {
				if len(target.Out) < 1 { // should never happen
					target = nil
					break
				}

				point := Vector{Length(target.Center.X), Length(target.Center.Y)}
				if edges, ok := c.labelNode[target]; ok {
					point.X = Length(target.Center.X + target.Anchor)
					// labels sit right beside the line, stacked vertically
					// around the node's center
					var height Length
					for _, edge := range edges {
						height += 2 * edge.LabelRadius.Y
					}
					y := point.Y - height/2
					side := Length(labelSide(c.graphdef.RankDir))
					for _, edge := range edges {
						edge.LabelPos = Vector{X: point.X + side*(c.graphdef.EdgePadding+edge.LabelRadius.X), Y: y + edge.LabelRadius.Y}
						y += 2 * edge.LabelRadius.Y
					}
				}
				path = append(path, point)

				target = target.Out[0]
			}
			if target == nil {
				continue
			}

			targetdef := c.reverse[target.ID]
			path = append(path, targetdef.TopCenter().Add(Vector{X: c.packed(targetdef)}))

			// clip ends to node outlines so fan-ins don't converge on one
			// point; packed ends stay at the first end, where they line up
			if !c.pack {
				path[0] = sourcedef.Boundary(path[1])
				path[len(path)-1] = targetdef.Boundary(path[len(path)-2])
			}

			if c.graphdef.Splines != SplinesOrtho {
				// orthogonal edges run on virtual node columns and rank
				// channels, which are free of nodes by construction
				path = routeAround(path, obstacles, sourcedef, targetdef, c.graphdef.EdgePadding)
				path = routeAroundClusters(path, c.graphdef.Clusters, sourcedef, targetdef, c.graphdef.EdgePadding)
			}

			edgePaths[[2]hier.ID{source.ID, target.ID}] = path
		}
	}
	return edgePaths
}

// flatPaths adds the paths of flat edges, which run sideways along the
// rank, arcing over nodes in between; overlapping arcs stack, narrower
// ones below
func (c *hierComponent) flatPaths(edgePaths map[[2]hier.ID][]Vector, byRank [][]*lnode) {
	type arc struct {
		flat   [2]*hier.Node
		lo, hi Length
		top    Length
		level  int
	}
	var arcs []*arc
	for _, flat := range c.graph.Flat {
		sourcedef, targetdef := c.reverse[flat[0].ID], c.reverse[flat[1].ID]
		if max(flat[1].Pos-flat[0].Pos, flat[0].Pos-flat[1].Pos) <= 1 {
			path := []Vector{sourcedef.Boundary(targetdef.Center), targetdef.Boundary(sourcedef.Center)}
			edgePaths[[2]hier.ID{flat[0].ID, flat[1].ID}] = path
			continue
		}
		a := &arc{flat: flat, top: min(sourcedef.Top(), targetdef.Top())}
		a.lo, a.hi = min(sourcedef.Center.X, targetdef.Center.X), max(sourcedef.Center.X, targetdef.Center.X)
		for _, node := range byRank[flat[0].Rank] {
			if node.Center.X > a.lo && node.Center.X < a.hi {
				a.top = min(a.top, node.Top())
			}
		}
		arcs = append(arcs, a)
	}
	slices.SortStableFunc(arcs, func(a, b *arc) int { return cmp.Compare(a.hi-a.lo, b.hi-b.lo) })
	for i, a := range arcs {
		for _, below := range arcs[:i] {
			if below.flat[0].Rank == a.flat[0].Rank && below.lo <= a.hi && a.lo <= below.hi {
				a.level = max(a.level, below.level+1)
				a.top = min(a.top, below.top)
			}
		}
		sourcedef, targetdef := c.reverse[a.flat[0].ID], c.reverse[a.flat[1].ID]
		y := a.top - 2*c.graphdef.EdgePadding*Length(a.level+1)
		edgePaths[[2]hier.ID{a.flat[0].ID, a.flat[1].ID}] = []Vector{
			sourcedef.TopCenter(),
			{sourcedef.Center.X, y},
			{targetdef.Center.X, y},
			targetdef.TopCenter(),
		}
	}
}

// assignPaths gives every edge its path, self-loops their own; edges
// between the same pair of nodes share one route, so they are spread out
func (c *hierComponent) assignPaths(edgePaths map[[2]hier.ID][]Vector) {
	pairKey := func(edge *ledge) [2]hier.ID { return unorderedPair(c.nodes[edge.From], c.nodes[edge.To]) }
	pairCount := map[[2]hier.ID]int{}
	for _, edge := range c.graphdef.Edges {
		pairCount[pairKey(edge)]++
	}
	pairIndex := map[[2]hier.ID]int{}
	loops := countLoops(c.graphdef.Edges)

	for _, edge := range c.graphdef.Edges {
		sourceid := c.nodes[edge.From]
		targetid := c.nodes[edge.To]

		if sourceid == targetid {
			edge.Path = loopPath(edge, c.loopWidth, c.loopHeight, loops.next(edge), loops.count[edge.From])
			// beside the far side of the loop
			loopMid := (edge.Path[0].Y + edge.Path[len(edge.Path)-1].Y) / 2
			edge.LabelPos = Vector{X: edge.From.Right() + c.loopWidth + c.graphdef.EdgePadding + edge.LabelRadius.X, Y: loopMid}
			if edge.FromPort != CompassAuto || edge.ToPort != CompassAuto {
				// ported loops can sit on any side; label the middle of the loop
				p, q := edge.Path[(len(edge.Path)-1)/2], edge.Path[len(edge.Path)/2]
				mid := Vector{X: (p.X + q.X) / 2, Y: (p.Y + q.Y) / 2}
				d := mid.Sub(edge.From.Center)
				if n := Length(math.Hypot(float64(d.X), float64(d.Y))); n > 0 {
					d = Vector{X: d.X / n, Y: d.Y / n}
					gap := c.graphdef.EdgePadding + Length(math.Abs(float64(d.X)))*edge.LabelRadius.X + Length(math.Abs(float64(d.Y)))*edge.LabelRadius.Y
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
			spacing := 2 * c.graphdef.EdgePadding
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
		if edge.FromField != "" {
			path[0] = c.graphdef.fieldEnd(edge.From, edge.FromField, path[1])
		}
		if edge.ToField != "" {
			path[len(path)-1] = c.graphdef.fieldEnd(edge.To, edge.ToField, path[len(path)-2])
		}
		edge.Path = path
	}
}

// crossLabelBands makes an edge cutting across a label on its way past it
// in the label's rank run straight across the band of the labels there
// instead
func (c *hierComponent) crossLabelBands() {
	if c.graphdef.Splines == SplinesOrtho {
		return
	}
	band := map[Length]Length{} // half heights by the y of their rank
	for node := range c.labelNode {
		y := Length(node.Center.Y)
		band[y] = max(band[y], Length(node.Radius.Y))
	}
	pad := c.graphdef.EdgePadding / 2
	cuts := func(edge *ledge, a, b Vector) bool {
		for _, other := range c.graphdef.Edges {
			if other != edge && other.Label != "" {
				tl := other.LabelPos.Sub(other.LabelRadius).Sub(Vector{X: pad, Y: pad})
				br := other.LabelPos.Add(other.LabelRadius).Add(Vector{X: pad, Y: pad})
				if segmentHitsRect(a, b, tl, br) {
					return true
				}
			}
		}
		return false
	}
	for _, edge := range c.graphdef.Edges {
		for i := len(edge.Path) - 2; i >= 1; i-- {
			p := edge.Path[i]
			h, ok := band[p.Y]
			if ok && (cuts(edge, edge.Path[i-1], p) || cuts(edge, p, edge.Path[i+1])) {
				if edge.Path[i-1].Y > p.Y {
					h = -h // reversed edges run up
				}
				edge.Path = slices.Replace(edge.Path, i, i+1, Vector{X: p.X, Y: p.Y - h}, Vector{X: p.X, Y: p.Y + h})
			}
		}
	}

}

// shapePaths routes the paths as the splines of the graph ask for
func (c *hierComponent) shapePaths() {
	if c.graphdef.Splines == SplinesOrtho {
		// row extents per rank: real node boxes, virtual nodes are flat
		rows := make([][2]Length, len(c.graph.ByRank))
		for r, layer := range c.graph.ByRank {
			rows[r] = [2]Length{Length(math.Inf(1)), Length(math.Inf(-1))}
			for _, node := range layer {
				top, bottom := Length(node.Center.Y), Length(node.Center.Y)
				if !node.Virtual {
					top, bottom = c.reverse[node.ID].Top(), c.reverse[node.ID].Bottom()
				}
				rows[r][0], rows[r][1] = min(rows[r][0], top), max(rows[r][1], bottom)
			}
		}
		orthoEdges(c.graphdef, rows, c.graphdef.EdgePadding, c.pack)
	}
	if c.graphdef.Splines != SplinesOrtho {
		spreadWaypoints(c.graphdef.Edges, c.graphdef.EdgePadding)
		spreadEnds(c.graphdef, 9*Point)
	}
	if c.graphdef.Splines == SplinesLine {
		for _, edge := range c.graphdef.Edges {
			if edge.From != edge.To && len(edge.Path) > 2 {
				from, to := edge.Path[0], edge.Path[len(edge.Path)-1]
				if edge.freeStart() {
					from = edge.From.Boundary(edge.To.Center)
				}
				if edge.freeEnd() {
					to = edge.To.Boundary(edge.From.Center)
				}
				edge.Path = []Vector{from, to}
			}
		}
	}
}

// placeFlatLabels places the labels of edges along a rank, which have no
// node of their own: above the middle of the topmost segment, outside an
// arc, or between neighbors in the room made for them: above the edge top
// to bottom, clear of the borders of the nodes, and below in the frame
// otherwise, which is right of the edge sideways
func (c *hierComponent) placeFlatLabels() {
	flat := Length(1)
	if c.graphdef.RankDir == TopToBottom {
		flat = -1
	}
	for _, edge := range c.graphdef.Edges {
		from, to := c.graph.Nodes[c.nodes[edge.From]], c.graph.Nodes[c.nodes[edge.To]]
		if edge.Label == "" || edge.From == edge.To || from.Rank != to.Rank || len(edge.Path) < 2 {
			continue
		}
		if max(from.Pos-to.Pos, to.Pos-from.Pos) == 1 {
			a, b := edge.Path[0], edge.Path[len(edge.Path)-1]
			y := max(a.Y, b.Y)
			if flat < 0 {
				y = min(a.Y, b.Y)
			}
			edge.LabelPos = Vector{X: (a.X + b.X) / 2, Y: y + flat*(c.graphdef.EdgePadding+edge.LabelRadius.Y)}
			continue
		}
		top := 0
		for i := 1; i+1 < len(edge.Path); i++ {
			if edge.Path[i].Y+edge.Path[i+1].Y < edge.Path[top].Y+edge.Path[top+1].Y {
				top = i
			}
		}
		a, b := edge.Path[top], edge.Path[top+1]
		edge.LabelPos = Vector{X: (a.X + b.X) / 2, Y: min(a.Y, b.Y) - c.graphdef.EdgePadding - edge.LabelRadius.Y}
	}
}
