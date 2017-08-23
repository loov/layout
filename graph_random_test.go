package layout

import "testing"

var BenchmarkGraphSizes = []struct {
	Name               string
	Nodes, Connections int
}{
	{"1x0", 1, 0},
	{"2x1", 2, 1},
	{"4x2", 4, 2},
	{"16x4", 16, 4},
	{"256x4", 256, 4},
	{"256x16", 256, 16},
}

func BenchmarkGenerateRegularGraph(b *testing.B) {
	for _, size := range BenchmarkGraphSizes {
		b.Run(size.Name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = GenerateRegularGraph(size.Nodes, size.Connections)
			}
		})
	}
}

func BenchmarkNewGraphFromEdgeList(b *testing.B) {
	for _, size := range BenchmarkGraphSizes {
		graph := GenerateRegularGraph(size.Nodes, size.Connections)
		edgeList := graph.ConvertToEdgeList()
		b.Run(size.Name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = NewGraphFromEdgeList(edgeList)
			}
		})
	}
}
