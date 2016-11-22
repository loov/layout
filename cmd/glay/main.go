package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/kr/pretty"
	"github.com/loov/layout"
)

var _ = pretty.Print

const WorldDynamics = `
	S8 -> 9; S24 -> 25; S24 -> 27; S1 -> 2; S1 -> 10; S35 -> 43; S35 -> 36;
	S30 -> 31; S30 -> 33; 9 -> 42; 9 -> T1; 25 -> T1; 25 -> 26; 27 -> T24;
	2 -> 3 16 17 T1 18; 10 -> 11 14 T1 13 12;
	31 -> T1; 31 -> 32; 33 -> T30; 33 -> 34; 42 -> 4; 26 -> 4; 3 -> 4;
	16 -> 15; 17 -> 19; 18 -> 29; 11 -> 4; 14 -> 15; 37 -> 39 41 38 40;
	13 -> 19; 12 -> 29; 43 -> 38; 43 -> 40; 36 -> 19; 32 -> 23; 34 -> 29;
	39 -> 15; 41 -> 29; 38 -> 4; 40 -> 19; 4 -> 5; 19 -> 21 20 28;
	5 -> 6 T35 23; 21 -> 22; 20 -> 15; 28 -> 29; 6 -> 7; 15 -> T1;
	22 -> T35; 22 -> 23; 29 -> T30; 7 -> T8;
	23 -> T24; 23 -> T1;
`

func parse(graph string, onedge func(src, dst string)) {
	for _, edge := range strings.Split(graph, ";") {
		edge = strings.TrimSpace(edge)
		tokens := strings.Split(edge, "->")
		if len(tokens) < 2 {
			continue
		}
		src, dsts := strings.TrimSpace(tokens[0]), strings.TrimSpace(tokens[1])
		for _, dst := range strings.Split(dsts, " ") {
			dst = strings.TrimSpace(dst)
			onedge(src, dst)
		}
	}
}

func printByRank(graph *layout.Graph, byID map[layout.NodeID]string) {
	for rank, nodes := range graph.ByRank {
		fmt.Println("- RANK ", rank, "-")
		for _, sid := range nodes {
			src := graph.Nodes[sid]
			id := src.ID
			if src.Virtual {
				id = -id
			}
			fmt.Printf("%3v['%3v']: %v\n", id, byID[src.ID], src.Out)
		}
	}
	pretty.Println(graph.ByRank)
}

func process(graphdef string) {
	graph := layout.NewGraph()
	byName := map[string]layout.NodeID{}
	byID := map[layout.NodeID]string{}

	node := func(name string) layout.NodeID {
		id, ok := byName[name]
		if !ok {
			id, _ = graph.Node()
			byName[name] = id
			byID[id] = name
		}
		return id
	}

	parse(graphdef, func(src, dst string) {
		sid, did := node(src), node(dst)
		graph.Edge(sid, did)
	})

	layout.Decycle(graph)
	layout.Rank(graph)

	//printByRank(graph, byID)
	fmt.Println("ACTUAL BY RANK")
	for _, rank := range graph.ByRank {
		fmt.Println(len(rank))
	}

	layout.CreateVirtualVertices(graph)

	if err := layout.VerifyProperDigraph(graph); err != nil {
		panic(err)
	}

	layout.OrderRanks(graph)
	layout.Position(graph)

	printByRank(graph, byID)

	file, err := os.Create("~world.svg")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	_, err = graph.WriteSVG(file)
	if err != nil {
		panic(err)
	}

}

func main() {
	flag.Parse()
	if flag.Arg(0) == "" {
		process(WorldDynamics)
	} else {
		src, _ := ioutil.ReadFile(flag.Arg(0))
		process(string(src))
	}
}
