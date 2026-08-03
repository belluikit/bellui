package dialog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/storage/repository"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// ViewLayout can be passed to SetView() to set the view of
// a FileDialog
//
// Since: 2.5
type ViewLayout int

const (
	defaultView ViewLayout = iota
	ListView
	GridView
)

const (
	viewLayoutKey    = "bellui:fileDialogViewLayout"
	lastFolderKey    = "bellui:fileDialogLastFolder"
	fileSidebarWidth = float32(220)
)

type textWidget interface {
	core.Widget
	SetText(string)
}

type favoriteItem struct {
	locName string
	locIcon bellui.Resource
	loc     bellui.URI
}

type fileDialogPanel interface {
	core.Widget

	Unselect(int)
}

type fileDialog struct {
	file             *FileDialog
	fileName         textWidget
	title            *widget.Label
	dismiss          *widget.Button
	open             *widget.Button
	breadcrumb       *bellui.Container
	breadcrumbScroll *container.Scroll
	files            fileDialogPanel
	filesScroll      *container.Scroll
	favorites        []favoriteItem
	favoritesList    *widget.List
	showHidden       bool

	view ViewLayout

	data         []bellui.URI
	filteredData []bellui.URI
	filterQuery  string

	win        *widget.PopUp
	selected   bellui.URI
	selectedID int
	dir        bellui.ListableURI

	// multi-select state (#627): the ordered set of chosen files and the
	// anchor id for Shift-range selection.
	multiSelected []bellui.URI
	multiAnchor   int

	// set while the dialog body is shown in its own native window (#597)
	hostWin   bellui.Window
	scrim     *modalScrim
	body      core.CanvasObject
	startSize geo.Size
	// this will be the initial filename in a FileDialog in save mode
	initialFileName string

	toggleViewButton *widget.IconButton
	searchField      *widget.SearchField
	searchContainer  *bellui.Container
	searchActive     bool
}

// FileDialog is a dialog containing a file picker for use in opening or saving files.
type FileDialog struct {
	callback         any
	onClosedCallback func(bool)
	parent           bellui.Window
	dialog           *fileDialog

	titleText                string
	confirmText, dismissText string
	desiredSize              geo.Size
	filter                   storage.FileFilter
	save                     bool
	// this will be applied to dialog.dir when it's loaded
	startingLocation bellui.ListableURI
	// this will be the initial filename in a FileDialog in save mode
	initialFileName string
	// this will be the initial view in a FileDialog
	initialView ViewLayout
	// this will set the initial state of the show hidden files toggle
	showHidden bool
	// windowed hosts the dialog in a draggable, resizable window frame
	windowed bool
	// windowHosted opens the dialog in its own native window (#597)
	windowHosted bool
	// multiSelect allows choosing more than one file (open mode only, #627)
	multiSelect bool
}

// Declare conformity to Dialog interface
var _ Dialog = (*FileDialog)(nil)

// noNativeFileDialogs records an explicit opt-out. The default is to use the
// platform's own picker, so the zero value has to mean "native" — hence the
// negative sense. Flatpak builds ignore it and always use the portal (the only
// correct path inside the sandbox).
var noNativeFileDialogs atomic.Bool

// UseNativeFileDialogs enables (or disables) the platform-native file dialog
// backends for FileOpen, FileSave and FolderOpen: NSOpenPanel/NSSavePanel on
// macOS, IFileOpenDialog/IFileSaveDialog on Windows and the
// org.freedesktop.portal.FileChooser portal on Linux/BSD.
//
// They are used by default: a file picker is where users expect their own
// sidebar, favourites, search and network mounts, and on Linux the portal is
// the only backend that works inside Flatpak/Snap. Pass false to force the
// canvas-drawn dialog everywhere instead.
//
// When the native backend cannot serve a request the canvas-drawn dialog is
// used, so an app never loses the feature. That covers no portal on the bus,
// an unsupported platform, and a window with no platform handle behind it.
// The SetShowHidden toggle is canvas-dialog-only; the native surfaces cannot
// express it.
//
// Flatpak builds always use the portal regardless of this setting.
//
// Since: 2.17. The native backends were opt-in when introduced; the default
// changed to enabled in 2.19 (#226).
func UseNativeFileDialogs(enable bool) {
	noNativeFileDialogs.Store(!enable)
}

// nativeFileDialogsEnabled reports whether the native backends are wanted at
// all. Flatpak forces them on; everywhere else an app can opt out.
func nativeFileDialogsEnabled() bool {
	return build.IsFlatpak || !noNativeFileDialogs.Load()
}

// nativeFileDialogsFor reports whether the native backend should handle this
// particular dialog. A native picker is a window of the platform's own, and
// needs a platform window to sit over: a driver that has none — the test
// driver, the embedded driver — falls back to the canvas dialog rather than
// putting up a picker its window system knows nothing about.
func nativeFileDialogsFor(d *FileDialog) bool {
	if !nativeFileDialogsEnabled() {
		return false
	}
	if build.IsFlatpak {
		// The portal is the only dialog that can see outside the sandbox — a
		// canvas dialog would browse an empty filesystem — and it works
		// unparented, so the window check must not disqualify it.
		return true
	}
	_, native := d.parent.(driver.NativeWindow)
	return native
}

func (f *fileDialog) makeUI() core.CanvasObject {
	th := bellui.CurrentApp().Settings().Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	if f.file.save {
		saveName := widget.NewEntry()
		saveName.Style = widget.EntryStyleOutlined
		saveName.OnChanged = func(s string) {
			if s == "" {
				f.open.Disable()
			} else {
				f.open.Enable()
			}
		}
		saveName.SetPlaceHolder(lang.L("Enter filename"))
		saveName.OnSubmitted = func(s string) {
			f.open.OnTapped()
		}
		f.fileName = saveName
	} else {
		f.fileName = widget.NewLabel("")
	}

	label := lang.L("Open")
	if f.file.save {
		label = lang.L("Save")
	}
	if f.file.confirmText != "" {
		label = f.file.confirmText
	}
	f.open = f.makeOpenButton(label)

	if f.file.save {
		f.fileName.SetText(f.initialFileName)
	}

	dismissLabel := lang.L("Cancel")
	if f.file.dismissText != "" {
		dismissLabel = f.file.dismissText
	}
	f.dismiss = f.makeDismissButton(dismissLabel)

	// --- File content area ---
	f.filesScroll = container.NewScroll(nil)
	verticalExtra := float32(float64(fileIconSize) * 0.25)
	itemMin := f.newFileItem(storage.NewFileURI("filename.txt"), false, false).MinSize()
	f.filesScroll.SetMinSize(itemMin.AddWidthHeight(itemMin.Width+theme.Padding()*3, verticalExtra))

	// --- Breadcrumbs ---
	f.breadcrumb = container.NewHBox()
	f.breadcrumbScroll = container.NewHScroll(container.NewPadded(f.breadcrumb))

	// --- Title ---
	title := label + " " + lang.L("File")
	if f.file.isDirectory() {
		title = label + " " + lang.L("Folder")
	}
	if f.file.titleText != "" {
		title = f.file.titleText
	}
	f.title = widget.NewLabelWithStyle(title, bellui.TextAlignLeading, bellui.TextStyle{Bold: true})

	// --- View toggle ---
	view := ViewLayout(bellui.PrefOr[int](bellui.CurrentApp().Preferences(), viewLayoutKey, 0))
	if view != GridView && view != ListView {
		view = defaultView
	}
	if view == defaultView {
		view = GridView
		if f.file.initialView != defaultView {
			view = f.file.initialView
		}
	}

	f.toggleViewButton = widget.NewIconButton(nil, func() {
		if f.view == GridView {
			f.setView(ListView)
		} else {
			f.setView(GridView)
		}
	})
	f.setView(view)

	// --- Search ---
	f.searchField = widget.NewSearchField(func(query string) {
		f.filterQuery = query
		f.updateFilter()
		f.files.Refresh()
		f.filesScroll.Refresh()
	})
	f.searchContainer = container.NewPadded(f.searchField)
	f.searchContainer.Hide()

	// --- Favorites sidebar ---
	f.loadFavorites()

	f.favoritesList = widget.NewList(
		func() int {
			return len(f.favorites)
		},
		func() core.CanvasObject {
			return container.NewHBox(container.New(&iconPaddingLayout{}, widget.NewIcon(theme.DocumentIcon())), widget.NewLabel("Template Object"))
		},
		func(id widget.ListItemID, item core.CanvasObject) {
			item.(*bellui.Container).Objects[0].(*bellui.Container).Objects[0].(*widget.Icon).SetResource(f.favorites[id].locIcon)
			item.(*bellui.Container).Objects[1].(*widget.Label).SetText(f.favorites[id].locName)
		},
	)
	f.favoritesList.OnSelected = func(id widget.ListItemID) {
		f.setLocation(f.favorites[id].loc)
	}

	// --- Action buttons ---
	searchButton := widget.NewIconButton(theme.SearchIcon(), func() {
		f.toggleSearch()
	})

	var optionsButton *widget.IconButton
	optionsButton = widget.NewIconButton(theme.SettingsIcon(), func() {
		f.optionsMenu(bellui.CurrentApp().Driver().AbsolutePositionForObject(optionsButton), optionsButton.Size())
	})

	newFolderButton := widget.NewIconButton(theme.FolderNewIcon(), func() {
		newFolderEntry := widget.NewEntry()
		ShowForm(lang.L("New Folder"), lang.L("Create Folder"), lang.L("Cancel"), []*widget.FormItem{
			{
				Text:   lang.X("file.name", "Name"),
				Widget: newFolderEntry,
			},
		}, func(s bool) {
			if !s || newFolderEntry.Text == "" {
				return
			}

			newFolderPath := filepath.Join(f.dir.Path(), newFolderEntry.Text)
			createFolderErr := os.MkdirAll(newFolderPath, 0o750)
			if createFolderErr != nil {
				bellui.LogError(
					fmt.Sprintf("Failed to create folder with path %s", newFolderPath),
					createFolderErr,
				)
				ShowError(errors.New("folder cannot be created"), f.dialogParent())
			}
			f.refreshDir(f.dir)
		}, f.dialogParent())
	})

	actions := container.NewHBox(searchButton, f.toggleViewButton, newFolderButton, optionsButton)

	// === HEADER BAR (TopAppBar-style) ===
	headerBg := canvas.NewRectangle(th.Color(theme.ColorNameSurfaceContainerLow, v))
	headerDivider := canvas.NewRectangle(th.Color(theme.ColorNameOutlineVariant, v))
	header := container.New(&headerBarLayout{}, headerBg, headerDivider, f.title, actions)

	// === BREADCRUMB BAR ===
	breadcrumbBg := canvas.NewRectangle(th.Color(theme.ColorNameSurface, v))
	breadcrumbBar := container.NewStack(breadcrumbBg, f.breadcrumbScroll)

	// === SIDEBAR ===
	sidebarBg := canvas.NewRectangle(th.Color(theme.ColorNameSurfaceContainerLow, v))
	sidebarContent := container.NewStack(sidebarBg, f.favoritesList)
	sidebar := container.New(&fixedWidthLayout{width: fileSidebarWidth}, sidebarContent)

	// === CONTENT AREA ===
	contentBg := canvas.NewRectangle(th.Color(theme.ColorNameSurface, v))
	contentArea := container.NewStack(contentBg, f.filesScroll)

	// === FOOTER ===
	buttons := container.NewGridWithRows(1, f.dismiss, f.open)
	footerBg := canvas.NewRectangle(th.Color(theme.ColorNameSurfaceContainerMedium, v))
	footerDivider := canvas.NewRectangle(th.Color(theme.ColorNameOutlineVariant, v))
	footerContent := container.NewBorder(nil, nil, nil, buttons, container.NewHScroll(f.fileName))
	footer := container.New(&footerBarLayout{}, footerBg, footerDivider, footerContent)

	// === COMPOSE LAYOUT ===
	topSection := container.NewVBox(header, breadcrumbBar, f.searchContainer)
	body := container.NewBorder(nil, nil, sidebar, nil, contentArea)

	dialogBg := canvas.NewRectangle(th.Color(theme.ColorNameSurface, v))
	return container.NewStack(dialogBg, container.NewBorder(topSection, footer, nil, nil, body))
}

func (f *fileDialog) makeOpenButton(label string) *widget.Button {
	btn := widget.NewButton(label, func() {
		if f.file.callback == nil {
			f.hide()
			if f.file.onClosedCallback != nil {
				f.file.onClosedCallback(false)
			}
			return
		}

		if f.file.save {
			callback := f.file.callback.(func(bellui.URIWriteCloser, error))
			name := f.fileName.(*widget.Entry).Text
			location, _ := storage.Child(f.dir, name)

			exists, _ := storage.Exists(location)
			if !exists {
				f.hide()
				if f.file.onClosedCallback != nil {
					f.file.onClosedCallback(true)
				}
				callback(storage.Writer(location))
				return
			}

			listable, err := storage.CanList(location)
			if err == nil && listable {
				ShowInformation("Cannot overwrite",
					"Files cannot replace a directory,\ncheck the file name and try again", f.dialogParent())
				return
			}

			ShowConfirm("Overwrite?", "Are you sure you want to overwrite the file\n"+name+"?",
				func(ok bool) {
					if !ok {
						return
					}
					f.hide()

					callback(storage.Writer(location))
					if f.file.onClosedCallback != nil {
						f.file.onClosedCallback(true)
					}
				}, f.dialogParent())
		} else if f.file.multiSelect {
			callback := f.file.callback.(func([]bellui.URIReadCloser, error))
			readers := make([]bellui.URIReadCloser, 0, len(f.multiSelected))
			for _, u := range f.multiSelected {
				r, err := storage.Reader(u)
				if err != nil {
					f.hide()
					if f.file.onClosedCallback != nil {
						f.file.onClosedCallback(true)
					}
					callback(nil, err)
					return
				}
				readers = append(readers, r)
			}
			f.hide()
			if f.file.onClosedCallback != nil {
				f.file.onClosedCallback(true)
			}
			callback(readers, nil)
		} else if f.selected != nil {
			callback := f.file.callback.(func(bellui.URIReadCloser, error))
			f.hide()
			if f.file.onClosedCallback != nil {
				f.file.onClosedCallback(true)
			}
			callback(storage.Reader(f.selected))
		} else if f.file.isDirectory() {
			callback := f.file.callback.(func(bellui.ListableURI, error))
			f.hide()
			if f.file.onClosedCallback != nil {
				f.file.onClosedCallback(true)
			}
			callback(f.dir, nil)
		}
	})

	btn.Importance = widget.HighImportance
	btn.Disable()

	return btn
}

func (f *fileDialog) makeDismissButton(label string) *widget.Button {
	btn := widget.NewButton(label, func() {
		f.hide()
		f.reportCancel()
	})

	return btn
}

// reportCancel reports a cancellation to the onClosed and result callbacks,
// passing a nil result of the type expected for this dialog's mode. It does not
// tear down the surface (the caller does that first, or the window is already
// gone on an external close).
func (f *fileDialog) reportCancel() {
	if f.file.onClosedCallback != nil {
		f.file.onClosedCallback(false)
	}
	if f.file.callback != nil {
		if f.file.save {
			f.file.callback.(func(bellui.URIWriteCloser, error))(nil, nil)
		} else if f.file.isDirectory() {
			f.file.callback.(func(bellui.ListableURI, error))(nil, nil)
		} else {
			f.file.callback.(func(bellui.URIReadCloser, error))(nil, nil)
		}
	}
}

func (f *fileDialog) optionsMenu(position geo.Position, buttonSize geo.Size) {
	hiddenFiles := widget.NewCheck(lang.L("Show Hidden Files"), func(changed bool) {
		f.showHidden = changed
		f.refreshDir(f.dir)
	})
	hiddenFiles.Checked = f.showHidden
	hiddenFiles.Refresh()
	content := container.NewVBox(hiddenFiles)

	p := position.Add(buttonSize)
	pos := geo.NewPos(p.X-content.MinSize().Width-theme.Padding()*2, p.Y+theme.Padding()*2)
	widget.ShowPopUpAtPosition(content, f.canvas(), pos)
}

func getFavoriteLocations() (map[string]bellui.ListableURI, error) {
	if runtime.GOOS == "js" {
		return make(map[string]bellui.ListableURI), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	homeURI := storage.NewFileURI(homeDir)
	home, _ := storage.ListerForURI(homeURI)

	favoriteLocations := map[string]bellui.ListableURI{"Home": home}
	for _, favName := range getFavoritesOrder() {
		uri, err1 := getFavoriteLocation(homeURI, favName)
		if err != nil {
			err = err1
			continue
		}

		listURI, err1 := storage.ListerForURI(uri)
		if err1 != nil {
			err = err1
			continue
		}
		favoriteLocations[favName] = listURI
	}

	return favoriteLocations, err
}

func (f *fileDialog) loadFavorites() {
	favoriteLocations, err := getFavoriteLocations()
	if err != nil {
		bellui.LogError("Getting favorite locations", err)
	}

	f.favorites = []favoriteItem{
		{locName: "Home", locIcon: theme.HomeIcon(), loc: favoriteLocations["Home"]},
	}
	app := bellui.CurrentApp()
	if hasAppFiles(app) {
		f.favorites = append(f.favorites,
			favoriteItem{locName: "App Files", locIcon: theme.FileIcon(), loc: storageURI(app)})
	}
	f.favorites = append(f.favorites, f.getPlaces()...)

	for _, locName := range getFavoritesOrder() {
		loc, ok := favoriteLocations[locName]
		if !ok {
			continue
		}
		locIcon := getFavoritesIcon(locName)
		f.favorites = append(f.favorites,
			favoriteItem{locName: locName, locIcon: locIcon, loc: loc})
	}
}

func (f *fileDialog) refreshDir(dir bellui.ListableURI) {
	f.data = nil

	files, err := dir.List()
	if err != nil {
		bellui.LogError("Unable to read ListableURI "+dir.String(), err)
		return
	}

	var icons []bellui.URI
	parent, err := storage.Parent(dir)
	if err != nil && err != repository.ErrURIRoot {
		bellui.LogError("Unable to get parent of "+dir.String(), err)
		return
	}
	if parent != nil && parent.String() != dir.String() {
		icons = append(icons, parent)
	}

	for _, file := range files {
		if !f.showHidden && storage.IsHidden(file) {
			continue
		}

		listable, err := storage.ListerForURI(file)
		if f.file.isDirectory() && err != nil {
			continue
		} else if err == nil { // URI points to a directory
			icons = append(icons, listable)
		} else if f.file.filter == nil || f.file.filter.Matches(file) {
			icons = append(icons, file)
		}
	}

	toSort := icons
	if parent != nil {
		toSort = icons[1:]
	}
	sort.Slice(toSort, func(i, j int) bool {
		if parent != nil { // avoiding the parent in [0]
			i++
			j++
		}

		return strings.ToLower(icons[i].Name()) < strings.ToLower(icons[j].Name())
	})
	f.data = icons
	f.updateFilter()

	f.files.Refresh()
	f.filesScroll.Offset = geo.NewPos(0, 0)
	f.filesScroll.Refresh()
}

func (f *fileDialog) setLocation(dir bellui.URI) error {
	if dir == nil {
		return errors.New("failed to open nil directory")
	}

	if f.selectedID > -1 {
		f.files.Unselect(f.selectedID)
	}

	list, err := storage.ListerForURI(dir)
	if err != nil {
		return err
	}

	bellui.SetPref(bellui.CurrentApp().Preferences(), lastFolderKey, dir.String())
	isFav := false
	for i, fav := range f.favorites {
		if storage.EqualURI(fav.loc, dir) {
			f.favoritesList.Select(i)
			isFav = true
			break
		}
	}
	if !isFav {
		f.favoritesList.UnselectAll()
	}

	f.setSelected(nil, -1)
	f.dir = list

	// Build breadcrumb chips
	var crumbs []core.CanvasObject
	for parent := dir; parent != nil && err == nil; parent, err = storage.Parent(parent) {
		currentParent := parent
		chip := widget.NewChip(currentParent.Name(), func() {
			err := f.setLocation(currentParent)
			if err != nil {
				bellui.LogError("Failed to set directory", err)
			}
		})
		chip.Style = widget.ChipStyleSuggestion
		crumbs = append(crumbs, chip)
	}

	// Reverse to root-first order
	for i, j := 0, len(crumbs)-1; i < j; i, j = i+1, j-1 {
		crumbs[i], crumbs[j] = crumbs[j], crumbs[i]
	}

	// Interleave with chevron separators
	th := bellui.CurrentApp().Settings().Theme()
	variant := bellui.CurrentApp().Settings().ThemeVariant()
	f.breadcrumb.Objects = nil
	for i, c := range crumbs {
		if i > 0 {
			sep := canvas.NewText("›", th.Color(theme.ColorNameOnSurfaceVariant, variant))
			sep.TextSize = th.Size(theme.SizeNameSubHeadingText)
			f.breadcrumb.Add(sep)
		}
		f.breadcrumb.Add(c)
	}

	f.breadcrumbScroll.Refresh()
	f.breadcrumbScroll.Offset.X = f.breadcrumbScroll.Content.Size().Width - f.breadcrumbScroll.Size().Width
	f.breadcrumbScroll.Refresh()

	if f.file.isDirectory() {
		f.fileName.SetText(dir.Name())
		f.open.Enable()
	}
	f.refreshDir(list)

	return nil
}

func (f *fileDialog) setSelected(file bellui.URI, id int) {
	if file != nil {
		if listable, err := storage.CanList(file); err == nil && listable {
			f.setLocation(file)
			return
		}
	}
	f.selected = file
	f.selectedID = id

	if file == nil || file.Path() == "" {
		// keep user input while navigating
		// in a FileSave dialog
		if !f.file.save {
			f.fileName.SetText("")
			f.open.Disable()
		}
	} else {
		f.fileName.SetText(file.Name())
		f.open.Enable()
	}
}

// isSelected reports whether uri is in the multi-select set.
func (f *fileDialog) isSelected(uri bellui.URI) bool {
	for _, u := range f.multiSelected {
		if storage.EqualURI(u, uri) {
			return true
		}
	}
	return false
}

// currentModifiers reads the live keyboard modifier state from the driver so a
// tap can tell a plain click from Ctrl/Cmd-click (toggle) or Shift-click (range).
func (f *fileDialog) currentModifiers() event.KeyModifier {
	if drv, ok := bellui.CurrentApp().Driver().(desktop.KeyModifierDriver); ok {
		return drv.CurrentKeyModifiers()
	}
	return 0
}

// toggleMultiSelect handles a tap on item id while the dialog is in
// multi-select mode: a directory navigates as usual, otherwise the file is
// added to / removed from / made the whole selection depending on modifiers.
func (f *fileDialog) toggleMultiSelect(id int) {
	f.applyMultiTap(id, f.currentModifiers())
}

// applyMultiTap is the modifier-driven multi-select core, split out from
// toggleMultiSelect so tests can drive it without a live driver.
func (f *fileDialog) applyMultiTap(id int, mods event.KeyModifier) {
	file, ok := f.getDisplayItem(id)
	if !ok {
		return
	}
	if listable, err := storage.CanList(file); err == nil && listable {
		f.setLocation(file) // navigate into folders / parent
		return
	}

	toggle := mods&(event.KeyModifierControl|event.KeyModifierSuper) != 0
	rangeSel := mods&event.KeyModifierShift != 0

	switch {
	case rangeSel && f.multiAnchor >= 0:
		f.selectRange(f.multiAnchor, id)
	case toggle:
		if f.isSelected(file) {
			f.removeSelected(file)
		} else {
			f.multiSelected = append(f.multiSelected, file)
		}
		f.multiAnchor = id
	default:
		f.multiSelected = []bellui.URI{file}
		f.multiAnchor = id
	}

	f.selectedID = id
	f.applyMultiSelection()
}

// selectRange replaces the selection with every selectable (non-directory)
// file between the two display ids inclusive.
func (f *fileDialog) selectRange(from, to int) {
	if from > to {
		from, to = to, from
	}
	f.multiSelected = nil
	for id := from; id <= to; id++ {
		file, ok := f.getDisplayItem(id)
		if !ok {
			continue
		}
		if listable, err := storage.CanList(file); err == nil && listable {
			continue // directories are not part of a file selection
		}
		f.multiSelected = append(f.multiSelected, file)
	}
}

// removeSelected drops uri from the multi-select set (in place).
func (f *fileDialog) removeSelected(uri bellui.URI) {
	out := f.multiSelected[:0]
	for _, u := range f.multiSelected {
		if !storage.EqualURI(u, uri) {
			out = append(out, u)
		}
	}
	f.multiSelected = out
}

// applyMultiSelection reflects the current selection in the filename field, the
// OK button state and the file grid (so each item repaints its highlight).
func (f *fileDialog) applyMultiSelection() {
	switch len(f.multiSelected) {
	case 0:
		f.fileName.SetText("")
		f.open.Disable()
	case 1:
		f.fileName.SetText(f.multiSelected[0].Name())
		f.open.Enable()
	default:
		f.fileName.SetText(lang.LocalizePluralKey("file.selectedCount", "{{.Count}} files selected",
			len(f.multiSelected), map[string]any{"Count": len(f.multiSelected)}))
		f.open.Enable()
	}
	if f.files != nil {
		f.files.Refresh()
	}
}

func (f *fileDialog) setView(view ViewLayout) {
	f.view = view
	bellui.SetPref(bellui.CurrentApp().Preferences(), viewLayoutKey, int(view))
	var selectF func(id int)
	choose := func(id int) {
		if file, ok := f.getDisplayItem(id); ok {
			f.selectedID = id
			f.setSelected(file, id)
		}
	}
	count := func() int {
		return f.displayCount()
	}
	template := func() core.CanvasObject {
		return f.newFileItem(storage.NewFileURI("./tempfile"), true, false)
	}
	update := func(id widget.GridWrapItemID, o core.CanvasObject) {
		if dir, ok := f.getDisplayItem(id); ok {
			parent := id == 0 && f.filterQuery == "" && len(dir.Path()) < len(f.dir.Path())
			_, isDir := dir.(bellui.ListableURI)
			item := o.(*fileDialogItem)
			item.setLocation(dir, isDir || parent, parent)
			item.choose = selectF
			item.id = id
			item.open = f.open.OnTapped
			item.selected = f.file.multiSelect && f.isSelected(dir)
		}
	}
	// In multi-select mode the items manage their own highlight via the
	// selection set, so the grid's single-selection is bypassed and taps route
	// straight to toggleMultiSelect.
	if f.view == GridView {
		grid := widget.NewGridWrap(count, template, update)
		if !f.file.multiSelect {
			grid.OnSelected = choose
		}
		f.files = grid
		f.toggleViewButton.Icon = theme.ListIcon()
		f.toggleViewButton.Refresh()
		selectF = grid.Select
	} else {
		list := widget.NewList(count, template, update)
		if !f.file.multiSelect {
			list.OnSelected = choose
		}
		f.files = list
		f.toggleViewButton.Icon = theme.GridIcon()
		f.toggleViewButton.Refresh()
		selectF = list.Select
	}
	if f.file.multiSelect {
		selectF = f.toggleMultiSelect
	}

	if f.dir != nil {
		f.refreshDir(f.dir)
	}
	f.filesScroll.Content = container.NewPadded(f.files)
	f.filesScroll.Refresh()
}

func (f *fileDialog) getDataItem(id int) (bellui.URI, bool) {
	if id >= len(f.data) {
		return nil, false
	}

	return f.data[id], true
}

// effectiveStartingDir calculates the directory at which the file dialog should
// open, based on the values of startingDirectory, CWD, home, and any error
// conditions which occur.
//
// Order of precedence is:
//
//   - file.startingDirectory if non-empty, os.Stat()-able, and uses the file://
//     URI scheme
//   - previously used file open/close folder within this app
//   - the current app's document storage, if App.Storage() documents have been saved
//   - os.UserHomeDir()
//   - os.Getwd()
//   - "/" (should be filesystem root on all supported platforms)
func (f *FileDialog) effectiveStartingDir() bellui.ListableURI {
	if f.startingLocation != nil {
		if f.startingLocation.Scheme() == "file" {
			path := f.startingLocation.Path()

			// the starting directory is set explicitly
			if _, err := os.Stat(path); err != nil {
				bellui.LogError("Error with StartingLocation", err)
			} else {
				return f.startingLocation
			}
		}
		return f.startingLocation
	}

	// last used
	lastPath := bellui.PrefOr(bellui.CurrentApp().Preferences(), lastFolderKey, "")
	if lastPath != "" {
		parsed, err := storage.ParseURI(lastPath)
		if err == nil {
			dir, err := storage.ListerForURI(parsed)
			if err == nil {
				return dir
			}
		}
	}

	// Try app storage
	app := bellui.CurrentApp()
	if hasAppFiles(app) {
		list, _ := storage.ListerForURI(storageURI(app))
		return list
	}

	// Try home dir
	dir, err := os.UserHomeDir()
	if err == nil {
		lister, err := storage.ListerForURI(storage.NewFileURI(dir))
		if err == nil {
			return lister
		}
		bellui.LogError("Could not create lister for user home dir", err)
	}
	bellui.LogError("Could not load user home dir", err)

	// Try to get ./
	wd, err := os.Getwd()
	if err == nil {
		lister, err := storage.ListerForURI(storage.NewFileURI(wd))
		if err == nil {
			return lister
		}
		bellui.LogError("Could not create lister for working dir", err)
	}

	lister, err := storage.ListerForURI(storage.NewFileURI("/"))
	if err != nil {
		bellui.LogError("could not create lister for /", err)
		return nil
	}
	return lister
}

func showFile(file *FileDialog) *fileDialog {
	d := &fileDialog{file: file, initialFileName: file.initialFileName, showHidden: file.showHidden, view: GridView}
	ui := d.makeUI()
	pad := theme.Padding()
	itemMin := d.newFileItem(storage.NewFileURI("filename.txt"), false, false).MinSize()
	size := ui.MinSize().Add(itemMin.AddWidthHeight(itemMin.Width+pad*4, pad*2))
	d.startSize = size

	if d.hosted() {
		d.body = ui
	} else {
		d.win = wrapModal(ui, file.parent.Canvas(), file.windowed, func() {
			d.dismiss.OnTapped()
		})
		d.win.Resize(size)
	}

	d.setLocation(file.effectiveStartingDir())

	if d.hosted() {
		d.showHostWindow(size)
	} else {
		d.win.Show()
	}
	if file.save {
		d.canvas().Focus(d.fileName.(*widget.Entry))
	}
	return d
}

// Dismiss instructs the dialog to close without any affirmative action.
//
// Since: 2.6
func (f *FileDialog) Dismiss() {
	f.dialog.dismiss.OnTapped()
}

// MinSize returns the size that this dialog should not shrink below
//
// Since: 2.1
func (f *FileDialog) MinSize() geo.Size {
	return f.dialog.minSize()
}

// Show shows the file dialog.
func (f *FileDialog) Show() {
	if f.save {
		if fileSaveOSOverride(f) {
			return
		}
	} else {
		// Multi-select is honoured natively only where a backend supports it
		// (macOS NSOpenPanel today, #716); backends without it decline a
		// multi-select request and fall through to the canvas dialog (#627).
		if fileOpenOSOverride(f) {
			return
		}
	}
	if f.dialog != nil {
		f.dialog.show()
		return
	}
	f.dialog = showFile(f)
	if !f.desiredSize.IsZero() {
		f.Resize(f.desiredSize)
	}
}

// Refresh causes this dialog to be updated
func (f *FileDialog) Refresh() {
	f.dialog.refresh()
}

// Resize dialog to the requested size, if there is sufficient space.
// If the parent window is not large enough then the size will be reduced to fit.
func (f *FileDialog) Resize(size geo.Size) {
	f.desiredSize = size
	if f.dialog == nil {
		return
	}
	f.dialog.startSize = size
	f.dialog.resize(size)
}

// Hide hides the file dialog.
func (f *FileDialog) Hide() {
	if f.dialog == nil {
		return
	}
	f.dialog.hide()
	if f.onClosedCallback != nil {
		f.onClosedCallback(false)
	}
}

// SetConfirmText allows custom text to be set in the confirmation button
//
// Since: 2.2
func (f *FileDialog) SetConfirmText(label string) {
	f.confirmText = label
	if f.dialog == nil {
		return
	}
	f.dialog.open.SetText(label)
	f.dialog.refresh()
}

// SetDismissText allows custom text to be set in the dismiss button
func (f *FileDialog) SetDismissText(label string) {
	f.dismissText = label
	if f.dialog == nil {
		return
	}
	f.dialog.dismiss.SetText(label)
	f.dialog.refresh()
}

// SetTitleText allows custom text to be set in the dialog title
//
// Since: 2.6
func (f *FileDialog) SetTitleText(label string) {
	f.titleText = label
	if f.dialog == nil {
		return
	}
	f.dialog.title.SetText(label)
}

// SetLocation tells this FileDialog which location to display.
// This is normally called before the dialog is shown.
//
// Since: 1.4
func (f *FileDialog) SetLocation(u bellui.ListableURI) {
	f.startingLocation = u
	if f.dialog != nil {
		f.dialog.setLocation(u)
	}
}

// SetOnClosed sets a callback function that is called when
// the dialog is closed.
func (f *FileDialog) SetOnClosed(closed func()) {
	// If there is already a callback set, remember it and call both.
	originalCallback := f.onClosedCallback

	f.onClosedCallback = func(response bool) {
		if originalCallback != nil {
			originalCallback(response)
		}
		closed()
	}
}

// notifyClosed reports a native backend's close through the OnClosed callback,
// mirroring the canvas dialog: fired immediately before the result callback,
// true only when the user confirmed a choice. A native picker never creates
// the canvas fileDialog, whose widgets otherwise make this report, so the
// backends call it from the same main-goroutine hop that delivers the result.
func (f *FileDialog) notifyClosed(confirmed bool) {
	if f.onClosedCallback != nil {
		f.onClosedCallback(confirmed)
	}
}

// SetFilter sets a filter for limiting files that can be chosen in the file dialog.
func (f *FileDialog) SetFilter(filter storage.FileFilter) {
	if f.isDirectory() {
		bellui.LogError("Cannot set a filter for a folder dialog", nil)
		return
	}
	f.filter = filter
	if f.dialog != nil {
		f.dialog.refreshDir(f.dialog.dir)
	}
}

// SetMultiSelect enables or disables choosing more than one file. It only
// applies to an open dialog (not save or folder). When enabled, use
// [NewFileOpenMultiple]/[ShowFileOpenMultiple] or supply a
// func([]bellui.URIReadCloser, error) callback; the canvas dialog then lets the
// user extend the selection with Ctrl/Cmd-click (toggle) and Shift-click
// (range). Call before the dialog is shown.
//
// Since: 2.19
func (f *FileDialog) SetMultiSelect(multi bool) {
	if f.save || f.isDirectory() {
		bellui.LogError("Multi-select is only supported for file open dialogs", nil)
		return
	}
	f.multiSelect = multi
}

// SetFileName sets the filename in a FileDialog in save mode.
// This is normally called before the dialog is shown.
func (f *FileDialog) SetFileName(fileName string) {
	if f.save {
		f.initialFileName = fileName
		// Update entry if fileDialog has already been created
		if f.dialog != nil {
			f.dialog.fileName.SetText(fileName)
		}
	}
}

// SetView changes the default display view of the FileDialog
// This is normally called before the dialog is shown.
//
// Since: 2.5
func (f *FileDialog) SetView(v ViewLayout) {
	f.initialView = v
	if f.dialog != nil {
		f.dialog.setView(v)
	}
}

// SetShowHidden sets whether the dialog should display hidden files.
// This is normally called before the dialog is shown.
//
// Since: 2.7
func (f *FileDialog) SetShowHidden(show bool) {
	f.showHidden = show
	if f.dialog != nil {
		f.dialog.showHidden = show
		f.dialog.refreshDir(f.dialog.dir)
	}
}

// NewFileOpen creates a file dialog allowing the user to choose a file to open.
//
// The callback function will run when the dialog closes and provide a reader for the chosen file.
// The reader will be nil when the user cancels or when nothing is selected.
// When the reader isn't nil it must be closed by the callback.
//
// The dialog will appear over the window specified when Show() is called.
func NewFileOpen(callback func(reader bellui.URIReadCloser, err error), parent bellui.Window) *FileDialog {
	dialog := &FileDialog{callback: callback, parent: parent, windowed: true}
	return dialog
}

// NewFileSave creates a file dialog allowing the user to choose a file to save
// to (new or overwrite). If the user chooses an existing file they will be
// asked if they are sure.
//
// The callback function will run when the dialog closes and provide a writer for the chosen file.
// The writer will be nil when the user cancels or when nothing is selected.
// When the writer isn't nil it must be closed by the callback.
//
// The dialog will appear over the window specified when Show() is called.
func NewFileSave(callback func(writer bellui.URIWriteCloser, err error), parent bellui.Window) *FileDialog {
	dialog := &FileDialog{callback: callback, parent: parent, save: true, windowed: true}
	return dialog
}

// ShowFileOpen creates and shows a file dialog allowing the user to choose a
// file to open.
//
// The callback function will run when the dialog closes and provide a reader for the chosen file.
// The reader will be nil when the user cancels or when nothing is selected.
// When the reader isn't nil it must be closed by the callback.
//
// The dialog will appear over the window specified.
func ShowFileOpen(callback func(reader bellui.URIReadCloser, err error), parent bellui.Window) {
	dialog := NewFileOpen(callback, parent)
	if fileOpenOSOverride(dialog) {
		return
	}
	dialog.Show()
}

// NewFileOpenMultiple creates a file dialog allowing the user to choose one or
// more files to open. It is the multi-select counterpart to [NewFileOpen]; the
// callback receives every chosen file (or a nil slice when the user cancels).
// Each non-nil reader must be closed by the callback.
//
// Multi-select is only available in the canvas-drawn dialog for now, so a
// multi-select open dialog always uses it even when [UseNativeFileDialogs] is
// enabled (native multi-select is a follow-up).
//
// The dialog will appear over the window specified when Show() is called.
//
// Since: 2.19
func NewFileOpenMultiple(callback func(readers []bellui.URIReadCloser, err error), parent bellui.Window) *FileDialog {
	return &FileDialog{callback: callback, parent: parent, windowed: true, multiSelect: true}
}

// ShowFileOpenMultiple creates and shows a file dialog allowing the user to
// choose one or more files to open.
//
// The dialog will appear over the window specified.
//
// Since: 2.19
func ShowFileOpenMultiple(callback func(readers []bellui.URIReadCloser, err error), parent bellui.Window) {
	NewFileOpenMultiple(callback, parent).Show()
}

// ShowFileSave creates and shows a file dialog allowing the user to choose a
// file to save to (new or overwrite). If the user chooses an existing file they
// will be asked if they are sure.
//
// The callback function will run when the dialog closes and provide a writer for the chosen file.
// The writer will be nil when the user cancels or when nothing is selected.
// When the writer isn't nil it must be closed by the callback.
//
// The dialog will appear over the window specified.
func ShowFileSave(callback func(writer bellui.URIWriteCloser, err error), parent bellui.Window) {
	dialog := NewFileSave(callback, parent)
	if fileSaveOSOverride(dialog) {
		return
	}
	dialog.Show()
}

func getFavoritesIcon(location string) bellui.Resource {
	switch location {
	case "Documents":
		return theme.DocumentIcon()
	case "Desktop":
		return theme.DesktopIcon()
	case "Downloads":
		return theme.DownloadIcon()
	case "Music":
		return theme.MediaMusicIcon()
	case "Pictures":
		return theme.MediaPhotoIcon()
	case "Videos":
		return theme.MediaVideoIcon()
	}

	if (runtime.GOOS == "darwin" && location == "Movies") ||
		(runtime.GOOS != "darwin" && location == "Videos") {
		return theme.MediaVideoIcon()
	}

	return nil
}

func getFavoritesOrder() [6]string {
	order := [6]string{
		"Desktop",
		"Documents",
		"Downloads",
		"Music",
		"Pictures",
		"Videos",
	}

	if runtime.GOOS == "darwin" {
		order[5] = "Movies"
	}

	return order
}

func hasAppFiles(a bellui.App) bool {
	if a.UniqueID() == "testApp" {
		return false
	}

	return len(a.Storage().List()) > 0
}

func storageURI(a bellui.App) bellui.URI {
	dir, _ := storage.Child(a.Storage().RootURI(), "Documents")
	return dir
}

// iconPaddingLayout adds padding to the left of a widget.Icon().
// NOTE: It assumes that the slice only contains one item.
type iconPaddingLayout struct{}

func (i *iconPaddingLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	padding := theme.Padding() * 2
	objects[0].Move(geo.NewPos(padding, 0))
	objects[0].Resize(size.SubtractWidthHeight(padding, 0))
}

func (i *iconPaddingLayout) MinSize(objects []core.CanvasObject) geo.Size {
	return objects[0].MinSize().AddWidthHeight(theme.Padding()*2, 0)
}

// === Search filtering ===

func (f *fileDialog) toggleSearch() {
	f.searchActive = !f.searchActive
	if f.searchActive {
		f.searchContainer.Show()
	} else {
		f.searchContainer.Hide()
		f.filterQuery = ""
		f.searchField.SetText("")
		f.updateFilter()
		f.files.Refresh()
		f.filesScroll.Refresh()
	}
}

func (f *fileDialog) updateFilter() {
	if f.filterQuery == "" {
		f.filteredData = nil
		return
	}
	query := strings.ToLower(f.filterQuery)
	f.filteredData = nil
	for _, uri := range f.data {
		if strings.Contains(strings.ToLower(uri.Name()), query) {
			f.filteredData = append(f.filteredData, uri)
		}
	}
}

func (f *fileDialog) displayCount() int {
	if f.filterQuery == "" {
		return len(f.data)
	}
	return len(f.filteredData)
}

func (f *fileDialog) getDisplayItem(id int) (bellui.URI, bool) {
	if f.filterQuery == "" {
		return f.getDataItem(id)
	}
	if id >= len(f.filteredData) {
		return nil, false
	}
	return f.filteredData[id], true
}

// === Layout helpers ===

// headerBarLayout lays out [bg, divider, title, actions] as a TopAppBar-style row.
type headerBarLayout struct{}

func (l *headerBarLayout) MinSize(objects []core.CanvasObject) geo.Size {
	th := bellui.CurrentApp().Settings().Theme()
	return geo.NewSize(200, th.Size(theme.SizeNameTopBarHeight))
}

func (l *headerBarLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	pad := theme.Padding() * 3
	// bg
	objects[0].Resize(size)
	// divider (1px at bottom)
	objects[1].Move(geo.NewPos(0, size.Height-1))
	objects[1].Resize(geo.NewSize(size.Width, 1))
	// title
	titleMin := objects[2].MinSize()
	objects[2].Move(geo.NewPos(pad, (size.Height-titleMin.Height)/2))
	objects[2].Resize(titleMin)
	// actions
	actionsMin := objects[3].MinSize()
	objects[3].Move(geo.NewPos(size.Width-actionsMin.Width-pad/2, (size.Height-actionsMin.Height)/2))
	objects[3].Resize(actionsMin)
}

// footerBarLayout lays out [bg, divider, content] as a footer action bar.
type footerBarLayout struct{}

func (l *footerBarLayout) MinSize(objects []core.CanvasObject) geo.Size {
	contentMin := objects[2].MinSize()
	pad := theme.Padding() * 3
	return geo.NewSize(contentMin.Width+pad*2, contentMin.Height+pad*2)
}

func (l *footerBarLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	pad := theme.Padding() * 3
	// bg
	objects[0].Resize(size)
	// divider (1px at top)
	objects[1].Resize(geo.NewSize(size.Width, 1))
	// content
	objects[2].Move(geo.NewPos(pad, pad))
	objects[2].Resize(geo.NewSize(size.Width-pad*2, size.Height-pad*2))
}

// fixedWidthLayout constrains all children to a fixed width.
type fixedWidthLayout struct {
	width float32
}

func (l *fixedWidthLayout) MinSize(objects []core.CanvasObject) geo.Size {
	h := float32(0)
	for _, o := range objects {
		if o.Visible() {
			h = geo.Max(h, o.MinSize().Height)
		}
	}
	return geo.NewSize(l.width, h)
}

func (l *fixedWidthLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	for _, o := range objects {
		o.Resize(size)
	}
}

// SetWindowed controls whether the dialog is hosted in a window frame that
// the user can drag by its title bar and resize from its corners. Windowed
// is the default; pass false for a fixed, centered dialog. Call before Show.
func (f *FileDialog) SetWindowed(windowed bool) {
	f.windowed = windowed
}

// SetWindowHosted controls whether Show opens this file dialog in its own
// native window instead of an overlay inside the parent window's canvas. While
// the dialog window is open the parent window is blocked by a modal scrim;
// other application windows stay interactive (window-modal). Closing the
// dialog window — its close button or the Escape key — dismisses the dialog as
// if Cancel were tapped. On single-window platforms (mobile, web) the mode is
// ignored and the dialog falls back to the standard overlay presentation.
// Call before Show.
//
// Since: 2.18
func (f *FileDialog) SetWindowHosted(hosted bool) {
	f.windowHosted = hosted
}
