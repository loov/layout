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
