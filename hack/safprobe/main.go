// Command safprobe verifies what an app can actually do with a folder the user
// picked through dialog.ShowFolderOpen on Android — the Storage Access
// Framework path that #303 reported unusable.
//
// It is scripted: on launch with no remembered folder it opens the picker
// straight away, and on the pick it runs the whole scaffold sequence and
// prints one "safprobe:" line per step. Relaunching it exercises the other
// half — the folder URI is remembered in preferences, so a second run that can
// still list it proves takePersistableUriPermission took (without the grant
// the list fails, which is what every pick did before #303).
//
// Drive it with hack/safprobe/drive.sh; see that script for the adb flow.
package main

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/dialog"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/widget"
)

const prefRoot = "root"

// scaffoldName is the directory the probe creates under the picked folder. It
// is deleted at the end of a run, so a rerun starts from the same state.
const scaffoldName = "belluiscaffold"

var log = widget.NewLabel("")

// say prints a step's result to logcat (tag GoLog) and appends it on screen.
// The label update hops to the main goroutine: the probe's steps run from the
// picker callback, and widget mutation off the main goroutine is exactly what
// the fork's discipline forbids.
func say(format string, args ...any) {
	line := "safprobe: " + fmt.Sprintf(format, args...)
	fmt.Println(line)
	bellui.Do(func() {
		log.SetText(strings.TrimPrefix(log.Text+"\n"+line, "\n"))
	})
}

// check prints PASS or FAIL for one claim, so a driver script can count both
// rather than read the absence of a FAIL as success.
func check(claim string, ok bool, detail string) {
	if ok {
		say("PASS %s%s", claim, detail)
		return
	}
	say("FAIL %s%s", claim, detail)
}

func main() {
	a := app.NewWithID("io.bellui.safprobe")
	w := a.NewWindow("safprobe")

	pick := widget.NewButton("Pick folder", func() {
		dialog.ShowFolderOpen(func(dir bellui.ListableURI, err error) {
			if err != nil {
				say("FAIL picker returned error: %v", err)
				return
			}
			if dir == nil {
				say("picker cancelled")
				return
			}
			bellui.SetPref(a.Preferences(), prefRoot, dir.String())
			run(dir)
		}, w)
	})

	w.SetContent(container.NewBorder(pick, nil, nil, nil, container.NewVScroll(log)))
	w.Resize(geo.NewSize(400, 700))

	// Start the scripted run only once the activity is actually in the
	// foreground. Before that the mobile driver has no platform app object
	// yet, so ShowFolderOpenPicker reports "no picker" and the dialog quietly
	// falls back to the canvas one — which browses app-private storage and
	// never touches SAF, so the run would pass while testing nothing.
	var once sync.Once
	a.Lifecycle().SetOnEnteredForeground(func() {
		once.Do(func() {
			// A launch marker, so a driver script reading a log buffer it
			// could not clear can tell this run's lines from the last one's.
			say("START")
			if saved := bellui.PrefOr(a.Preferences(), prefRoot, ""); saved != "" {
				rerun(saved)
				return
			}
			pick.OnTapped()
		})
	})

	bellui.ShowAndRun(w)
}

// rerun answers item 4 of #303: the folder URI was stored last run, so listing
// it now is only possible if the grant was persisted across process death.
func rerun(saved string) {
	say("remembered folder: %s", saved)
	u, err := storage.ParseURI(saved)
	if err != nil {
		check("remembered URI parses", false, ": "+err.Error())
		return
	}

	children, err := storage.List(u)
	check("persisted grant still lists the folder", err == nil,
		fmt.Sprintf(": %d children, err=%v", len(children), err))

	dir, err := storage.CreateChild(u, scaffoldName+"-rerun", true)
	check("persisted grant still writes", err == nil, fmt.Sprintf(": %v", err))
	if err == nil {
		check("cleanup of the rerun directory", storage.Delete(dir) == nil, "")
	}
	say("DONE rerun")
}

func run(dir bellui.ListableURI) {
	say("picked: %s", dir.String())
	say("  Scheme()=%q Name()=%q Path()=%q", dir.Scheme(), dir.Name(), dir.Path())

	listable, err := storage.CanList(dir)
	check("CanList(picked folder)", listable && err == nil, fmt.Sprintf(": %v %v", listable, err))

	before, err := storage.List(dir)
	check("List(picked folder)", err == nil, fmt.Sprintf(": %d children, err=%v", len(before), err))

	// Child() of something that is not there: SAF cannot name it, and saying
	// so is the point — the old code answered ErrOperationNotSupported for
	// every content:// URI, existing or not.
	_, err = storage.Child(dir, scaffoldName)
	check("Child(missing) reports ErrNotExists", errors.Is(err, storage.ErrNotExists),
		fmt.Sprintf(": %v", err))

	// 1. a directory under the picked folder — the operation #303 opened on.
	scaffold, err := storage.CreateChild(dir, scaffoldName, true)
	if err != nil {
		check("CreateChild(dir, listable)", false, ": "+err.Error())
		return
	}
	check("CreateChild(dir, listable)", true, ": "+scaffold.String())

	// 2. Child() finds it now that it exists, and answers the same URI.
	found, err := storage.Child(dir, scaffoldName)
	check("Child(existing) resolves by display name",
		err == nil && found != nil && found.String() == scaffold.String(),
		fmt.Sprintf(": %v %v", found, err))

	// 3. a nested directory: this is what the doc-ID bug broke. Before the
	// fix, List and CanList of a sub-document answered for the tree ROOT,
	// because getTreeDocumentId always returns the root's ID.
	sub, err := storage.CreateChild(scaffold, "sub", true)
	check("CreateChild(nested dir)", err == nil, fmt.Sprintf(": %v %v", sub, err))
	if err != nil {
		return
	}

	kids, err := storage.List(scaffold)
	names := make([]string, 0, len(kids))
	for _, k := range kids {
		names = append(names, k.Name())
	}
	check("List(subdirectory) lists ITS children, not the tree root",
		err == nil && len(kids) == 1 && len(names) == 1 && names[0] == "sub",
		fmt.Sprintf(": %v err=%v", names, err))

	// 4. a file in the nested directory, written and read back.
	file, err := storage.CreateChild(sub, "main.go", false)
	check("CreateChild(file)", err == nil, fmt.Sprintf(": %v %v", file, err))
	if err != nil {
		return
	}

	const payload = "package main // written by safprobe\n"
	wc, err := storage.Writer(file)
	if err == nil {
		_, err = wc.Write([]byte(payload))
		if cerr := wc.Close(); err == nil {
			err = cerr
		}
	}
	check("Writer(file) writes", err == nil, fmt.Sprintf(": %v", err))

	rc, err := storage.Reader(file)
	var got []byte
	if err == nil {
		got, err = io.ReadAll(rc)
		rc.Close()
	}
	check("Reader(file) reads back what was written",
		err == nil && string(got) == payload, fmt.Sprintf(": %q err=%v", string(got), err))

	exists, err := storage.Exists(file)
	check("Exists(file)", exists && err == nil, fmt.Sprintf(": %v %v", exists, err))

	// 5. creating the same name twice fails rather than silently making
	// "belluiscaffold (1)", which is what the provider does on its own.
	_, err = storage.CreateChild(dir, scaffoldName, true)
	check("CreateChild refuses a duplicate name", err != nil, fmt.Sprintf(": %v", err))

	after, err := storage.List(dir)
	check("the picked folder gained exactly one child",
		err == nil && len(after) == len(before)+1,
		fmt.Sprintf(": %d -> %d, err=%v", len(before), len(after), err))

	// 6. Parent() is still unsupported for content://, on purpose: SAF has no
	// upward link. Pinned here so a change of mind is a visible failure.
	_, err = storage.Parent(scaffold)
	check("Parent(content://) is still unsupported", err != nil, fmt.Sprintf(": %v", err))

	// 7. delete, recursively — and prove it took.
	err = storage.Delete(file)
	check("Delete(file)", err == nil, fmt.Sprintf(": %v", err))
	exists, _ = storage.Exists(file)
	check("the deleted file is gone", !exists, "")

	err = storage.Delete(scaffold)
	check("Delete(directory, recursive)", err == nil, fmt.Sprintf(": %v", err))

	final, err := storage.List(dir)
	check("the picked folder is back to where it started",
		err == nil && len(final) == len(before),
		fmt.Sprintf(": %d -> %d, err=%v", len(before), len(final), err))

	say("DONE first run")
}
