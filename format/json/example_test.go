package json_test

import (
	"fmt"
	"log"
	"os"

	"github.com/loov/layout"
	"github.com/loov/layout/format/json"
)

// Write exports the coordinates for drawing the graph elsewhere.
func ExampleWrite() {
	graph := layout.NewDigraph()
	graph.Edge("a", "b")
	if err := layout.Hierarchical(graph); err != nil {
		log.Fatal(err)
	}
	if err := json.Write(os.Stdout, graph); err != nil {
		log.Fatal(err)
	}
	// Output:
	// {
	//   "directed": true,
	//   "x": 0,
	//   "y": 0,
	//   "width": 64,
	//   "height": 128,
	//   "nodes": [
	//     {
	//       "id": "a",
	//       "x": 32,
	//       "y": 32,
	//       "width": 32,
	//       "height": 32
	//     },
	//     {
	//       "id": "b",
	//       "x": 32,
	//       "y": 96,
	//       "width": 32,
	//       "height": 32
	//     }
	//   ],
	//   "edges": [
	//     {
	//       "from": "a",
	//       "to": "b",
	//       "directed": true,
	//       "path": [
	//         [
	//           32,
	//           48
	//         ],
	//         [
	//           32,
	//           80
	//         ]
	//       ]
	//     }
	//   ]
	// }
}

// Convert gives the same data as Write without encoding it.
func ExampleConvert() {
	graph := layout.NewDigraph()
	graph.Edge("a", "b")
	if err := layout.Hierarchical(graph); err != nil {
		log.Fatal(err)
	}
	converted := json.Convert(graph)
	for _, node := range converted.Nodes {
		fmt.Printf("%s: %vx%v at %v,%v\n", node.ID, node.Width, node.Height, node.X, node.Y)
	}
	// Output:
	// a: 32x32 at 32,32
	// b: 32x32 at 32,96
}
