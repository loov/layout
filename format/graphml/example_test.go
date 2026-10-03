package graphml_test

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/loov/layout"
	"github.com/loov/layout/format/graphml"
)

const document = `<?xml version="1.0" encoding="UTF-8"?>
<graphml xmlns="http://graphml.graphdrawing.org/xmlns">
	<key id="d0" for="node" attr.name="label" attr.type="string"/>
	<graph id="G" edgedefault="directed">
		<node id="a"><data key="d0">Start</data></node>
		<node id="b"/>
		<edge source="a" target="b"/>
	</graph>
</graphml>`

func ExampleParse() {
	graphs, err := graphml.Parse(strings.NewReader(document))
	if err != nil {
		log.Fatal(err)
	}
	graph := graphs[0]
	fmt.Println(graph.ID, graph.Directed)
	fmt.Println(graph.Node("a").Label, len(graph.Edges))
	// Output:
	// G true
	// Start 1
}

func ExampleParseString() {
	graphs, err := graphml.ParseString(document)
	if err != nil {
		log.Fatal(err)
	}
	l, err := layout.Hierarchical(graphs[0], layout.Options{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(l.Node(graphs[0].Node("b")).Center)
	// Output:
	// {38.96 96}
}

func ExampleParseFile() {
	graphs, err := graphml.ParseFile("graph.graphml")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := layout.Hierarchical(graphs[0], layout.Options{}); err != nil {
		log.Fatal(err)
	}
}

// Write puts all the graphs in one document.
func ExampleWrite() {
	graph := layout.NewDigraph()
	graph.ID = "G"
	graph.Edge("a", "b")
	if err := graphml.Write(os.Stdout, graph); err != nil {
		log.Fatal(err)
	}
	// Output:
	// <graphml xmlns="http://graphml.graphdrawing.org/xmlns" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:y="http://www.yworks.com/xml/graphml" xsi:schemaLocation="http://graphml.graphdrawing.org/xmlns http://graphml.graphdrawing.org/xmlns/1.0/graphml.xsd">
	// 	<key id="d0" for="node" attr.name="label" attr.type="string"></key>
	// 	<key id="d1" for="node" attr.name="shape" attr.type="string"></key>
	// 	<key id="d2" for="node" attr.name="tooltip" attr.type="string"></key>
	// 	<key id="d3" for="edge" attr.name="label" attr.type="string"></key>
	// 	<key id="d4" for="edge" attr.name="tooltip" attr.type="string"></key>
	// 	<key id="d5" for="node" yfiles.type="nodegraphics"></key>
	// 	<key id="d6" for="edge" yfiles.type="edgegraphics"></key>
	// 	<graph id="G" edgedefault="directed">
	// 		<node id="a">
	// 			<data key="d0">a</data>
	// 			<data key="d5"><y:ShapeNode><y:NodeLabel>a</y:NodeLabel></y:ShapeNode></data>
	// 		</node>
	// 		<node id="b">
	// 			<data key="d0">b</data>
	// 			<data key="d5"><y:ShapeNode><y:NodeLabel>b</y:NodeLabel></y:ShapeNode></data>
	// 		</node>
	// 		<edge source="a" target="b"></edge>
	// 	</graph>
	// </graphml>
}

// Convert gives the GraphML elements to adjust before encoding them
// with encoding/xml.
func ExampleConvert() {
	graph := layout.NewDigraph()
	graph.Edge("a", "b")

	converted := graphml.Convert(graph)
	for _, node := range converted.Node {
		fmt.Println(node.ID)
	}
	fmt.Println(converted.EdgeDefault)
	// Output:
	// a
	// b
	// directed
}
