//go:build unix

// Command pickedreadprobe checks, on a device, that a document opened through
// the mobile driver's reader is read through its file rather than into memory
// (#1020): a file larger than the device's memory is read end to end, seeked
// and read at offsets, and the process's peak resident size stays flat.
//
// Launched, it runs by itself: it writes a fixture of fixtureSize bytes into
// its own storage, opens it with storage.Reader (the same reader a picked file
// gets, minus the security scope), checks every block, then deletes the
// fixture and quits. Lines starting "PROBE" go to the console:
//
//	xcrun devicectl device process launch --console --device <udid> com.trendvidia.pickedreadprobe
//
// The "Pick a file" button runs the same read on a file picked from Files,
// with its security scope, for a check by hand.
package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/dialog"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/widget"
)

const (
	block = 1 << 20
	// fixtureSize is all the memory of an iPad (10th generation), so more
	// than it has free.
	fixtureSize = 4 << 30
)

var logLbl *widget.Label

func logf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Print("PROBE " + msg)
	bellui.Do(func() { logLbl.SetText(logLbl.Text + msg + "\n") })
}

// maxRSS is the process's peak resident size in MiB; on Darwin ru_maxrss is
// in bytes.
func maxRSS() int64 {
	var ru syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return ru.Maxrss >> 20
}

// fill writes block n's pattern: its index, repeated.
func fill(b []byte, n int64) {
	for i := 0; i+8 <= len(b); i += 8 {
		binary.LittleEndian.PutUint64(b[i:], uint64(n))
	}
}

// check reports whether b holds block n's pattern from byte at onward.
func check(b []byte, n int64, at int) bool {
	for i := at - at%8; i+8 <= len(b); i += 8 {
		if i < at {
			continue
		}
		if binary.LittleEndian.Uint64(b[i:]) != uint64(n) {
			return false
		}
	}
	return true
}

func writeFixture(path string) error {
	var st syscall.Statfs_t
	if err := syscall.Statfs(filepath.Dir(path), &st); err != nil {
		return err
	}
	free := int64(st.Bavail) * int64(st.Bsize)
	logf("free storage %d MiB, fixture %d MiB", free>>20, int64(fixtureSize)>>20)
	if free < fixtureSize+(512<<20) {
		return fmt.Errorf("not enough free storage for the fixture")
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	b := make([]byte, block)
	for n := int64(0); n < fixtureSize/block; n++ {
		fill(b, n)
		if _, err := f.Write(b); err != nil {
			f.Close()
			return err
		}
	}
	return f.Close()
}

// readAll reads r end to end, checking blocks when verify, then seeks and
// reads at offsets. It returns an error on the first mismatch.
func readAll(r bellui.URIReadCloser, size int64, verify bool) error {
	s, seeks := r.(io.Seeker)
	ra, readsAt := r.(io.ReaderAt)
	logf("reader: io.Seeker %v, io.ReaderAt %v, rss %d MiB", seeks, readsAt, maxRSS())
	start := time.Now()
	b := make([]byte, block)
	var total int64
	for {
		n, err := io.ReadFull(r, b)
		if n > 0 {
			if verify && !check(b[:n], total/block, 0) {
				return fmt.Errorf("block %d reads wrong", total/block)
			}
			total += int64(n)
			if total%(1<<30) == 0 {
				logf("read %d GiB, rss %d MiB", total>>30, maxRSS())
			}
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return err
		}
	}
	logf("read %d bytes in %v, rss %d MiB", total, time.Since(start).Round(time.Millisecond), maxRSS())
	if size >= 0 && total != size {
		return fmt.Errorf("read %d bytes, want %d", total, size)
	}
	if !seeks {
		return fmt.Errorf("the reader cannot seek")
	}
	for _, at := range []int64{total - block, total / 2, 12345, 0} {
		if at < 0 {
			continue
		}
		pos, err := s.Seek(at, io.SeekStart)
		if err != nil || pos != at {
			return fmt.Errorf("seek to %d: %d, %v", at, pos, err)
		}
		n, err := io.ReadFull(r, b[:4096])
		if err != nil && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("read after seek to %d: %v", at, err)
		}
		if verify && !check(b[:n], at/block, int(at%8)) && at%block+int64(n) <= block {
			return fmt.Errorf("read after seek to %d reads wrong", at)
		}
	}
	end, err := s.Seek(0, io.SeekEnd)
	if err != nil || end != total {
		return fmt.Errorf("seek to the end: %d, %v; want %d", end, err, total)
	}
	logf("seeks ok, end at %d", end)
	if !readsAt {
		return fmt.Errorf("the reader cannot read at an offset")
	}
	at := (total / block / 3) * block
	// A read that reaches the end is short and may say io.EOF, as io.ReaderAt
	// allows: a file under one block reads that way.
	want := min(int64(block), total-at)
	n, err := ra.ReadAt(b, at)
	if int64(n) != want || (err != nil && (err != io.EOF || at+want != total)) || (verify && !check(b[:n], at/block, 0)) {
		return fmt.Errorf("read at %d: %d, %v; want %d", at, n, err, want)
	}
	logf("read at ok, rss %d MiB", maxRSS())
	return nil
}

func runFixture(a bellui.App) {
	dir := a.Storage().RootURI().Path()
	path := filepath.Join(dir, "pickedreadprobe.bin")
	logf("rss at start %d MiB", maxRSS())
	defer os.Remove(path) // the probe's own fixture: nothing is left on the device
	if err := writeFixture(path); err != nil {
		logf("RESULT SKIP: %v", err)
		return
	}
	logf("fixture written, rss %d MiB", maxRSS())
	r, err := storage.Reader(storage.NewFileURI(path))
	if err != nil {
		logf("RESULT FAIL: open: %v", err)
		return
	}
	defer r.Close()
	if err := readAll(r, fixtureSize, true); err != nil {
		logf("RESULT FAIL: %v", err)
		return
	}
	logf("RESULT PASS: %d MiB read, peak rss %d MiB", int64(fixtureSize)>>20, maxRSS())
}

func main() {
	a := app.NewWithID("com.trendvidia.pickedreadprobe")
	w := a.NewWindow("Picked read probe")
	logLbl = widget.NewLabel("")
	logLbl.Wrapping = bellui.TextWrapWord
	pick := widget.NewButton("Pick a file", func() {
		dialog.ShowFileOpen(func(r bellui.URIReadCloser, err error) {
			if err != nil || r == nil {
				logf("pick: %v", err)
				return
			}
			go func() {
				defer r.Close()
				logf("picked %s", r.URI().Name())
				if err := readAll(r, -1, false); err != nil {
					logf("PICK FAIL: %v", err)
					return
				}
				logf("PICK PASS: peak rss %d MiB", maxRSS())
			}()
		}, w)
	})
	w.SetContent(container.NewBorder(pick, nil, nil, nil, container.NewVScroll(logLbl)))
	a.Lifecycle().SetOnStarted(func() {
		go func() {
			runFixture(a)
			if os.Getenv("PROBE_STAY") == "" {
				// A mobile app is not ended by Quit, so the probe ends itself:
				// nothing it started keeps running.
				logf("quitting")
				time.Sleep(time.Second)
				os.Exit(0)
			}
		}()
	})
	bellui.ShowAndRun(w)
}
