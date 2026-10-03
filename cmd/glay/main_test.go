package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvalidOutputFormatPreservesExistingFile(t *testing.T) {
	if os.Getenv("GLAY_TEST_PROCESS") == "1" {
		os.Args = []string{"glay", "-t", "png", "-o", os.Getenv("GLAY_TEST_OUTPUT")}
		main()
		return
	}
	path := filepath.Join(t.TempDir(), "existing.svg")
	const original = "existing drawing"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestInvalidOutputFormatPreservesExistingFile$")
	cmd.Env = append(os.Environ(), "GLAY_TEST_PROCESS=1", "GLAY_TEST_OUTPUT="+path)
	cmd.Stdin = strings.NewReader("digraph {a -> b}")
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "unknown output format") {
		t.Fatalf("error = %v, output = %s", err, output)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatalf("output overwritten: %q", got)
	}
}

// glay runs main in a subprocess with args and the graph on stdin
func glay(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestGlayProcess$")
	cmd.Env = append(os.Environ(), "GLAY_TEST_ARGS="+strings.Join(args, "\n"))
	cmd.Stdin = strings.NewReader("digraph {a -> b}")
	return cmd.CombinedOutput()
}

func TestGlayProcess(t *testing.T) {
	if args := os.Getenv("GLAY_TEST_ARGS"); args != "" {
		os.Args = append([]string{"glay"}, strings.Split(args, "\n")...)
		main()
	}
}

func TestOutputFormatFromExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.graphml")
	if output, err := glay(t, "-o", path); err != nil {
		t.Fatalf("%v: %s", err, output)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "<graphml") {
		t.Errorf("not GraphML:\n%s", got)
	}

	path = filepath.Join(dir, "out.png")
	if output, err := glay(t, "-o", path); err == nil {
		t.Errorf("unknown extension accepted: %s", output)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("output written for an unknown extension")
	}
}

func TestInvalidTextOptionsFailForEveryFormat(t *testing.T) {
	for _, args := range [][]string{{"-bg", "nope", "-t", "txt"}, {"-colors", "8", "-t", "svg"}, {"-bg", "nope", "-t", "ans"}} {
		if output, err := glay(t, args...); err == nil {
			t.Errorf("%v accepted: %s", args, output)
		}
	}
}
