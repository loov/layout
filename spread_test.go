package layout

import "testing"

func TestSpreadWaypointsBesideNode(t *testing.T) {
	const pad = 6
	node := &lnode{Center: Vector{100, 100}, Radius: Vector{40, 20}}
	for _, side := range []Length{-1, 1} {
		way := Vector{node.Center.X + side*(node.Radius.X+pad), node.Center.Y}
		var edges []*ledge
		for i := range 3 {
			next := Vector{way.X + side*Length(i)*20, 300}
			edges = append(edges, &ledge{Path: Path{{way.X, 0}, way, next}})
		}
		spreadWaypoints(edges, []*lnode{node}, pad)
		for _, edge := range edges {
			if p := edge.Path[1]; segmentHitsNode(edge.Path[0], p, node, 0) || segmentHitsNode(p, edge.Path[2], node, 0) {
				t.Errorf("side %v: waypoint %v spread into the node", side, p)
			}
		}
	}
}
