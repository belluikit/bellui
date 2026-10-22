// This is a -race harness: a tight Localize busy-loop against concurrent
// SetLocale writes. On wasm there is no race detector and no parallelism to
// exercise — and the yield-free reader loop LIVELOCKS the cooperative
// single-threaded scheduler (the whole lang package hung 3m30s in-browser
// until the go test timeout killed it; #79 Phase B). The native -race legs
// keep the coverage this exists for.
//go:build !wasm

package lang

import (
	"sync"
	"testing"

	"github.com/belluikit/bellui"
)

// TestSetLocale_ConcurrentLocalize exercises the localizer pointer under
// concurrent SetLocale writes and Localize reads. With -race it fails if the
// localizer global is accessed without synchronisation (#82).
func TestSetLocale_ConcurrentLocalize(t *testing.T) {
	t.Cleanup(func() { SetLocale("") })

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = Localize("OK")
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		locales := []bellui.Locale{"en", "ar-EG", "fr-FR", ""}
		for i := 0; i < 200; i++ {
			SetLocale(locales[i%len(locales)])
		}
		close(stop)
	}()

	wg.Wait()
}

// TestAddTranslations_Concurrent exercises the shared bundle and translated
// globals: AddTranslations mutates them while other goroutines localize and
// switch locale. With -race it fails if those globals are accessed without
// synchronisation (#84).

func TestAddTranslations_Concurrent(t *testing.T) {
	t.Cleanup(func() { SetLocale("") })

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = Localize("OK")
					SetLocale("fr-FR")
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = AddTranslations(bellui.NewStaticResource("de.pxf", []byte(`messages = { OK: "OK" }`)))
		}
		close(stop)
	}()

	wg.Wait()
}

// TestUpdateLocalizer_EmptyLocaleList guards the C/POSIX-locale startup path
// (pre-rebrand tracker #1148): locale.GetLocales reports an empty list without an error under
// LANG=C / LC_ALL=C, and updateLocalizer must fall back to "en" instead of
// panicking on all[0] — the panic ran from this package's init, killing any
// binary before main on hosts with a stripped locale environment.
func TestCompare_ConcurrentSafe(t *testing.T) {
	SetLocaleForTests(t, "de")

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = Compare("äpfel", "zebra")
				_ = CompareFold("Apple", "apple")
			}
		}()
	}
	wg.Wait()
}
