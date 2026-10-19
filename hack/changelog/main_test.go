package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// write drops a fragment into dir.
func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The order comes from the file names, not from the order the directory is
// read in, so two people adding fragments produce the same CHANGELOG.md.
func TestLoadOrdersByCategoryThenNewestFirst(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "42.fixed.md", "* fixed 42\n")
	write(t, dir, "981.fixed.md", "* fixed 981\n")
	write(t, dir, "700.added.md", "* added 700\n")
	write(t, dir, "877.security.md", "* security 877\n")
	write(t, dir, "981.fixed-2.md", "* fixed 981 again\n")
	write(t, dir, "README.md", "not a fragment\n")

	frags, err := load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var got []string
	for _, f := range frags {
		got = append(got, f.file)
	}
	want := []string{"877.security.md", "700.added.md", "981.fixed.md", "981.fixed-2.md", "42.fixed.md"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("order = %v, want %v (category, then highest number first, then counter)", got, want)
	}
}

func TestAssembleGroupsOneHeadingPerCategory(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "1.fixed.md", "* fix one\n")
	write(t, dir, "2.added.md", "* add two\n  continued  \n  \n")
	write(t, dir, "3.fixed.md", "* fix three\n")

	frags, err := load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got := assemble(frags, "## v1.2.0 — 2026-10-20")
	want := "## v1.2.0 — 2026-10-20\n\n### Added\n\n* add two\n  continued\n\n### Fixed\n\n* fix three\n* fix one\n"
	if got != want {
		t.Errorf("assembled:\n%s\nwant:\n%s", got, want)
	}
}

// A malformed fragment fails check, rather than turning up misfiled at
// release time.
func TestLoadRejectsMalformedFragments(t *testing.T) {
	for _, tc := range []struct {
		name, file, body, wantErr string
	}{
		{"no category", "508.md", "* x\n", "must be <issue>.<category>.md"},
		{"unknown category", "508.improved.md", "* x\n", "unknown category"},
		{"no number", "entry.fixed.md", "* x\n", "must be <issue>.<category>.md"},
		{"body starts with a heading", "508.fixed.md", "### Fixed\n\n* x\n", "one markdown bullet"},
		{"body holds a heading", "508.fixed.md", "* x\n\n### Added\n\n* y\n", "holds a heading"},
		{"body is prose", "508.fixed.md", "we fixed a thing\n", "one markdown bullet"},
		{"dash bullet", "508.fixed.md", "- x\n", "one markdown bullet"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, tc.file, tc.body)
			_, err := load(dir)
			if err == nil {
				t.Fatalf("load accepted %s containing %q", tc.file, tc.body)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

// release puts the new section above the most recent released one, keeps the
// Unreleased heading and its note, and consumes the fragments.
func TestReleaseSplicesAndConsumes(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, fragmentDir), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, fragmentDir), "508.fixed.md", "* fix the thing\n")
	write(t, filepath.Join(dir, fragmentDir), "README.md", "how to write one\n")
	prior := "# Changelog\n\n## Unreleased\n\nEntries live in changelog.d/.\n\n## v1.1.2 — 2026-10-16\n\n### Fixed\n\n* old entry\n"
	write(t, dir, "CHANGELOG.md", prior)

	frags, err := load(filepath.Join(dir, fragmentDir))
	if err != nil {
		t.Fatal(err)
	}
	if err := release(dir, frags, "## v1.2.0 — 2026-10-20"); err != nil {
		t.Fatalf("release: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "# Changelog\n\n## Unreleased\n\nEntries live in changelog.d/.\n\n" +
		"## v1.2.0 — 2026-10-20\n\n### Fixed\n\n* fix the thing\n\n" +
		"## v1.1.2 — 2026-10-16\n\n### Fixed\n\n* old entry\n"
	if string(raw) != want {
		t.Errorf("CHANGELOG.md after release:\n%s\nwant:\n%s", raw, want)
	}
	if _, err := os.Stat(filepath.Join(dir, fragmentDir, "508.fixed.md")); !os.IsNotExist(err) {
		t.Errorf("the consumed fragment is still there (stat: %v)", err)
	}
	if _, err := os.Stat(filepath.Join(dir, fragmentDir, "README.md")); err != nil {
		t.Errorf("release removed the README: %v", err)
	}
	if n, err := check(dir); err != nil || n != 0 {
		t.Errorf("check after release = %d, %v; want 0 fragments and no error", n, err)
	}
}

// An entry written into the Unreleased section, as a keep-both merge
// resolution brings the old entries back, fails check.
func TestCheckRejectsEntriesInUnreleased(t *testing.T) {
	for _, bullet := range []string{"* ", "- "} {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, fragmentDir), 0o755); err != nil {
			t.Fatal(err)
		}
		write(t, dir, "CHANGELOG.md", "# Changelog\n\n## Unreleased\n\nEntries live in changelog.d/.\n\n### Fixed\n\n"+
			bullet+"an entry\n\n## v1.1.2 — 2026-10-16\n\n* old entry\n")
		if _, err := check(dir); err == nil || !strings.Contains(err.Error(), "1 entry bullet") {
			t.Errorf("check with a %q entry in Unreleased = %v, want it refused", bullet, err)
		}
	}
}

// The repository's own changelogs pass check, so go test catches a bad
// fragment as well as the CI step does.
func TestRepositoryChangelogs(t *testing.T) {
	for _, c := range changelogs {
		if _, err := check(filepath.Join("..", "..", c)); err != nil {
			t.Error(err)
		}
	}
}
