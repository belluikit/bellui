package bellui

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// sinceTag matches a "Since:" version in a Go comment.
var sinceTag = regexp.MustCompile(`//.*\bSince:?\s+(\d+)\.(\d+)`)

// Doc comments say which bellui release introduced an API. Tags carried over
// from Fyne used Fyne's versions (2.0 to 2.9), which mean nothing for bellui,
// whose first release is 1.0 (#336). Every tag in the repository, including
// the x/ and cmd/ modules, must name a 1.x version.
func TestSinceTagsNameBelluiVersions(t *testing.T) {
	var bad []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != "." && (strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		s := bufio.NewScanner(f)
		s.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for n := 1; s.Scan(); n++ {
			if m := sinceTag.FindStringSubmatch(s.Text()); m != nil && m[1] != "1" {
				bad = append(bad, path+":"+strconv.Itoa(n)+": "+strings.TrimSpace(s.Text()))
			}
		}
		return s.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) > 0 {
		t.Errorf("%d Since tags name a version outside bellui's 1.x line:\n%s", len(bad), strings.Join(bad, "\n"))
	}
}
