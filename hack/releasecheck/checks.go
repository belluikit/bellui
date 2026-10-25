package main

import (
	"fmt"
	"slices"
	"strings"
)

// version is the release these checks validate. A later release copies this
// file, keeps the checks that still apply and replaces the ones that named
// this release's changes.
const version = "v1.3.0"

// Where the checks run. "Gallery" is github.com/belluikit/gallery built from
// this checkout; "Companion" is ./app. Both are built by `setup`.
const (
	gallery   = "Gallery"
	companion = "Companion app"
)

type check struct {
	ID        string
	Area      string
	Title     string
	Platforms []string // empty: every platform
	Refs      []string // issues and PRs whose change this check covers
	Special   bool     // an edge case or one of the release's riskiest changes
	Where     string   // which app, and how to get to the screen
	Setup     string   // what to do before the steps, if anything
	Steps     []string
	Expect    []string
	// Per lists platform-specific instructions, shown only on that platform.
	Per map[string]string
}

func (c check) on(platform string) bool {
	return len(c.Platforms) == 0 || slices.Contains(c.Platforms, platform)
}

var areaTitle = map[string]string{
	"smoke":  "Smoke: every page, theme and window",
	"touch":  "Touch and keys",
	"i18n":   "Internationalisation",
	"rtl":    "Right-to-left",
	"a11y":   "Accessibility",
	"charts": "x/charts",
	"tables": "x/tables",
	"dag":    "x/dag",
	"media":  "x/mmedia",
	"cli":    "Packaging and the bellui command",
	"edge":   "Edge cases",
}

// text is the check as the runner shows it on one platform.
func (c check) text(platform string) string {
	var b strings.Builder
	title := c.Title
	if c.Special {
		title = "SPECIAL — " + title
	}
	fmt.Fprintf(&b, "%s  [%s]", title, c.Area)
	if len(c.Refs) > 0 {
		fmt.Fprintf(&b, "  (%s)", strings.Join(c.Refs, ", "))
	}
	fmt.Fprintf(&b, "\nWhere: %s\n", c.Where)
	if c.Setup != "" {
		fmt.Fprintf(&b, "Before: %s\n", c.Setup)
	}
	if n, ok := c.Per[platform]; ok {
		fmt.Fprintf(&b, "On %s: %s\n", platform, n)
	}
	b.WriteString("Steps:\n")
	for i, s := range c.Steps {
		fmt.Fprintf(&b, "  %d. %s\n", i+1, s)
	}
	b.WriteString("Expected:\n")
	for _, e := range c.Expect {
		fmt.Fprintf(&b, "  - %s\n", e)
	}
	return b.String()
}

// markdown is the check for the printed checklist, with every platform note.
func (c check) markdown() string {
	var b strings.Builder
	mark := ""
	if c.Special {
		mark = " — **SPECIAL**"
	}
	fmt.Fprintf(&b, "### %s %s%s\n\n", c.ID, c.Title, mark)
	plats := "all"
	if len(c.Platforms) > 0 {
		plats = strings.Join(c.Platforms, ", ")
	}
	fmt.Fprintf(&b, "- [ ] Platforms: %s", plats)
	if len(c.Refs) > 0 {
		fmt.Fprintf(&b, " · Covers: %s", strings.Join(c.Refs, ", "))
	}
	fmt.Fprintf(&b, "\n\n**Where:** %s\n\n", c.Where)
	if c.Setup != "" {
		fmt.Fprintf(&b, "**Before:** %s\n\n", c.Setup)
	}
	for _, p := range platforms {
		if n, ok := c.Per[p]; ok {
			fmt.Fprintf(&b, "**On %s:** %s\n\n", p, n)
		}
	}
	b.WriteString("**Steps**\n\n")
	for i, s := range c.Steps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, s)
	}
	b.WriteString("\n**Expected**\n\n")
	for _, e := range c.Expect {
		fmt.Fprintf(&b, "- %s\n", e)
	}
	b.WriteString("\n")
	return b.String()
}

// Platform shorthands.
var (
	mobile  = []string{"android", "ios"}
	desktop = []string{"desktop"}
	keyed   = []string{"desktop", "web"}
	native  = []string{"desktop", "android", "ios"}
)

// localeHow says how to set the system language on each platform, for the
// checks that need the OS, not the app, to choose the locale.
var localeHow = map[string]string{
	"desktop": "Linux: start the app with LANG=<locale>.UTF-8 (for example LANG=de_DE.UTF-8 ./releasecheck-app). macOS: System Settings → General → Language & Region, then relaunch. Windows: Settings → Time & language → Language, then sign out and in.",
	"android": "Settings → System → Languages, put the language first, then reopen the app. On Android 13+ also try Settings → Apps → <app> → Language, with the app open.",
	"ios":     "Settings → General → Language & Region, or Settings → <app> → Language; reopen the app.",
	"web":     "Chrome/Edge: Settings → Languages, move the language to the top, then reload the page.",
}

var checks = []check{
	// ---- smoke ----
	{
		ID: "SM-01", Area: "smoke", Title: "Every gallery page opens",
		Where: gallery + ", navigation (rail on desktop, drawer on a phone)",
		Steps: []string{
			"Open every page in the navigation, top to bottom.",
			"On each, scroll to the bottom and back.",
		},
		Expect: []string{
			"Every page draws within about two seconds, with no blank or partly drawn area.",
			"No crash, no Go panic in the terminal, logcat or browser console.",
			"Desktop only pages (Clair, Code Assistant, Media Player) are listed on desktop and absent on Android and the web.",
		},
	},
	{
		ID: "SM-02", Area: "smoke", Title: "Light, dark and dense themes", Refs: []string{"#688", "#695"},
		Where: gallery + ", top app bar toggles, and the Theme page",
		Steps: []string{
			"Toggle light/dark in the app bar twice, then toggle dense twice.",
			"On the Theme page choose Light, quit and relaunch the app.",
			"On the Theme page choose System, then switch the OS (or browser) between light and dark.",
		},
		Expect: []string{
			"Each toggle repaints the whole window at once; no widget keeps the old colours.",
			"After the relaunch the gallery opens in Light, whatever the OS setting (#688).",
			"With System chosen, the gallery follows the OS switch within a second (#695).",
		},
		Per: map[string]string{"web": "The browser follows the OS's dark mode; DevTools → Rendering → Emulate prefers-color-scheme also works."},
	},
	{
		ID: "SM-03", Area: "smoke", Title: "Resize and rotate",
		Where: gallery + ", Responsive page, then any page",
		Steps: []string{
			"Desktop/web: drag the window from wide to as narrow as it goes, and back.",
			"Phone: rotate to landscape and back on the Responsive page and on Charts.",
		},
		Expect: []string{
			"The layout switches between its wide and compact forms; nothing overlaps or is cut off.",
			"After rotating back, the page is exactly as it was.",
		},
	},
	{
		ID: "SM-04", Area: "smoke", Title: "Partial repaint leaves no stale pixels on phones", Platforms: mobile, Special: true,
		Refs:  []string{"#593", "#619"},
		Where: gallery + ", Communication page (progress, snackbar), Scrolling page",
		Steps: []string{
			"Watch the progress indicators animate for ten seconds.",
			"Show a snackbar and let it disappear; show a toast.",
			"Fling the Scrolling page's list up and down; toggle light/dark mid-fling.",
			"If anything looks wrong, rebuild the gallery with -tags nopartialrepaint and repeat, to tell a partial-repaint fault from any other.",
		},
		Expect: []string{
			"No ghost of a snackbar, toast, ripple or old scroll position stays on screen.",
			"Partial repaint is on by default on phones in this release; any difference from the nopartialrepaint build is a failure.",
		},
	},
	{
		ID: "SM-05", Area: "smoke", Title: "The web build draws with WebGPU", Platforms: []string{"web"}, Refs: []string{"#333"}, Special: true,
		Where: gallery + " served with `bellui serve`, in Chrome or Edge at http://localhost:8080",
		Steps: []string{
			"Load the page and wait for the .wasm to finish downloading (about 64 MB).",
			"Open DevTools → Console.",
			"Open Charts → 3D, then Theme.",
		},
		Expect: []string{
			"The gallery draws; in v1.2.0 a WebGPU build showed a blank page (#333).",
			"No uncaught error in the console.",
			"3D charts draw and respond to dragging.",
		},
	},

	{
		ID: "SM-06", Area: "smoke", Title: "Collections inside a theme override", Refs: []string{"#755", "#758"}, Special: true,
		Where: companion + " → Theme override (yellow text, orange lines and accents)",
		Steps: []string{
			"On List, Table and Tree: scroll to the end and back, resize the window, toggle dark mode on the Direction tab and come back.",
			"Hover and select a few rows.",
		},
		Expect: []string{
			"Every row's text stays yellow and every separator and header line orange — also rows that scrolled into view and after the resize and theme change.",
			"Hover and selection use teal.",
			"No row, separator or header ever shows the app theme's colours.",
		},
	},

	// ---- touch and keys ----
	{
		ID: "TK-01", Area: "touch", Title: "A slider stops where the finger lifts", Platforms: mobile, Refs: []string{"#751"}, Special: true,
		Where: companion + " → Touch & keys (and " + gallery + " → Inputs → Slider)",
		Steps: []string{
			"Drag the slider thumb quickly from left to right and lift the finger mid-way while still moving.",
			"Read \"Slider value\" right after lifting, then again two seconds later.",
		},
		Expect: []string{
			"The thumb stops exactly where the finger lifted: it does not glide on.",
			"The value does not change after the finger is up.",
		},
	},
	{
		ID: "TK-02", Area: "touch", Title: "Lists fling, and a held finger does not", Platforms: mobile, Refs: []string{"#751"},
		Where: companion + " → Touch & keys → \"Flick this list\"",
		Steps: []string{
			"Flick the list up quickly and let go.",
			"While it is still moving, touch it once.",
			"Drag the list, hold the finger still for half a second, then lift.",
		},
		Expect: []string{
			"After the flick the list keeps scrolling and slows to a stop.",
			"The touch stops it at once, and the next drag starts from there (#751).",
			"After holding still, the list does not move on when the finger lifts.",
		},
	},
	{
		ID: "TK-03", Area: "touch", Title: "Carousel follows the finger and pages on a flick", Refs: []string{"#686", "#688", "#761"},
		Where: companion + " → Touch & keys → carousel",
		Per:   map[string]string{"desktop": "Use the mouse: press, drag sideways, release.", "web": "Use the mouse: press, drag sideways, release."},
		Steps: []string{
			"Drag page 1 slowly a quarter of its width to the left and let go.",
			"Flick it quickly to the left.",
			"Press \"Add a carousel page\" twice, then page through to the last page.",
			"At the last page, drag further left.",
		},
		Expect: []string{
			"While dragging, the page moves with the finger; the slow short drag snaps back to page 1.",
			"The flick moves to page 2 and the label says \"Carousel page 2 of 3\".",
			"The added pages can be reached: the label reaches \"page 5 of 5\" (#761).",
			"Past the last page it resists and springs back; it does not wrap round.",
		},
	},
	{
		ID: "TK-04", Area: "touch", Title: "A tap on a chart or the DAG canvas does not open the keyboard", Platforms: mobile, Refs: []string{"#415"},
		Where: gallery + " → Charts (any tab) and DAG Editor",
		Steps: []string{
			"Tap a chart's plot a few times; pan it.",
			"Tap the DAG Editor's empty canvas and a node.",
		},
		Expect: []string{"The on-screen keyboard never appears."},
	},
	{
		ID: "TK-05", Area: "touch", Title: "The keyboard's Return finishes a chart text label", Platforms: mobile, Refs: []string{"#666"}, Special: true,
		Where: gallery + " → Charts → Trading → Drawing Tools",
		Steps: []string{
			"Choose the Text tool and tap the chart.",
			"Type \"hi\" on the on-screen keyboard and press its Return key.",
		},
		Expect: []string{
			"The keyboard opens when the Text tool waits for text.",
			"Return finishes the label: \"hi\" stays on the chart and the keyboard closes (#666).",
		},
	},
	{
		ID: "TK-06", Area: "touch", Title: "A hardware Return reaches a widget that takes keys but no text", Platforms: native, Refs: []string{"#666"},
		Where: companion + " → Touch & keys → Key target",
		Per: map[string]string{
			"android": "With no keyboard attached, send the keys with: adb shell input keyevent 66 (Return), then adb shell input keyevent 21 (Left).",
			"ios":     "Attach a Bluetooth keyboard.",
		},
		Steps: []string{
			"Tap or click the Key target card.",
			"Press Return, then the left arrow, then type the letter a.",
		},
		Expect: []string{
			"The card says \"focused\" after the tap.",
			"It then shows \"key Return\", \"key Left\" and \"rune 'a'\" in turn.",
		},
	},
	{
		ID: "TK-07", Area: "touch", Title: "Tab order and focus visibility", Platforms: keyed,
		Where: gallery + " → Inputs",
		Steps: []string{
			"Click an empty spot, then press Tab repeatedly through the page, then Shift+Tab back.",
			"Do the same in dark mode.",
		},
		Expect: []string{
			"Focus moves through every control in reading order, and back the same way.",
			"The focused control always has a visible focus indicator, in both themes.",
		},
	},
	{
		ID: "TK-08", Area: "touch", Title: "Leaving a widget that keeps Tab", Platforms: keyed, Refs: []string{"#340", "#357"},
		Where: gallery + " → Scrolling, the multi-line entry",
		Steps: []string{
			"Click into the multi-line entry and press Tab.",
			"Desktop: press Ctrl+Tab. Then click back in and, on desktop and web, press Escape, then Tab.",
		},
		Expect: []string{
			"Tab inside the entry inserts a tab and keeps the focus there.",
			"Ctrl+Tab (desktop) and Escape then Tab (desktop and web) each move the focus out of the entry to the next control.",
		},
	},
	{
		ID: "TK-09", Area: "touch", Title: "Editor shortcuts on Linux and Windows", Platforms: desktop, Refs: []string{"#691"},
		Where: gallery + " → Code Editor",
		Steps: []string{"Click into the editor, press Ctrl+F, type a word that appears, press Enter, then Ctrl+S."},
		Expect: []string{
			"Ctrl+F opens find and Enter jumps to the next match.",
			"Ctrl+S sets the status to \"Saved (demo — no actual file written)\". On macOS the same works with Cmd.",
		},
	},

	// ---- i18n ----
	{
		ID: "I18N-01", Area: "i18n", Title: "Switching the locale at run time", Refs: []string{"#87"},
		Where: companion + " → Locale",
		Steps: []string{
			"Press en-US, then de-DE, then ja-JP, then System.",
		},
		Expect: []string{
			"en-US: Cancel; date 03/09/2026; number 1,234,567.89; currency €1,234.50; list \"red, green, and blue\".",
			"de-DE: Abbrechen; date 09.03.2026; number 1.234.567,89; currency 1.234,50 €; list \"red, green und blue\".",
			"ja-JP: キャンセル; date 2026/03/09; list \"red、green、blue\".",
			"Each press updates the text at once, with no restart. System returns to the OS language.",
		},
	},
	{
		ID: "I18N-02", Area: "i18n", Title: "The OS language is picked up at start", Special: true,
		Where: companion + " → Locale, and " + gallery + " → Communication → Confirm Dialog",
		Per:   localeHow,
		Setup: "Set the system (or browser) language to German.",
		Steps: []string{
			"Open the companion app; read the Locale tab without pressing a locale button.",
			"Press \"Open a confirm dialog\".",
		},
		Expect: []string{
			"Locale reads de (or de-DE); \"Cancel\" reads Abbrechen; date 09.03.2026.",
			"The dialog's buttons read Ja and Nein.",
		},
	},
	{
		ID: "I18N-03", Area: "i18n", Title: "Persian users of an app that ships Arabic get English", Refs: []string{"#342"}, Special: true,
		Where: companion + " → Locale (the app ships English and Arabic for its greeting, not Persian)",
		Steps: []string{"Press fa-IR, then ar-EG."},
		Expect: []string{
			"fa-IR: Right-to-left true; App greeting \"Hello Ada\" — English, not Arabic (#342); date 2026-03-09.",
			"ar-EG: Right-to-left true; App greeting \"مرحبا Ada\"; date 09/03/2026.",
			"The whole window lays out right-to-left for both.",
		},
	},
	{
		ID: "I18N-04", Area: "i18n", Title: "Languages with no catalog fall back to English, not a neighbour", Refs: []string{"#342", "#688"},
		Where: companion + " → Locale",
		Steps: []string{"Press sr-RS, he-IL, ur-PK and pt-BR in turn."},
		Expect: []string{
			"sr-RS: \"Cancel\" in English — not Russian (#342); date 09.03.2026; currency 1.234,50 €.",
			"he-IL and ur-PK: English text, Right-to-left true.",
			"pt-BR: \"Cancelar\"; list \"red, green e blue\".",
		},
	},
	{
		ID: "I18N-05", Area: "i18n", Title: "Long translated text fits", Special: true,
		Where:  gallery + ", system language German",
		Per:    localeHow,
		Setup:  "Set the system (or browser) language to German.",
		Steps:  []string{"Open Communication and Auth; open each dialog the pages offer; open a file dialog where one is offered."},
		Expect: []string{"Buttons and labels show the whole German word or wrap; nothing is cut off mid-word or overlaps."},
	},
	{
		ID: "I18N-06", Area: "i18n", Title: "Per-app language change on Android 13+", Platforms: []string{"android"}, Refs: []string{"#288"},
		Where:  companion + " → Locale, with the app open",
		Steps:  []string{"Settings → Apps → Release Check → Language → Deutsch, then switch back to the app."},
		Expect: []string{"Without restarting, the Locale tab reads de and \"Cancel\" reads Abbrechen."},
	},

	// ---- rtl ----
	{
		ID: "RTL-01", Area: "rtl", Title: "The gallery in right-to-left", Refs: []string{"#749"},
		Where: gallery + ", app bar RTL toggle (⋯ icon)",
		Steps: []string{
			"Toggle RTL on. Visit Navigation, Inputs, Containment, Data Grid and Charts.",
			"Toggle RTL off.",
		},
		Expect: []string{
			"The navigation moves to the right; rows, forms and app bars mirror; text aligns to the right.",
			"Directional icons (back, forward, navigate) point the other way.",
			"Known, not a failure in this release (#779): Slider, ProgressBar, Split, Tree and Table column order keep their left-to-right shape.",
			"Toggling off restores the left-to-right layout exactly.",
		},
	},
	{
		ID: "RTL-02", Area: "rtl", Title: "Menus mirror", Refs: []string{"#769"}, Special: true,
		Where: companion + " → Menus, and the window's menu bar (desktop)",
		Steps: []string{
			"Open the pop-up menu and its Submenu; note the layout.",
			"Direction tab → \"Toggle the app's direction\"; back on Menus, open the pop-up menu and Submenu again.",
			"Move the window to the left edge of the screen and open the Submenu again.",
			"Desktop: open the Check menu in the menu bar, in both directions.",
		},
		Expect: []string{
			"Right-to-left: the check mark and icon are on the right, the label right-aligned, the shortcut on the left, the submenu arrow points left.",
			"The submenu opens to the left of its item; where there is no room on the left it opens to the right instead.",
			"Left-to-right is the mirror image.",
		},
		Per: map[string]string{"desktop": "On macOS the menu bar is the system's; check the pop-up menu only."},
	},
	{
		ID: "RTL-03", Area: "rtl", Title: "One window right-to-left in a left-to-right app", Refs: []string{"#749", "#769"},
		Where: companion + " → Direction → \"Toggle this window's direction\"",
		Steps: []string{
			"Make sure the app's direction is auto (press its toggle until the pop-up menu is left-to-right).",
			"Press \"Toggle this window's direction\", then open the pop-up menu on the Menus tab.",
		},
		Expect: []string{
			"The whole window lays out right-to-left while the app stays left-to-right.",
			"The pop-up menu opens mirrored, as in RTL-02.",
		},
	},
	{
		ID: "RTL-04", Area: "rtl", Title: "A right-to-left scope inside a left-to-right window", Refs: []string{"#696", "#749", "#760"}, Special: true,
		Where: companion + " → Direction, \"Right-to-left scope\" card",
		Steps: []string{
			"Press \"Report direction\".",
			"Click the outside entry, then press Tab through the scope's controls and its three buttons.",
			"Open the scope's Select.",
			"Press \"Toggle light / dark\" twice, then \"Report direction\" again.",
		},
		Expect: []string{
			"It reports \"DirectionOf(entry) = right-to-left\"; the outside entry stays left-to-right.",
			"Inside the scope, labels are on the right, and the buttons read First, Second, Third from right to left; Tab goes First → Second → Third (#749).",
			"The Select's list opens aligned to the right edge of the field.",
			"After the theme change the scope is still right-to-left (#696).",
		},
	},
	{
		ID: "RTL-05", Area: "rtl", Title: "Mixed-direction text", Special: true,
		Where: companion + " → Direction, the scope's Entry (\"abc 123 שלום\")",
		Steps: []string{
			"Click at the end of the text; press Home, End, and the arrow keys; select with Shift+arrows.",
			"Type \" xyz\" at the end, then some Hebrew or Arabic if the keyboard has it.",
		},
		Expect: []string{
			"The Latin and Hebrew runs stay in their own order; the caret moves logically through them without jumping out of the text.",
			"Selection highlights the characters it covers; typed text appears at the caret.",
		},
		Per: map[string]string{"android": "Use the on-screen keyboard's cursor keys if it has them; otherwise tap to place the caret.", "ios": "Tap to place the caret."},
	},
	{
		ID: "RTL-06", Area: "rtl", Title: "An RTL system language lays the app out right-to-left", Special: true,
		Where:  gallery,
		Per:    localeHow,
		Setup:  "Set the system (or browser) language to Arabic or Hebrew.",
		Steps:  []string{"Start the gallery without touching the RTL toggle; open Navigation and Inputs."},
		Expect: []string{"The gallery starts right-to-left, as RTL-01 with the toggle on; text without a translation stays English."},
	},

	// ---- a11y ----
	{
		ID: "A11Y-01", Area: "a11y", Title: "A screen reader reads the gallery", Platforms: native, Special: true,
		Where: gallery + " built with -tags accessibility (desktop: gallery-a11y)",
		Per: map[string]string{
			"desktop": "Linux: Orca (Super+Alt+S). macOS: VoiceOver (Cmd+F5). Windows: Narrator (Ctrl+Win+Enter).",
			"android": "TalkBack: Settings → Accessibility → TalkBack. Swipe right/left to move, double-tap to activate.",
			"ios":     "VoiceOver: Settings → Accessibility → VoiceOver. Known gaps in #592 (word selection, grid exploration) are not failures here.",
		},
		Steps: []string{
			"Move through the navigation items and open Actions.",
			"Move through the buttons; activate one.",
			"Open Inputs: move to a checkbox, a switch, the slider and an entry; change each.",
		},
		Expect: []string{
			"Each item is announced with its label and role (button, check box, slider, edit text).",
			"State changes are announced: checked/unchecked, the slider's new value, the entry's text.",
			"Activating with the screen reader's gesture or key does the same as a tap.",
		},
	},
	{
		ID: "A11Y-02", Area: "a11y", Title: "Announcements and dialogs", Platforms: native,
		Where: gallery + " (a11y build) → Communication",
		Steps: []string{"With the screen reader on, press Show Snackbar, then a toast button, then Confirm Dialog."},
		Expect: []string{
			"The snackbar and toast text is spoken without moving focus to it.",
			"The dialog's title, message and buttons are read, and focus stays inside the dialog until it closes.",
		},
	},
	{
		ID: "A11Y-03", Area: "a11y", Title: "Data grid cells and selection", Platforms: native, Refs: []string{"#758"},
		Where: gallery + " (a11y build) → Data Grid → Editing and Large Data",
		Steps: []string{"With the screen reader on, move into the grid, across a row and down a column; toggle a row's checkbox."},
		Expect: []string{
			"Each cell is read with its value and its row and column.",
			"The checkbox's selection state is announced.",
		},
	},
	{
		ID: "A11Y-04", Area: "a11y", Title: "Screen reader in right-to-left", Platforms: native, Special: true,
		Where:  companion + " (a11y build) → Direction",
		Steps:  []string{"With the screen reader on, move through the right-to-left scope's controls."},
		Expect: []string{"They are read in visual order, right to left: Entry, Select, Slider, Check, then First, Second, Third."},
	},
	{
		ID: "A11Y-05", Area: "a11y", Title: "Web: keyboard only", Platforms: []string{"web"},
		Where: gallery + " in the browser",
		Steps: []string{"Do TK-07 and TK-08 without a mouse."},
		Expect: []string{
			"Everything on Inputs and Actions can be reached and used with the keyboard.",
			"Known: the web build has no screen-reader bridge in this release; a screen reader there is not a test.",
		},
	},
	{
		ID: "A11Y-06", Area: "a11y", Title: "Contrast on a light theme", Refs: []string{"#343", "#784"},
		Where: gallery + " in light mode → Charts (Basic, Specialized, Hierarchical, Geo) and Communication",
		Steps: []string{"Read every legend, label, tooltip and the zoom slider with the theme set to light."},
		Expect: []string{
			"Legend text, band labels, gauge value, tree, graph, sankey and parallel labels are dark on the light background.",
			"Labels on coloured tiles (funnel, treemap, sunburst) are black or white, whichever reads on the tile.",
			"The zoom slider has no dark slab; annotation and text-drawing chips are light.",
		},
	},

	// ---- charts ----
	{
		ID: "CH-01", Area: "charts", Title: "Legend colours match the series", Refs: []string{"#343", "#403"},
		Where:  gallery + " → Charts → 3D → 3D Line, and Basic",
		Steps:  []string{"Compare each legend swatch with the colour of its series."},
		Expect: []string{"Every swatch is the colour its series is drawn in; in v1.2.0 the 3D Line legend showed palette colours instead."},
	},
	{
		ID: "CH-02", Area: "charts", Title: "Crosshair and zoom on candles", Refs: []string{"#343"}, Special: true,
		Where: gallery + " → Charts → Trading (Price multi-pane) and Financial → Candlestick",
		Steps: []string{
			"Hover (desktop/web) or press and hold (phone) over the candles.",
			"Zoom in with the wheel, a pinch or the zoom buttons until about a third of the candles show.",
		},
		Expect: []string{
			"A crosshair and an axis tooltip follow the pointer over the candles and the volume bars.",
			"Zoomed in, the candles and bars get wider; they do not stay thin with wide gaps.",
		},
	},
	{
		ID: "CH-03", Area: "charts", Title: "Drawing tools", Refs: []string{"#343"},
		Where: gallery + " → Charts → Trading → Drawing Tools",
		Steps: []string{
			"Draw two trend lines and a text label (on desktop: type, then Enter).",
			"With the pointer over a data point (tooltip showing), click one of the lines.",
			"Delete the selected drawing (Delete key or the page's control).",
		},
		Expect: []string{
			"Clicking a line selects it, even while the tooltip is showing.",
			"Deleting removes only the drawing that was selected.",
		},
	},
	{
		ID: "CH-04", Area: "charts", Title: "Drilldown and breadcrumbs", Refs: []string{"#343"},
		Where:  gallery + " → Charts → Drilldown",
		Steps:  []string{"Click a bar to drill in, zoom the child chart, then click the breadcrumb's first segment."},
		Expect: []string{"The child chart opens for the bar clicked; the breadcrumb returns to the top level, with the top level's own zoom."},
	},
	{
		ID: "CH-05", Area: "charts", Title: "Gauge, heatmap and choropleth", Refs: []string{"#343"},
		Where: gallery + " → Charts → Specialized (Gauge), Financial (Heatmap), Geo (Choropleth)",
		Steps: []string{
			"Look at the gauge.",
			"Look at the heatmap's edges.",
			"On the choropleth, hover or tap several countries, including small ones (Portugal, Uruguay, Malaysia).",
		},
		Expect: []string{
			"Gauge: every coloured range is visible, with the value drawn as a thin ring inside them, not painted over them.",
			"Heatmap: row labels on the left and column labels below the cells.",
			"Choropleth: every country with data shows its name and value, not only large ones.",
		},
	},
	{
		ID: "CH-06", Area: "charts", Title: "Toolbox and save as image", Refs: []string{"#663", "#691", "#717"},
		Where: gallery + " → Charts → Basic → Line + Overlays",
		Steps: []string{"Check the toolbox and the zoom buttons; use the toolbox's save-as-image and open the file."},
		Expect: []string{
			"The toolbox and the zoom buttons do not overlap the plot or each other.",
			"A save dialog opens; the saved PNG is the chart only, with no toolbox, zoom buttons or tooltip in it.",
		},
	},

	// ---- tables ----
	{
		ID: "TB-01", Area: "tables", Title: "Group aggregates and tree data", Refs: []string{"#689", "#691"},
		Where: gallery + " → Data Grid → Grouping, and Tree Data",
		Steps: []string{"Group by sector; collapse and expand a group. Open Tree Data and expand every level."},
		Expect: []string{
			"Group header rows show their aggregates (sum, average, …) aligned under their columns.",
			"Every tree row is shown; none is missing at any level.",
		},
	},
	{
		ID: "TB-02", Area: "tables", Title: "Large data, light and dark",
		Where:  gallery + " → Data Grid → Large Data",
		Steps:  []string{"Scroll fast to the end and back; resize the window; toggle dark mode; select rows with the checkboxes."},
		Expect: []string{"Scrolling stays smooth; after the resize and theme change every visible row, checkbox and header uses the current theme."},
	},

	// ---- dag ----
	{
		ID: "DG-01", Area: "dag", Title: "Properties panel and palette", Refs: []string{"#638", "#643", "#653"},
		Where: gallery + " → DAG Editor",
		Steps: []string{
			"Select one node, then another; type into a property field of the second.",
			"Press Note; then add a node from \"Add node\" while the note sits in the middle of the canvas.",
		},
		Expect: []string{
			"The panel always shows the node selected last, and typing keeps working.",
			"The new node is placed clear of the note, not under it.",
		},
	},

	// ---- media ----
	{
		ID: "MD-01", Area: "media", Title: "Picture and sound start together", Platforms: desktop, Refs: []string{"#787", "#504"}, Special: true,
		Where: gallery + " → Media Player (the bundled sample, or Open file… with a video that has speech)",
		Steps: []string{"Open the page with speakers on; watch the first two seconds closely. Seek forward and back."},
		Expect: []string{
			"The first frames do not run ahead of the sound; lips and voice line up from the start (#787).",
			"After a seek, picture and sound resume together within a moment.",
		},
	},
	{
		ID: "MD-02", Area: "media", Title: "No audio device", Platforms: desktop, Refs: []string{"#690"}, Special: true,
		Where:  gallery + " → Media Player",
		Setup:  "Disable every audio output: unplug or disable the sound device, or on Linux run without a sound server.",
		Steps:  []string{"Open the Media Player page."},
		Expect: []string{"The page shows \"Cannot play …\" with the audio error at once; the app does not hang or play silently."},
	},

	// ---- cli ----
	{
		ID: "CLI-01", Area: "cli", Title: "Android packaging", Platforms: []string{"android"}, Refs: []string{"#672", "#674"},
		Where:  "The gallery and companion builds from setup",
		Steps:  []string{"Run setup's packaging steps with the newest build-tools and SDK platform installed (including a minor platform such as android-36.1 if available)."},
		Expect: []string{"Both packages build, install and start; the newest build-tools are used (the command prints which)."},
		Per:    map[string]string{"android": "On Windows hosts too, if one is available (#674)."},
	},
	{
		ID: "CLI-02", Area: "cli", Title: "iOS packaging is validly signed", Platforms: []string{"ios"}, Refs: []string{"#677", "#672"},
		Where:  "The gallery build from setup",
		Steps:  []string{"Install the packaged app on a device: codesign --verify --deep --strict <App>.app first."},
		Expect: []string{"codesign reports no error; the app installs and launches with its icon."},
	},
	{
		ID: "CLI-03", Area: "cli", Title: "macOS Info.plist", Platforms: desktop, Refs: []string{"#361", "#672"},
		Where: "macOS only: package the companion app",
		Steps: []string{"In hack/releasecheck/app run: bellui package -os darwin, then plutil -lint \"Release Check.app/Contents/Info.plist\"."},
		Expect: []string{
			"plutil reports OK.",
			"The plist has no LSApplicationCategoryType key (the app sets no category) (#361).",
		},
	},
	{
		ID: "CLI-04", Area: "cli", Title: "bellui serve names the app", Platforms: []string{"web"}, Refs: []string{"#703", "#683"},
		Where:  "The terminal running bellui serve",
		Steps:  []string{"Read the line bellui serve prints when it starts, for the gallery and for the companion app."},
		Expect: []string{"It names the app it serves and the URL; the companion (whose manifest has an id) and the gallery both serve."},
	},

	// ---- edge ----
	{
		ID: "ED-01", Area: "edge", Title: "Display scale", Platforms: desktop, Special: true,
		Where:  gallery,
		Steps:  []string{"Start the gallery with BELLUI_SCALE=2, then BELLUI_SCALE=0.75; open Inputs, Charts and Data Grid."},
		Expect: []string{"Everything scales evenly; text stays sharp; nothing overlaps at either scale."},
	},
	{
		ID: "ED-02", Area: "edge", Title: "A bad WebGPU backend list", Platforms: desktop, Refs: []string{"#731"}, Special: true,
		Where: gallery,
		Steps: []string{"Start the gallery with BELLUI_WEBGPU_BACKENDS=gl (and on Linux also =metal)."},
		Expect: []string{
			"The app starts and draws; the terminal shows a warning naming the native backend it added (#731).",
			"It never aborts and never shows a blank window.",
		},
	},
	{
		ID: "ED-03", Area: "edge", Title: "No network", Special: true,
		Where: gallery + " → Charts → Geo (World Map, Choropleth)",
		Setup: "Turn the network off (airplane mode, or DevTools → Network → Offline on the web) before opening Geo.",
		Steps: []string{"Open Geo; wait ten seconds; turn the network back on and wait."},
		Expect: []string{
			"Offline: the charts draw what they can (axes, cities) without hanging; no crash.",
			"Online again: after panning the map once, tiles and country shapes appear without reopening the page.",
		},
	},
	{
		ID: "ED-04", Area: "edge", Title: "Backgrounding and resuming", Platforms: mobile, Special: true,
		Where:  gallery + " → Charts → Trading (Live Streaming) and Communication",
		Steps:  []string{"Put the app in the background for a minute, bring it back; repeat with the screen locked."},
		Expect: []string{"The app resumes where it was, draws at once and keeps responding; the live chart carries on."},
	},
	{
		ID: "ED-05", Area: "edge", Title: "Fast repeated input", Special: true,
		Where:  companion + " → Locale and Direction",
		Steps:  []string{"Press the locale buttons and the direction toggles as fast as you can for ten seconds."},
		Expect: []string{"The app keeps up and ends in the state of the last press; no crash, no stuck layout."},
	},
}
