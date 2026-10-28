package mobile

import (
	"errors"
	"testing"

	"github.com/belluikit/bellui/storage"
)

// TestOpenFailureSaysWhy pins #849: an Android document that would not open
// read "document does not exist" whatever the cause. The error now carries the
// Java exception, and a FileNotFoundException still matches
// storage.ErrNotExists.
func TestOpenFailureSaysWhy(t *testing.T) {
	for _, tc := range []struct {
		name, desc string
		notFound   bool
		wantMsg    string
		wantIs     bool
	}{
		{
			"offline Drive document", "java.io.FileNotFoundException: tfe: Cello error 2.", true,
			"java.io.FileNotFoundException: tfe: Cello error 2.", true,
		},
		{
			"permission refused", "java.lang.SecurityException: Permission Denial: reading com.example.Provider", false,
			"java.lang.SecurityException: Permission Denial: reading com.example.Provider", false,
		},
		{"no stream and no exception", "", false, storage.ErrNotExists.Error(), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := openFailure(tc.desc, tc.notFound)
			if err.Error() != tc.wantMsg {
				t.Errorf("Error() = %q; want %q", err.Error(), tc.wantMsg)
			}
			if got := errors.Is(err, storage.ErrNotExists); got != tc.wantIs {
				t.Errorf("errors.Is(err, storage.ErrNotExists) = %v; want %v", got, tc.wantIs)
			}
		})
	}
}
