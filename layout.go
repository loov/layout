package layout

import (
	"github.com/loov/layout/internal/hier"
)

const epsilon = 1e-6

func (graph *Graph) AssignMissingValues() {
	for _, node := range graph.Nodes {
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

func Hierarchical(graphdef *Graph) {
	graphdef.AssignMissingValues()

	nodes := map[*Node]hier.ID{}
	reverse := map[hier.ID]*Node{}

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
		graph.AddEdge(graph.Nodes[from], graph.Nodes[to])
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
		nodedef, ok := reverse[hier.ID(id)]
		if !ok {
			node.Radius.X = float32(graphdef.FontSize + graphdef.NodePadding)
			node.Radius.Y = float32(graphdef.FontSize + graphdef.RowPadding)
			continue
		}

		node.Radius.X = float32(nodedef.Radius.X + graphdef.NodePadding)
		node.Radius.Y = float32(nodedef.Radius.Y + graphdef.RowPadding)
	}

	// position nodes
	positionedGraph := hier.DefaultPosition(orderedGraph)

	// assign final positions
	for nodedef, id := range nodes {
		node := positionedGraph.Nodes[id]
		nodedef.Center.X = Length(node.Center.X)
		nodedef.Center.Y = Length(node.Center.Y)
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

			edgePaths[[2]hier.ID{source.ID, target.ID}] = path
		}
	}

	for _, edge := range graphdef.Edges {
		sourceid := nodes[edge.From]
		targetid := nodes[edge.To]

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

		// ??? shouldn't happen
	}
}

func reversePath(path []Vector) []Vector {
	rs := make([]Vector, 0, len(path))
	for i := len(path) - 1; i >= 0; i-- {
		rs = append(rs, path[i])
	}
	return rs
}
