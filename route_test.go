package layout

import "testing"

func TestClearCornersAroundNode(t *testing.T) {
	const pad, radius = 6, 32
	node := &lnode{Center: Vector{100, 100}, Radius: Vector{40, 20}}
	// a detour on the padded corner, whose rounding cuts toward the node
	edge := &ledge{Path: Path{{170, 140}, {146, 74}, {40, 20}}}
	near := func() bool {
		flat := flattenPath(edge.Path, radius, pad)
		for i := 0; i+1 < len(flat); i++ {
			if segmentHitsNode(flat[i], flat[i+1], node, pad) {
				return true
			}
		}
		return false
	}
	if !near() {
		t.Fatal("the rounded corner does not come near the node")
	}
	clearCorners([]*ledge{edge}, []*lnode{node}, radius, pad)
	if near() {
		t.Errorf("rounded corner at %v still near the node", edge.Path[1])
	}
}
