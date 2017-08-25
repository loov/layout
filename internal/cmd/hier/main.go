package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/internal/hier"
)

var (
	informat  = flag.String("S", "", "input format")
	outformat = flag.String("T", "svg", "output format")

	verbose     = flag.Bool("v", false, "verbose output")
	veryVerbose = flag.Bool("vv", false, "very-verbose output")
)

func info(format string, args ...interface{}) {
	if *verbose {
		fmt.Fprintf(os.Stderr, format, args...)
		if !strings.HasSuffix("\n", format) {
			fmt.Fprint(os.Stderr, "\n")
		}
	}
}

func main() {
	flag.Parse()

	input := flag.Arg(0)
	output := flag.Arg(1)

	if input == "" {
		info("input is missing")
		flag.Usage()
		return
	}

	*verbose = *verbose || *veryVerbose

	if *informat == "" {
		// try to detect input format
		switch strings.ToLower(filepath.Ext(input)) {
		case ".dot":
			*informat = "dot"
		case ".gv":
			*informat = "dot"
		}
	}

	if output != "" {
		*outformat = ""
	}
	if *outformat == "" {
		// try to detect output format
		switch strings.ToLower(filepath.Ext(output)) {
		case ".svg":
			*outformat = "svg"
		default:
			*outformat = "svg"
		}
	}

	if *informat == "" || *outformat == "" {
		info("unable to detect input or output format")
		flag.Usage()
		os.Exit(1)
		return
	}

	var graphs []*layout.Graph
	var err error

	info("parsing %q", input)

	switch *informat {
	case "dot":
		graphs, err = dot.ParseFile(input)
	default:
		info("unknown input format %q", *informat)
		flag.Usage()
		os.Exit(1)
		return
	}

	if err != nil || len(graphs) == 0 {
		if len(graphs) == 0 && err == nil {
			err = errors.New("file doesn't contain graphs")
		}
		info("failed to parse %q: %v", input, err)
		os.Exit(1)
		return
	}

	if len(graphs) != 1 {
		info("parsed %v graphs", len(graphs))
	} else {
		info("parsed 1 graph")
	}

	graphdef := graphs[0]
	if len(graphs) > 1 {
		fmt.Fprintf(os.Stderr, "file %q contains multiple graphs, processing only first\n", input)
	}

	var start, stop time.Time

	info("\nCONVERTING")

	nodeID := map[*layout.Node]hier.ID{}

	graph := &hier.Graph{}
	for _, nodedef := range graphdef.Nodes {
		node := graph.AddNode()
		nodeID[nodedef] = node.ID
		node.Label = nodedef.ID
	}
	for _, edge := range graphdef.Edges {
		from, to := nodeID[edge.From], nodeID[edge.To]
		graph.AddEdge(graph.Nodes[from], graph.Nodes[to])
	}

	if *verbose {
		info("  nodes: %-8v roots: %-8v", graph.NodeCount(), graph.CountRoots())
		info("  edges: %-8v links: %-8v", graph.CountEdges(), graph.CountUndirectedLinks())
		info("  cycle: %-8v", graph.IsCyclic())
	}

	info("\nDECYCLING")
	start = time.Now()
	decycle := hier.NewDecycle(graph)
	decycle.Run()
	stop = time.Now()

	if *verbose {
		info("   time: %.3f ms", float64(stop.Sub(start).Nanoseconds())/1e6)
		info("  nodes: %-8v roots: %-8v", graph.NodeCount(), graph.CountRoots())
		info("  edges: %-8v links: %-8v", graph.CountEdges(), graph.CountUndirectedLinks())
	}

	info("\nRANKING")
	start = time.Now()
	hier.Rank(graph)
	stop = time.Now()
	if *verbose {
		info("   time: %.3f ms", float64(stop.Sub(start).Nanoseconds())/1e6)
		info("  ranks: %-8v   avg: %-8.2f   var: %-8.2f", len(graph.ByRank), rankWidthAverage(graph), rankWidthVariance(graph))
		if *veryVerbose {
			for i, rank := range graph.ByRank {
				info("   %4d-  count: %-2d      %v", i, len(rank), rank)
			}
		}
	}

	info("\nADDING VIRTUALS")
	start = time.Now()
	hier.AddVirtualVertices(graph)
	stop = time.Now()
	if *verbose {
		info("   time: %.3f ms", float64(stop.Sub(start).Nanoseconds())/1e6)
		info("  nodes: %-8v roots: %-8v", graph.NodeCount(), graph.CountRoots())
		info("  edges: %-8v links: %-8v", graph.CountEdges(), graph.CountUndirectedLinks())
		// TODO: add info about crossings
		info("  ranks: %-8v   avg: %-8.2f   var: %-8.2f", len(graph.ByRank), rankWidthAverage(graph), rankWidthVariance(graph))
		if *veryVerbose {
			for i, rank := range graph.ByRank {
				info("   %4d-  count: %-2d      %v", i, len(rank), rank)
			}
		}
	}

	info("\nORDERING")
	start = time.Now()
	hier.OrderRanks(graph)
	stop = time.Now()
	if *verbose {
		info("   time: %.3f ms", float64(stop.Sub(start).Nanoseconds())/1e6)
		// TODO: add info about crossings
		if *veryVerbose {
			for i, rank := range graph.ByRank {
				info("   %4d-  count: %-2d      %v", i, len(rank), rank)
			}
		}
	}

	// TODO: add step about initial positions
	// TODO: add average, max, total edge length

	info("\nPOSITIONING")
	start = time.Now()
	hier.Position(graph)
	stop = time.Now()
	if *verbose {
		info("   time: %.3f ms", float64(stop.Sub(start).Nanoseconds())/1e6)
		// TODO: add average, max, total edge length
	}

	info("\nOUTPUTTING")

	start = time.Now()

	var out io.Writer
	if output == "" {
		out = os.Stdout
	} else {
		file, err := os.Create(output)
		if err != nil {
			info("unable to create file %q: %v", output, err)
			os.Exit(1)
			return
		}
		defer file.Close()
		out = file
	}

	if err != nil {
		info("writing %q failed: %v", output, err)
		os.Exit(1)
		return
	}

	stop = time.Now()
	if *verbose {
		info("   time: %.3f ms", float64(stop.Sub(start).Nanoseconds())/1e6)
	}
}

func rankWidthAverage(graph *hier.Graph) float64 {
	return float64(len(graph.Nodes)) / float64(len(graph.ByRank))
}

func rankWidthVariance(graph *hier.Graph) float64 {
	if graph.NodeCount() < 2 {
		return 0
	}

	averageWidth := rankWidthAverage(graph)
	total := float64(0)
	for _, rank := range graph.ByRank {
		deviation := float64(len(rank)) - averageWidth
		total += deviation * deviation
	}

	return math.Sqrt(total / float64(len(graph.ByRank)-1))
}
