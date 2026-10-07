// Command testdiff shows the changed testdata drawings side by side in a
// browser: old, new and an onion skin of the two.
//
// Usage:
//
//	go run ./internal/cmd/testdiff          # the working tree against HEAD
//	go run ./internal/cmd/testdiff <commit> # a commit against its parent
//	go run ./internal/cmd/testdiff <old> <new> # two commits, such as main HEAD
//	go run ./internal/cmd/testdiff <dir> <dir> # the drawings of two directories, by name
//	go run ./internal/cmd/testdiff -all     # every drawing, changed or not
//	go run ./internal/cmd/testdiff -title "option 1" # titled, to tell pages apart
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
	// OldStats and NewStats are the diagnostics of the layout drawn, see
	// layout.Diagnostics, by key, with the edges of text counted as drawn,
	// see textStats; nil when missing
	OldStats map[string]float64 `json:"oldStats"`
	NewStats map[string]float64 `json:"newStats"`
}

func main() {
	noOpen := flag.Bool("n", false, "only write the page, don't open it")
	all := flag.Bool("all", false, "show every drawing, also the ones that did not change")
	name := flag.String("title", "", "title of the page, in place of what it compares")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: testdiff [-n] [-all] [-title title] [commit | old new | olddir newdir]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() > 2 {
		flag.Usage()
		os.Exit(2)
	}

	var changes []Change
	var title string
	var err error
	if args := flag.Args(); len(args) == 2 && isDir(args[0]) && isDir(args[1]) {
		title = args[0] + " → " + args[1]
		changes, err = collectDirs(args[0], args[1], *all)
	} else {
		var base, target string
		base, target, title, err = revisions(args)
		if err == nil {
			changes, err = collect(base, target, *all)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "testdiff:", err)
		os.Exit(1)
	}
	if *name != "" {
		title = *name
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

// revisions returns what the arguments compare, and a title for it: the
// working tree against HEAD, a commit against its parent, or an old commit
// against a new one; an empty base has nothing, the new commit of a root
// commit, and an empty target is the working tree
func revisions(args []string) (base, target, title string, err error) {
	for _, rev := range args {
		if _, err := git("rev-parse", "--verify", "--quiet", rev+"^{commit}"); err != nil {
			return "", "", "", fmt.Errorf("unknown commit %q", rev)
		}
	}
	subject := func(rev string) string {
		out, _ := git("log", "-1", "--format=%h %s", rev)
		return strings.TrimSpace(out)
	}
	switch len(args) {
	case 0:
		return "HEAD", "", "the working tree", nil
	case 1:
		base = args[0] + "^"
		if _, err := git("rev-parse", "--verify", "--quiet", base); err != nil {
			base = "" // a root commit
		}
		return base, args[0], subject(args[0]), nil
	}
	return args[0], args[1], args[0] + " → " + args[1], nil
}

// collect returns the drawings under testdata that differ between base and
// target, see revisions; with all, also the ones that don't
func collect(base, target string, all bool) ([]Change, error) {
	root, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	root = strings.TrimSpace(root)

	var args []string
	switch {
	case target == "":
		args = []string{"diff", "--name-only", "--no-renames", base}
	case base == "":
		args = []string{"diff-tree", "--root", "--no-commit-id", "--name-only", "-r", target}
	default:
		args = []string{"diff", "--name-only", "--no-renames", base, target}
	}
	commit := target
	listed, err := git(append(args, "--", "testdata")...)
	if err != nil {
		return nil, err
	}
	paths := strings.Fields(listed)
	if commit == "" {
		untracked, err := git("ls-files", "--others", "--exclude-standard", "--", "testdata")
		if err != nil {
			return nil, err
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
			return nil, err
		}
		paths = append(paths, strings.Fields(tracked)...)
	}
	slices.Sort(paths)
	paths = slices.Compact(paths)

	// the diagnostics of the layouts, as TestDiagnostics records them
	read := func(path string) string {
		if commit != "" {
			return show(commit, path)
		}
		data, _ := os.ReadFile(filepath.Join(root, path))
		return string(data)
	}
	oldStats := map[string]map[string]map[string]float64{
		"svg": diagnostics(show(base, "testdata/diagnostics.txt")),
		"txt": diagnostics(show(base, "testdata/diagnostics_text.txt")),
	}
	newStats := map[string]map[string]map[string]float64{
		"svg": diagnostics(read("testdata/diagnostics.txt")),
		"txt": diagnostics(read("testdata/diagnostics_text.txt")),
	}

	var changes []Change
	for _, path := range paths {
		if change, ok := compare(path, show(base, path), read(path), oldStats, newStats, all); ok {
			changes = append(changes, change)
		}
	}
	return changes, nil
}

// collectDirs returns the drawings that differ between the directories
// oldDir and newDir, matched by name; with all, also the ones that don't.
// The diagnostics come from each directory, or the one above it, as
// testdata/graphviz has them in testdata.
func collectDirs(oldDir, newDir string, all bool) ([]Change, error) {
	var paths []string
	for _, dir := range []string{oldDir, newDir} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				paths = append(paths, entry.Name())
			}
		}
	}
	slices.Sort(paths)
	paths = slices.Compact(paths)

	read := func(dir, name string) string {
		data, _ := os.ReadFile(filepath.Join(dir, name))
		return string(data)
	}
	stats := func(dir string) map[string]map[string]map[string]float64 {
		find := func(name string) string {
			if content := read(dir, name); content != "" {
				return content
			}
			return read(filepath.Dir(filepath.Clean(dir)), name)
		}
		return map[string]map[string]map[string]float64{
			"svg": diagnostics(find("diagnostics.txt")),
			"txt": diagnostics(find("diagnostics_text.txt")),
		}
	}
	oldStats, newStats := stats(oldDir), stats(newDir)

	var changes []Change
	for _, path := range paths {
		if change, ok := compare(path, read(oldDir, path), read(newDir, path), oldStats, newStats, all); ok {
			changes = append(changes, change)
		}
	}
	return changes, nil
}

// compare returns the change of the drawing at path from old to new, with
// the diagnostics of its graph by kind and name; ok is false when path is
// no drawing, or it did not change and all is not set
func compare(path, old, new string, oldStats, newStats map[string]map[string]map[string]float64, all bool) (change Change, ok bool) {
	kind := strings.TrimPrefix(filepath.Ext(path), ".")
	if kind != "svg" && kind != "txt" && kind != "ans" || strings.HasPrefix(filepath.Base(path), "diagnostics") {
		return Change{}, false // diagnostics are no drawing
	}
	if !(all && (old != "" || new != "") || old != new) {
		return Change{}, false
	}
	// drawings are named as the graph, text colored or not
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	name = strings.TrimSuffix(name, ".truecolor")
	stats := "txt"
	if kind == "svg" {
		stats = "svg"
	}
	change = Change{
		Path: path, Kind: kind, Old: old, New: new,
		OldSize: size(kind, old), NewSize: size(kind, new),
		OldStats: oldStats[stats][name], NewStats: newStats[stats][name],
	}
	if kind != "svg" {
		// text is counted as drawn, see textStats
		change.OldStats = withText(change.OldStats, old)
		change.NewStats = withText(change.NewStats, new)
	}
	return change, true
}

// isDir reports whether path is a directory
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
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

// withText returns stats with the counts of the text drawing content in
// place of those of the layout, or nil when there is no drawing
func withText(stats map[string]float64, content string) map[string]float64 {
	if content == "" {
		return nil
	}
	out := map[string]float64{}
	for k, v := range stats {
		out[k] = v
	}
	for k, v := range textStats(content) {
		out[k] = v
	}
	return out
}

// diagnostics parses the lines of a diagnostics file, a name followed by
// key=value pairs, see layout.Diagnostics.String, into values by key by name
func diagnostics(content string) map[string]map[string]float64 {
	stats := map[string]map[string]float64{}
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		values := map[string]float64{}
		for _, field := range fields[1:] {
			key, value, ok := strings.Cut(field, "=")
			if v, err := strconv.ParseFloat(value, 64); ok && err == nil {
				values[key] = v
			}
		}
		stats[fields[0]] = values
	}
	return stats
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
