package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertInPlace(t *testing.T) {
	if path := os.Getenv("DOT2GRAPHML_TEST_FILE"); path != "" {
		os.Args = []string{"dot2graphml", path, path}
		main()
		return
	}
	path := filepath.Join(t.TempDir(), "graph.dot")
	if err := os.WriteFile(path, []byte("digraph {alpha -> beta}"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestConvertInPlace$")
	cmd.Env = append(os.Environ(), "DOT2GRAPHML_TEST_FILE="+path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, output)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `id="alpha"`) {
		t.Fatalf("input lost:\n%s", got)
	}
}

func TestErrorsAreReported(t *testing.T) {
	if args := os.Getenv("DOT2GRAPHML_TEST_ARGS"); args != "" {
		os.Args = strings.Fields(args)
		main()
		return
	}
	dir := t.TempDir()
	missing, valid := filepath.Join(dir, "missing"), filepath.Join(dir, "valid.dot")
	if err := os.WriteFile(valid, []byte("digraph {a}"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ args, stdin, want string }{
		{"dot2graphml", "digraph {", "failed to parse input: "},
		{"dot2graphml " + missing + ".dot", "", "failed to parse input: "},
		{"dot2graphml " + valid + " " + missing + "/out.graphml", "", "failed to create " + missing + "/out.graphml: "},
	} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestErrorsAreReported$")
		cmd.Env = append(os.Environ(), "DOT2GRAPHML_TEST_ARGS="+tc.args)
		cmd.Stdin = strings.NewReader(tc.stdin)
		output, err := cmd.CombinedOutput()
		if err == nil || !strings.HasPrefix(string(output), tc.want) || !strings.HasSuffix(string(output), "\n") || len(output) == len(tc.want)+1 {
			t.Errorf("%s: error = %v, output = %q", tc.args, err, output)
		}
	}
}
