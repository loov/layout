// Command testdiff shows the changed testdata drawings side by side in a
// browser: old, new and an onion skin of the two.
//
// Usage:
//
//	go run ./internal/cmd/testdiff          # the working tree against HEAD
//	go run ./internal/cmd/testdiff <commit> # a commit against its parent
//	go run ./internal/cmd/testdiff -all     # every drawing, changed or not
//
// It writes the page to a temporary directory and opens it.
package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/loov/layout/internal/draw"
)

//go:embed page.html
var page string

// Change is a testdata drawing in the two versions; Old or New is empty
// when the file was added or removed, and they are the same when it did
// not change, see -all.
type Change struct {
	Path string `json:"path"`
	Kind string `json:"kind"` // svg, txt or ans
	Old  string `json:"old"`
	New  string `json:"new"`
	// OldSize and NewSize are the width and height of the drawings, in
	// columns and rows of text or pixels of svg; nil when missing
	OldSize *[2]float64 `json:"oldSize"`
	NewSize *[2]float64 `json:"newSize"`
}

func main() {
	noOpen := flag.Bool("n", false, "only write the page, don't open it")
	all := flag.Bool("all", false, "show every drawing, also the ones that did not change")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: testdiff [-n] [-all] [commit]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}

	changes, title, err := collect(flag.Arg(0), *all)
	if err != nil {
		fmt.Fprintln(os.Stderr, "testdiff:", err)
		os.Exit(1)
	}
	if len(changes) == 0 {
		if *all {
			fmt.Fprintln(os.Stderr, "testdiff: no testdata drawings in", title)
		} else {
			fmt.Fprintln(os.Stderr, "testdiff: no testdata drawings changed in", title)
		}
		return
	}

	dir, err := os.MkdirTemp("", "testdiff-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testdiff:", err)
		os.Exit(1)
	}
	file := filepath.Join(dir, "index.html")
	if err := write(file, title, changes); err != nil {
		fmt.Fprintln(os.Stderr, "testdiff:", err)
		os.Exit(1)
	}
	fmt.Println(file)
	if !*noOpen {
		if err := open(file); err != nil {
			fmt.Fprintln(os.Stderr, "testdiff:", err)
			os.Exit(1)
		}
	}
}

// collect returns the changed drawings under testdata, of the commit
// against its parent, or of the working tree against HEAD when commit is
// empty, and a title for them; with all, also the ones that did not change
func collect(commit string, all bool) ([]Change, string, error) {
	root, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, "", err
	}
	root = strings.TrimSpace(root)

	base, title := "HEAD", "the working tree"
	args := []string{"diff", "--name-only", "--no-renames", "HEAD"}
	if commit != "" {
		if _, err := git("rev-parse", "--verify", "--quiet", commit+"^{commit}"); err != nil {
			return nil, "", fmt.Errorf("unknown commit %q", commit)
		}
		title, _ = git("log", "-1", "--format=%h %s", commit)
		title = strings.TrimSpace(title)
		base = commit + "^"
		args = []string{"diff", "--name-only", "--no-renames", base, commit}
		if _, err := git("rev-parse", "--verify", "--quiet", base); err != nil {
			// a root commit: everything is added, and there is no old
			base = ""
			args = []string{"diff-tree", "--root", "--no-commit-id", "--name-only", "-r", commit}
		}
	}
	listed, err := git(append(args, "--", "testdata")...)
	if err != nil {
		return nil, "", err
	}
	paths := strings.Fields(listed)
	if commit == "" {
		untracked, err := git("ls-files", "--others", "--exclude-standard", "--", "testdata")
		if err != nil {
			return nil, "", err
		}
		paths = append(paths, strings.Fields(untracked)...)
	}
	if all {
		var tracked string
		if commit == "" {
			tracked, err = git("ls-files", "--", "testdata")
		} else {
			tracked, err = git("ls-tree", "-r", "--name-only", commit, "--", "testdata")
		}
		if err != nil {
			return nil, "", err
		}
		paths = append(paths, strings.Fields(tracked)...)
	}
	slices.Sort(paths)
	paths = slices.Compact(paths)

	var changes []Change
	for _, path := range paths {
		kind := strings.TrimPrefix(filepath.Ext(path), ".")
		if kind != "svg" && kind != "txt" && kind != "ans" || filepath.Base(path) == "diagnostics.txt" {
			continue // diagnostics are no drawing
		}
		old := show(base, path)
		var new string
		if commit != "" {
			new = show(commit, path)
		} else if data, err := os.ReadFile(filepath.Join(root, path)); err == nil {
			new = string(data)
		}
		if all && (old != "" || new != "") || old != new {
			changes = append(changes, Change{
				Path: path, Kind: kind, Old: old, New: new,
				OldSize: size(kind, old), NewSize: size(kind, new),
			})
		}
	}
	return changes, title, nil
}

var (
	// escape matches the escape codes of colored text
	escape = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")
	// svgSize matches the size on the root element of an svg
	svgSize = regexp.MustCompile(`<svg\b[^>]*?\swidth=['"]([0-9.]+)['"][^>]*?\sheight=['"]([0-9.]+)['"]`)
)

// size returns the width and height of a drawing of kind: the columns and
// rows of text, without trailing blank lines, or the pixels of an svg
func size(kind, content string) *[2]float64 {
	if content == "" {
		return nil
	}
	if kind == "svg" {
		m := svgSize.FindStringSubmatch(content)
		if m == nil {
			return nil
		}
		w, _ := strconv.ParseFloat(m[1], 64)
		h, _ := strconv.ParseFloat(m[2], 64)
		return &[2]float64{w, h}
	}
	lines := strings.Split(strings.TrimRight(escape.ReplaceAllString(content, ""), " \n"), "\n")
	w := 0
	for _, line := range lines {
		w = max(w, draw.Columns(strings.TrimRight(line, " ")))
	}
	return &[2]float64{float64(w), float64(len(lines))}
}

// show returns the contents of path at rev, or nothing when it is not there
// or rev is empty
func show(rev, path string) string {
	if rev == "" {
		return ""
	}
	out, err := git("show", rev+":"+path)
	if err != nil {
		return ""
	}
	return out
}

func git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// write writes the page with the changes to file
func write(file, title string, changes []Change) error {
	data, err := json.Marshal(struct {
		Title   string   `json:"title"`
		Changes []Change `json:"changes"`
	}{title, changes})
	if err != nil {
		return err
	}
	// json escapes <, > and &, so the data can't close the script
	html := strings.Replace(page, "/*DATA*/null", string(data), 1)
	if html == page {
		return errors.New("page has no place for the data")
	}
	return os.WriteFile(file, []byte(html), 0o644)
}

// open opens file in the default browser
func open(file string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", file).Run()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", file).Run()
	}
	return exec.Command("xdg-open", file).Run()
}
