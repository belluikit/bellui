// Command changelog assembles the per-change fragments under changelog.d/ into
// CHANGELOG.md, and checks that they are well-formed (#511). It is adapted from
// belluikit/govm's scripts/changelog.
//
// Why fragments rather than editing the Unreleased section directly: every
// change added its entry near the top of that section, so any two open PRs
// edited the same lines and the second to merge conflicted. Each change now
// adds a file of its own, and two PRs adding different files cannot conflict.
// The category is in the file name, so the assembled section has exactly one
// heading per category in a fixed order.
//
// The repository has two changelogs, each with its own fragments: the
// toolkit's (CHANGELOG.md, changelog.d/) and the bellui command's, which is
// its own module with its own tags (cmd/bellui/CHANGELOG.md,
// cmd/bellui/changelog.d/).
//
// Usage, from anywhere in the repository:
//
//	go run ./hack/changelog check                           # validate both changelogs
//	go run ./hack/changelog preview [-C cmd/bellui]         # print the Unreleased section
//	go run ./hack/changelog release [-C cmd/bellui] vX.Y.Z  # fold fragments into CHANGELOG.md
//
// release rewrites CHANGELOG.md and deletes the fragments it consumed; run it
// in the release commit (docs/RELEASING.md, step 1).
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// fragmentDir holds one file per change, next to the CHANGELOG.md it feeds.
const fragmentDir = "changelog.d"

// changelogs are the directories, relative to the repository root, that hold
// a CHANGELOG.md and its fragmentDir. check validates all of them.
var changelogs = []string{".", "cmd/bellui"}

// unreleasedHeading is the section the fragments stand in for. It keeps only
// a note saying where entries live.
const unreleasedHeading = "## Unreleased"

// sectionOrder is the order categories appear in an assembled section: Keep a
// Changelog's. A category outside it is a file name error, not a new heading.
var sectionOrder = []string{"security", "added", "changed", "deprecated", "removed", "fixed"}

var sectionTitle = map[string]string{
	"security":   "Security",
	"added":      "Added",
	"changed":    "Changed",
	"deprecated": "Deprecated",
	"removed":    "Removed",
	"fixed":      "Fixed",
}

// fragmentName is `<number>.<category>[-<n>].md`. The number is the issue the
// change closes, or its PR when it closes none. A second entry for the same
// number and category adds a counter: `427.fixed-2.md`.
var fragmentName = regexp.MustCompile(`^(\d+)\.([a-z]+)(?:-(\d+))?\.md$`)

type fragment struct {
	file   string
	number int
	cat    string
	seq    int
	body   string
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	dir := fs.String("C", ".", "the changelog to use: . for the toolkit's, cmd/bellui for the command's")
	_ = fs.Parse(os.Args[2:])

	root, err := repoRoot()
	if err != nil {
		fatal("%v", err)
	}
	switch cmd {
	case "check":
		for _, c := range changelogs {
			n, err := check(filepath.Join(root, c))
			if err != nil {
				fatal("%v", err)
			}
			fmt.Printf("changelog: %d fragment(s) in %s — ok\n", n, filepath.Join(c, fragmentDir))
		}
	case "preview":
		frags, err := load(filepath.Join(root, *dir, fragmentDir))
		if err != nil {
			fatal("%v", err)
		}
		if len(frags) == 0 {
			fmt.Println("changelog: no fragments")
			return
		}
		fmt.Print(assemble(frags, unreleasedHeading))
	case "release":
		if fs.NArg() != 1 || !strings.HasPrefix(fs.Arg(0), "v") {
			usage()
		}
		version := fs.Arg(0)
		frags, err := load(filepath.Join(root, *dir, fragmentDir))
		if err != nil {
			fatal("%v", err)
		}
		if len(frags) == 0 {
			fatal("no fragments in %s: a release section would be empty", filepath.Join(*dir, fragmentDir))
		}
		heading := fmt.Sprintf("## %s — %s", version, time.Now().Format("2006-01-02"))
		if err := release(filepath.Join(root, *dir), frags, heading); err != nil {
			fatal("%v", err)
		}
		fmt.Printf("changelog: folded %d fragment(s) into %s as %s\n", len(frags), filepath.Join(*dir, "CHANGELOG.md"), version)
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: changelog check | preview [-C dir] | release [-C dir] vX.Y.Z")
	os.Exit(2)
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "changelog: "+format+"\n", a...)
	os.Exit(1)
}

// check validates the fragments in dir's fragmentDir and that dir's
// CHANGELOG.md has no entries left in its Unreleased section. It returns the
// number of fragments.
func check(dir string) (int, error) {
	frags, err := load(filepath.Join(dir, fragmentDir))
	if err != nil {
		return 0, err
	}
	return len(frags), checkUnreleasedEmpty(filepath.Join(dir, "CHANGELOG.md"))
}

// checkUnreleasedEmpty reports entry bullets in the Unreleased section, where
// entries no longer live. The way they come back is a merge conflict resolved
// by keeping both sides, which is the usual fix for a changelog conflict and
// the wrong one for a change that moved the entries out. govm hit exactly
// this: the restored entries existed both there and as fragments, so a release
// would have printed each one twice.
func checkUnreleasedEmpty(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	in, n := false, 0
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(l, "## ") {
			if in {
				break
			}
			in = l == unreleasedHeading
			continue
		}
		if in && (strings.HasPrefix(l, "* ") || strings.HasPrefix(l, "- ")) {
			n++
		}
	}
	if n > 0 {
		return fmt.Errorf("%s: the Unreleased section holds %d entry bullet(s); entries belong in %s/ "+
			"(a merge that kept them would print each one twice at release). "+
			"Move each to %s/<issue>.<category>.md and keep the section's note", path, n, fragmentDir, fragmentDir)
	}
	return nil
}

// repoRoot walks up from the working directory to the repository's top, the
// directory holding .git (a directory in a clone, a file in a worktree).
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("not inside a git repository")
		}
		dir = parent
	}
}

// load reads and validates every fragment in dir, sorted into assembly order:
// category, then number with the newest (highest) first, as the changelog has
// always listed entries, then the counter.
func load(dir string) ([]fragment, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	var frags []fragment
	for _, e := range ents {
		if e.IsDir() || e.Name() == "README.md" || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		where := filepath.Join(filepath.Base(dir), e.Name())
		m := fragmentName.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("%s: name must be <issue>.<category>.md (e.g. 508.fixed.md); categories: %s",
				where, strings.Join(sectionOrder, ", "))
		}
		if _, ok := sectionTitle[m[2]]; !ok {
			return nil, fmt.Errorf("%s: unknown category %q; use one of: %s",
				where, m[2], strings.Join(sectionOrder, ", "))
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		body := strings.TrimRightFunc(string(raw), unicode.IsSpace)
		if !strings.HasPrefix(body, "* ") {
			return nil, fmt.Errorf("%s: body must be one markdown bullet starting `* ` "+
				"(the section heading comes from the file name, not the file)", where)
		}
		for _, l := range strings.Split(body, "\n") {
			if strings.HasPrefix(l, "#") {
				return nil, fmt.Errorf("%s: body holds a heading %q; the heading comes from the file name", where, l)
			}
		}
		number, _ := strconv.Atoi(m[1])
		seq := 1
		if m[3] != "" {
			seq, _ = strconv.Atoi(m[3])
		}
		frags = append(frags, fragment{file: e.Name(), number: number, cat: m[2], seq: seq, body: body})
	}
	rank := map[string]int{}
	for i, c := range sectionOrder {
		rank[c] = i
	}
	sort.Slice(frags, func(i, j int) bool {
		a, b := frags[i], frags[j]
		if rank[a.cat] != rank[b.cat] {
			return rank[a.cat] < rank[b.cat]
		}
		if a.number != b.number {
			return a.number > b.number
		}
		return a.seq < b.seq
	})
	return frags, nil
}

// assemble renders the fragments as one section under heading.
func assemble(frags []fragment, heading string) string {
	var b strings.Builder
	b.WriteString(heading)
	b.WriteString("\n")
	cat := ""
	for _, f := range frags {
		if f.cat != cat {
			cat = f.cat
			fmt.Fprintf(&b, "\n### %s\n\n", sectionTitle[cat])
		}
		b.WriteString(f.body)
		b.WriteString("\n")
	}
	return b.String()
}

// release puts the assembled section in dir's CHANGELOG.md, immediately above
// the most recent released one, so the Unreleased heading and its note stay at
// the top, and removes the fragments it consumed.
func release(dir string, frags []fragment, heading string) error {
	path := filepath.Join(dir, "CHANGELOG.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	unreleased := slices.Index(lines, unreleasedHeading)
	if unreleased < 0 {
		return fmt.Errorf("%s has no %q section", path, unreleasedHeading)
	}
	insert := len(lines)
	if i := slices.IndexFunc(lines[unreleased+1:], func(l string) bool { return strings.HasPrefix(l, "## ") }); i >= 0 {
		insert = unreleased + 1 + i
	}
	section := assemble(frags, heading)
	out := append([]string{}, lines[:insert]...)
	out = append(out, strings.Split(strings.TrimRight(section, "\n"), "\n")...)
	out = append(out, "")
	out = append(out, lines[insert:]...)
	if err := os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644); err != nil {
		return err
	}
	for _, f := range frags {
		if err := os.Remove(filepath.Join(dir, fragmentDir, f.file)); err != nil {
			return err
		}
	}
	return nil
}
