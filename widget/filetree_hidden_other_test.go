//go:build !windows

package widget

// hideTestFile marks the file hidden; outside Windows the dot-prefix of the
// test file's name already does.
func hideTestFile(string) error {
	return nil
}
