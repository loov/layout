// Command glay lays out a graph and writes it as an image.
//
// Usage:
//
//	glay [-s dot] [-t svg|dot|json|txt] [-o output] [-g name] [-q fast|quality] [-l hierarchical|force] [input]
//
// The input format is detected from the file extension when -s is not set;
// input "-" or no input reads stdin (dot unless -s is set). Files with
// several graphs need -g to pick one by name or index.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"

	"github.com/loov/layout"
	"github.com/loov/layout/format/dot"
	"github.com/loov/layout/format/graphml"
	"github.com/loov/layout/format/json"
	"github.com/loov/layout/format/svg"
	"github.com/loov/layout/format/text"
)

var (
	cpuprofile = flag.String("cpuprofile", "", "profile cpu usage")
	memprofile = flag.String("memprofile", "", "profile memory usage")

	informat  = flag.String("s", "", "input format")
	outformat = flag.String("t", "svg", "output format")
	outfile   = flag.String("o", "", "output file (default stdout)")
	pick      = flag.String("g", "", "graph to lay out when the input has several, by name or index")
	quality   = flag.String("q", "", "layout preset: fast, quality (default balanced)")
	algorithm = flag.String("l", "hierarchical", "layout algorithm: hierarchical, force")

	verbose = flag.Bool("v", false, "verbose output")
)

func infof(format string, args ...any) {
	if *verbose {
		fmt.Fprintf(os.Stderr, format, args...)
		if !strings.HasSuffix("\n", format) {
			fmt.Fprint(os.Stderr, "\n")
		}
	}
}

func errorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	if !strings.HasSuffix("\n", format) {
		fmt.Fprint(os.Stderr, "\n")
	}
}

func main() {
	flag.Parse()

	input := flag.Arg(0)
	output := *outfile
	if output == "" {
		output = flag.Arg(1)
	}
	if input == "" || input == "-" {
		input = "-"
		if *informat == "" {
			*informat = "dot"
		}
	}

	if *informat == "" {
		// try to detect input format
		switch strings.ToLower(filepath.Ext(input)) {
		case ".dot":
			*informat = "dot"
		case ".gv":
			*informat = "dot"
		case ".graphml":
			*informat = "graphml"
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
		case ".dot", ".gv":
			*outformat = "dot"
		case ".json":
			*outformat = "json"
		case ".txt":
			*outformat = "txt"
		default:
			*outformat = "svg"
		}
	}

	if *informat == "" || *outformat == "" {
		errorf("unable to detect input or output format")
		flag.Usage()
		os.Exit(1)
		return
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			errorf("unable to create cpu-profile %q: %v", *cpuprofile, err)
			os.Exit(1)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			errorf("unable to start cpu-profile: %v", err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

	if *memprofile != "" {
		defer func() {
			f, err := os.Create(*memprofile)
			if err != nil {
				errorf("unable to create mem-profile %q: %v", *memprofile, err)
				os.Exit(1)
			}
			defer f.Close()

			runtime.GC()
			if err := pprof.WriteHeapProfile(f); err != nil {
				errorf("unable to start mem-profile: %v", err)
				os.Exit(1)
			}
		}()
	}

	var graphs []*layout.Graph
	var err error

	infof("parsing %q", input)

	in := io.Reader(os.Stdin)
	if input != "-" {
		file, err := os.Open(input)
		if err != nil {
			errorf("unable to open %q: %v", input, err)
			os.Exit(1)
		}
		defer file.Close()
		in = file
	}
	switch *informat {
	case "dot":
		graphs, err = dot.Parse(in)
	case "graphml":
		graphs, err = graphml.Parse(in)
	default:
		errorf("unknown input format %q", *informat)
		flag.Usage()
		os.Exit(1)
		return
	}

	if err != nil || len(graphs) == 0 {
		if len(graphs) == 0 && err == nil {
			err = errors.New("file doesn't contain graphs")
		}
		errorf("failed to parse %q: %v", input, err)
		os.Exit(1)
		return
	}

	if len(graphs) != 1 {
		infof("parsed %v graphs", len(graphs))
	} else {
		infof("parsed 1 graph")
	}

	graph := graphs[0]
	if *pick != "" {
		graph = nil
		for i, g := range graphs {
			if g.ID == *pick || fmt.Sprint(i) == *pick {
				graph = g
			}
		}
		if graph == nil {
			errorf("no graph %q in %q", *pick, input)
			os.Exit(1)
		}
	} else if len(graphs) > 1 {
		errorf("%q contains %v graphs, pick one with -g", input, len(graphs))
		os.Exit(1)
	}

	// layout
	if *outformat == "txt" || *outformat == "text" {
		text.Prepare(graph)
	}
	var opts layout.Options
	switch *quality {
	case "":
	case "fast":
		opts = layout.Fast
	case "quality":
		opts = layout.Quality
	default:
		errorf("unknown preset %q", *quality)
		os.Exit(1)
	}
	switch *algorithm {
	case "hierarchical":
		err = layout.HierarchicalWith(graph, opts)
	case "force":
		err = layout.Force(graph)
	default:
		err = fmt.Errorf("unknown algorithm %q", *algorithm)
	}
	if err != nil {
		errorf("layout failed: %v", err)
		os.Exit(1)
		return
	}

	// output
	var out io.Writer
	if output == "" {
		out = os.Stdout
	} else {
		file, err := os.Create(output)
		if err != nil {
			errorf("unable to create file %q: %v", output, err)
			os.Exit(1)
			return
		}
		defer file.Close()
		out = file
	}

	switch *outformat {
	case "svg":
		err = svg.Write(out, graph)
	case "dot":
		err = dot.Write(out, graph)
	case "json":
		err = json.Write(out, graph)
	case "txt", "text":
		err = text.Write(out, graph)
	default:
		errorf("unknown output format %q", *outformat)
		os.Exit(1)
		return
	}

	if err != nil {
		errorf("writing %q failed: %v", output, err)
		os.Exit(1)
		return
	}
}
