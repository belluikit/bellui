// Package build contains information about they type of build currently running.
package build

import (
	"sync"

	"github.com/belluikit/bellui"
)

var (
	migrateCheck sync.Once

	migratedBelluiDo bool
)

func MigratedToBelluiDo() bool {
	if DisableThreadChecks {
		return true
	}

	migrateCheck.Do(func() {
		v, ok := bellui.CurrentApp().Metadata().Migrations["belluiDo"]
		if ok {
			migratedBelluiDo = v
		}
	})

	return migratedBelluiDo
}
