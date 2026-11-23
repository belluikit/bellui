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
	"smoke":   "Smoke: every page, theme and window",
	"widgets": "Widgets: disabled, pressed, drawer, wrapping",
	"touch":   "Touch and keys",
	"i18n":    "Internationalisation",
	"rtl":     "Right-to-left",
	"a11y":    "Accessibility",
	"charts":  "x/charts",
	"tables":  "x/tables",
	"dag":     "x/dag",
	"media":   "x/mmedia",
	"cli":     "Packaging and the bellui command",
	"edge":    "Edge cases",
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
		ID: "SM-02", Area: "smoke", Title: "Light, dark and dense themes", Refs: []string{"#688", "#695", "#1381"},
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
		Refs:  []string{"#593", "#619", "#975"},
		Where: gallery + ", Communication page (progress, snackbar), Scrolling page",
		Steps: []string{
			"Watch the progress indicators animate for ten seconds.",
			"Show a snackbar and let it disappear; show a toast.",
			"Fling the Scrolling page's list up and down; toggle light/dark mid-fling.",
			"If anything looks wrong, rebuild the gallery with -tags nopartialrepaint and repeat, to tell a partial-repaint fault from any other.",
		},
		Expect: []string{
			"No ghost of a snackbar, toast, ripple or old scroll position stays on screen, and no thin dark line is left along an animating card or clipped container (#975).",
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
	{
		ID: "SM-07", Area: "smoke", Title: "A window around wrapping content can shrink again", Platforms: desktop, Refs: []string{"#1349", "#1346"},
		Where: companion,
		Steps: []string{
			"Start the companion app and open the Chips & layout tab, then the Text tab.",
			"Drag the window much wider, then narrower than it started, then shorter.",
		},
		Expect: []string{
			"The window opens at its set size and does not grow taller when either tab opens (#1349).",
			"It can be made narrower and shorter again after being widened; the chips and the flow re-wrap to fit (#1346).",
		},
	},
	{
		ID: "SM-08", Area: "smoke", Title: "SVG strokes scale with the image", Refs: []string{"#1292"},
		Where: companion + " → Chips & layout → SVG strokes scale",
		Steps: []string{"Compare the small ring with the large one."},
		Expect: []string{
			"The large ring is the small one magnified: its orange stroke is about 14 px wide, the small one's about 2 px, and the blue dashes scale the same way.",
			"The small ring's stroke does not fill it in.",
		},
	},

	// ---- widgets ----
	{
		ID: "WG-01", Area: "widgets", Title: "Disabled widgets are visible but inert, with one look", Special: true,
		Refs:  []string{"#1212", "#1213", "#1214", "#1215", "#1216", "#1223", "#1232", "#1251"},
		Where: companion + " → Disabled",
		Steps: []string{
			"Tick \"Disable everything below\".",
			"Tap or click every control on the page: the buttons, FAB, entry, select, slider, switch, check, radio, segments, chips, the date and time pickers, the rich entry, the app bar's menu and search icons, the navigation bar's items, the carousel's arrow, and the rows of the list, tree and tree view. Try typing into the entry.",
			"Drag the slider, the carousel and the time picker's dial.",
			"Over the list, tree and tree view, turn the mouse wheel (desktop, web) or swipe up (phone).",
			"In the entry, select a word, copy it and paste it somewhere else.",
			"Untick the box.",
		},
		Expect: []string{
			"Every control keeps its place and shows MD3's disabled look: labels and icons at about 38% of the text colour. A disabled Button's label is as readable as the disabled entry's text; in rc.1 the desktop dimmed it twice, to about 14% (#1223).",
			"The look is the same on desktop, phone and web. In rc.1 a phone dimmed nothing for a widget with no disabled look of its own.",
			"The rich entry's orange image is dimmed with its text (#1232). The app bar's title stays at full colour; only its icons look disabled (#1251).",
			"Nothing responds: the status line reports no tap, no value, page, selection, date or time changes, nothing opens, and no pressed or hover state shows.",
			"The wheel or swipe over the disabled list, tree and tree view moves neither them nor the page around them (#1251).",
			"The disabled entry's text can still be selected and copied, but not edited.",
			"After unticking, everything works again.",
		},
	},
	{
		ID: "WG-02", Area: "widgets", Title: "A disabled container disables what it holds", Refs: []string{"#1218", "#1378", "#1288"}, Special: true,
		Where: companion + " → Disabled → Containers (tabs, document tabs, an inner window)",
		Steps: []string{
			"Tick \"Disable everything below\".",
			"Tap \"Second tab\" and \"Doc two\"; drag the inner window by its title bar and by its corner; press its close, minimise and maximise buttons.",
			"Tap the buttons and entries inside the three containers, and the link to example.com in the first tab and in the inner window; hover over the links.",
			"Keyboard (desktop, web): press Tab through the page, then Space or Enter wherever the focus lands.",
			"Untick the box, then look at the \"Disabled by the app\" check in the first tab.",
		},
		Expect: []string{
			"While disabled, no tab switches, no document tab closes, and the inner window neither moves, resizes nor closes.",
			"Every button and entry inside them, in every tab, looks disabled and does nothing.",
			"The tab labels are dimmed once, not twice.",
			"The links open nothing and show no hover underline or pointer, and Tab never stops on them, so Space or Enter cannot open them (#1378, #1288).",
			"After unticking, everything inside works again, except \"Disabled by the app\", which stays disabled because the app disabled it itself.",
		},
	},
	{
		ID: "WG-03", Area: "widgets", Title: "A disabled widget takes no focus", Platforms: keyed, Refs: []string{"#1221", "#1251"},
		Where: companion + " → Disabled",
		Steps: []string{
			"Press \"Disable everything in 3 seconds\", then at once click into the entry under \"Actions and choices\" so its caret shows.",
			"When everything is disabled, type a letter.",
			"Click an empty spot, then press Tab repeatedly through the page.",
			"Untick \"Disable everything below\".",
		},
		Expect: []string{
			"When the page disables itself, the entry loses its caret and focus ring at once, and the letter goes nowhere (#1221).",
			"Tab skips every disabled control: only the box at the top and the \"Disable everything in 3 seconds\" button take the focus.",
			"After unticking, the entry does not take the focus back by itself.",
		},
	},
	{
		ID: "WG-04", Area: "widgets", Title: "Controls show a pressed state", Refs: []string{"#1085", "#1091", "#1156"},
		Where: gallery + " → Actions, Inputs, Navigation and Auth",
		Steps: []string{
			"Press and hold, then release, each kind of control: filled, outlined and text buttons, icon buttons, FABs, segmented and split buttons, chips, check boxes, radio buttons, switches, list rows, menu items, tabs, and navigation bar, rail and drawer items.",
			"On a phone, start a press on a list row and turn it into a scroll.",
			"On Auth, tap a sign-in button.",
		},
		Expect: []string{
			"Each shows a pressed state from the moment of the press, in the control's own content colour, and it fades after the release, so a quick tap is seen too.",
			"A segmented button's pressed state follows the rounded ends of its first and last segment (#1156).",
			"A press that turns into a scroll leaves no pressed state behind.",
			"The sign-in tap does not hang the app; the page keeps responding.",
		},
	},
	{
		ID: "WG-05", Area: "widgets", Title: "Navigation drawer: either edge, below a header, open and closed", Special: true,
		Refs:  []string{"#1171", "#1172", "#1173", "#1186", "#1238", "#1280"},
		Where: companion + " → Drawer",
		Steps: []string{
			"Press the app bar's menu icon, then close the drawer with a tap on the dimmed area beside it.",
			"Press Show, then close it with Escape (desktop, web) or Back (Android).",
			"Choose End edge, press Show and pick Documents.",
			"Tick \"Open below the app bar\", press the menu icon, then press the app bar's X icon.",
			"Press \"Show twice, then Hide after 2 seconds\" and wait.",
			"Open the drawer again and turn the mouse wheel, or swipe, over the dimmed area.",
			"Direction tab → \"Toggle the app's direction\"; come back and open the drawer at Start edge, then at End edge. Toggle the direction back.",
		},
		Expect: []string{
			"With animations on, the drawer slides in from its edge and back out to it (#1186).",
			"Start edge opens on the left, End edge on the right; in right-to-left, the other way round (#1171).",
			"On a phone narrower than 360 pt, the drawer reaches its edge and leaves a strip of dimmed area to tap (#1238).",
			"Below the app bar, the drawer and the dimmed area start under the bar, and the bar's X icon can be tapped and closes the drawer (#1171, #1172).",
			"The app bar shows an X while the drawer is open and the menu icon while it is closed, whatever opened or closed it. The status line counts one change per open and one per close: \"Show twice\" adds two in all, not three (#1280).",
			"Picking Documents closes the drawer and shows \"Destination: Documents\".",
			"The wheel or swipe over the dimmed area does not scroll the page under it (#1173).",
		},
	},
	{
		ID: "WG-06", Area: "widgets", Title: "Wrapping text keeps its width, and long snackbars wrap",
		Refs:  []string{"#1038", "#1099", "#1106", "#1195", "#1198", "#1199", "#1202"},
		Where: companion + " → Text → \"Wrapping text keeps its width\" and Snackbars",
		Steps: []string{
			"Read each example, then make the window narrower and wider (desktop, web) or rotate (phone).",
			"Press \"Show a long snackbar\", then \"Show two snackbars at once\".",
		},
		Expect: []string{
			"Beside the button, centred, in the row-wrap flow and in the form's label column, the label is as wide as its text or its room allows and wraps between words. None collapses to one letter per line with its text running down the screen.",
			"In the form, the label column takes at most half the width, and the field beside it keeps its room (#1202).",
			"The French text never starts a line with \":\", \";\", \"!\" or \"»\", and \"10 km\" stays on one line (#1099).",
			"The long snackbar wraps inside the window with its Undo button at the end, cut off at neither edge (#1106).",
			"Two snackbars at once show as one: only \"Second snackbar\" (#1038).",
		},
	},
	{
		ID: "WG-07", Area: "widgets", Title: "Chips: selected and excluded", Special: true, Refs: []string{"#1383", "#1364"},
		Where: companion + " → Chips & layout → Chips: selected and excluded",
		Steps: []string{
			"Tap Rock once, twice and a third time, reading the status lines after each tap.",
			"Tap Jazz, Pop and Folk twice each, so all of them are excluded but Rock, then tap Rock twice.",
			"Toggle light/dark on the Direction tab and come back.",
			"Tick \"Disable the group\", tap the chips, then untick it.",
			"In the \"One at a time\" group, tap Small, Medium, then Medium again.",
		},
		Expect: []string{
			"Rock goes selected (filled, a check mark in front), then excluded (a cross in front, a red outline and its label struck through), then off. Each tap's change is named on \"Last change\".",
			"The fourth style cannot be excluded: Rock stays selected and \"Last change\" says the last style cannot be excluded.",
			"Selected, excluded and off are easy to tell apart in both light and dark, without relying on colour: the check mark, the cross and the strike-through show it.",
			"Disabled, the chips keep their check mark, cross and strike-through in the disabled colours and take no tap.",
			"The one-at-a-time group never shows the excluded look: a second tap on Medium deselects it.",
		},
	},
	{
		ID: "WG-08", Area: "widgets", Title: "Destructive buttons are red in every style", Refs: []string{"#1327"},
		Where: gallery + " → Actions → Danger",
		Steps: []string{"Look at the card in light, then toggle to dark and look again. Press each button."},
		Expect: []string{
			"Text, Outlined and Elevated buttons draw their label and icon in red (Outlined its outline too); Filled is filled red and FilledTonal with the pale error colour; the danger icon buttons are red or red-filled.",
			"Each still shows a pressed state, in both themes.",
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
		ID: "TK-06", Area: "touch", Title: "A hardware Return reaches a widget that takes keys but no text", Platforms: native, Refs: []string{"#666", "#793"},
		Where: companion + " → Touch & keys → Key target",
		Per: map[string]string{
			"android": "With no keyboard attached, send the keys with: adb shell input keyevent 66 (Return), then adb shell input keyevent 21 (Left).",
			"ios":     "Attach a Bluetooth keyboard.",
		},
		Steps: []string{
			"Tap or click the Key target card.",
			"Press Return, then the left arrow, then type the letter a.",
			"Hold the left arrow down for two seconds.",
		},
		Expect: []string{
			"The card says \"focused\" after the tap.",
			"It then shows \"key Return\", \"key Left\" and \"rune 'a'\" in turn.",
			"Each key shows when it goes down, not when it is released, and the held arrow repeats (#793). On Android, adb cannot hold a key: use a real keyboard or hack/keyprobe/keyboard.sh.",
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
	{
		ID: "TK-10", Area: "touch", Title: "A swipe on a nested widget scrolls the page", Platforms: mobile, Special: true,
		Refs:  []string{"#662", "#801", "#986", "#987", "#1229", "#1266"},
		Where: companion + " → Swipes",
		Steps: []string{
			"Swipe up and down, starting each swipe on: the slider, the disabled slider, the carousel, the sideways row of chips and the selectable text.",
			"Swipe sideways on the slider, the carousel and the row of chips.",
			"On the inner list, the table, the tree, the fixed tree view and the multi-line entry, swipe up until each reaches its end, then keep swiping up.",
			"On the tree view whose rows can be dragged, drag \"Inbox 1\" below \"Inbox 3\".",
		},
		Expect: []string{
			"Every vertical swipe in the first step scrolls the page, as it would starting on plain text. In rc.1 a swipe that started on a slider, a carousel or the chip row left the page where it was.",
			"Sideways, the slider moves, the carousel pages and the chip row scrolls; the page does not move.",
			"The inner list, table, tree, fixed tree view and entry scroll their own content first; once one is at its end, the swipe scrolls the page (#662, #801, #1266).",
			"A swipe at the table's end never stretches its first row (#1266).",
			"A swipe over the selectable text and the entry selects nothing (#986, #801).",
			"In the movable tree view, the drag moves the row and the drop lands on the row under the finger; the line below reads \"Moved: Inbox 1 …\" (#1229). Its rows keep the drag, so this tree view does not scroll the page.",
		},
	},
	{
		ID: "TK-11", Area: "touch", Title: "Charts and the data grid hand a swipe to the page", Platforms: mobile, Refs: []string{"#1266"}, Special: true,
		Where: gallery + " → Charts (Basic, Trading) and Data Grid (Sort & Filter)",
		Steps: []string{
			"On Charts → Basic, swipe up and down starting on a chart's plot.",
			"Pinch to zoom into a chart, then swipe sideways on it, then swipe up on it.",
			"On Trading → Drawing Tools, pick the trend line tool and draw a line with one finger.",
			"On Data Grid → Sort & Filter, swipe up on the grid body until its last row shows, then keep swiping; swipe up on the column header row.",
		},
		Expect: []string{
			"A vertical swipe on an unzoomed chart scrolls the page. In rc.1 the chart kept every swipe, which is why the gallery leaves gaps between the charts on a phone.",
			"Zoomed in, a sideways swipe pans the chart and an upward one still scrolls the page.",
			"With a drawing tool on, the drag draws the line; the page does not move.",
			"The grid scrolls its rows until its end, then the swipe scrolls the page; a swipe on its header scrolls the page.",
		},
	},
	{
		ID: "TK-12", Area: "touch", Title: "A long-press keeps its word, and a drag extends it a word at a time", Platforms: mobile, Refs: []string{"#1267", "#894"}, Special: true,
		Where: companion + " → Text → \"Long-press a word\"",
		Steps: []string{
			"Long-press the word \"unwrapped\" in the sentence and keep the finger still for a second after the selection appears, then lift.",
			"Long-press \"unwrapped\" again and, without lifting, drag right across the next three words, then back left past the start of \"unwrapped\".",
			"Do the same two steps in the text field below it.",
			"Android: with the on-screen keyboard open for the text field, close it (Back or the keyboard's hide key).",
		},
		Expect: []string{
			"After the long-press, the selection is the whole of \"unwrapped\", also once the finger has been held still; in rc.2 it shrank to \"unw\" as the finger crept (#1267).",
			"Dragged right, the selection grows a whole word at a time; dragged back left past it, it runs from the end of \"unwrapped\" back over whole words.",
			"The text field behaves the same.",
			"Android: when the keyboard goes, the page grows back to the full height of the screen (#894).",
		},
	},
	{
		ID: "TK-13", Area: "touch", Title: "Wheel picker and swipe card", Refs: []string{"#977", "#984", "#989", "#1103"},
		Where: companion + " → Swipes, near the end of the page",
		Per:   map[string]string{"desktop": "Use the mouse: press, drag, release.", "web": "Use the mouse: press, drag, release."},
		Steps: []string{
			"Spin the wheel up with a short flick, then drag it down slowly past its first item.",
			"Drag the swipe card a little sideways and let go; then drag it most of the way off and let go; then flick it.",
			"Under it, flick the directions card left, then right, then up and down.",
		},
		Expect: []string{
			"The wheel coasts and settles with one item centred, and \"Wheel: Item N\" names that item. Past the first item it resists and springs back.",
			"Items away from the centre are smaller and fainter.",
			"The card follows the finger and tilts a little; a short drag springs back, a long drag or a flick throws it off, \"Card: swiped left\" (or right) shows and a new card takes its place.",
			"The directions card: left throws it off and a new one comes; right lets it follow, then springs it back with \"swiped right\" counted; up and down do not move it and count nothing (#1103).",
		},
	},

	// ---- i18n ----
	{
		ID: "I18N-01", Area: "i18n", Title: "Switching the locale at run time", Refs: []string{"#87", "#944"},
		Where: companion + " → Locale",
		Steps: []string{
			"Press en-US, then de-DE, then ja-JP, then System.",
		},
		Expect: []string{
			"en-US: Cancel; date 03/09/2026; number 1,234,567.89; currency €1,234.50; list \"red, green, and blue\".",
			"de-DE: Abbrechen; date 09.03.2026; number 1.234.567,89; currency 1.234,50 €; list \"red, green und blue\".",
			"ja-JP: キャンセル; date 2026/03/09; list \"red、green、blue\".",
			"Each press updates the text at once, with no restart. System returns to the OS language.",
			"\"Time now\" is the device clock's time, in the device's time zone; on iOS it is not UTC (#944).",
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
		ID: "RTL-01", Area: "rtl", Title: "The gallery in right-to-left", Refs: []string{"#749", "#945", "#1078", "#1080"},
		Where: gallery + ", app bar RTL toggle (⋯ icon)",
		Steps: []string{
			"Toggle RTL on. Visit Navigation, Inputs, Containment, Data Grid and Charts.",
			"Toggle RTL off.",
		},
		Expect: []string{
			"The navigation moves to the right; rows, forms and app bars mirror; text aligns to the right.",
			"Directional icons (back, forward, navigate) point the other way.",
			"On Actions, the extended FAB's icon sits beside its label, not over it (#945).",
			"On Charts, with the window narrow enough that the tabs overflow, the tab indicator sits under the selected tab (#1078, #1080).",
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
	{
		ID: "RTL-07", Area: "rtl", Title: "Chip rows and wrapping layouts in right-to-left", Refs: []string{"#1371", "#1372"},
		Where: companion + " → Chips & layout",
		Steps: []string{
			"Under \"Chip rows\", choose Start, Center and End in turn.",
			"Press \"Toggle the app's direction\" and choose Start, Center and End again; look at the row-wrap flow, the flex row and the flex column.",
			"Toggle the direction back.",
		},
		Expect: []string{
			"Each row of chips sits at the left, the middle or the right, the shorter last row included.",
			"Right-to-left, Start and End swap sides; the first chip of each row is the rightmost.",
			"Right-to-left, the flow starts with 1 at the right edge and every row runs leftwards; the flex row reads \"1 first\" at the right; the flex column's buttons sit at the right edge.",
			"Toggling back restores the left-to-right layout exactly.",
		},
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
	{
		ID: "A11Y-07", Area: "a11y", Title: "Disabled widgets are read as unavailable", Platforms: native, Refs: []string{"#1214", "#1251"},
		Where: companion + " (a11y build) → Disabled",
		Steps: []string{
			"With the screen reader on, move through the controls, then tick \"Disable everything below\" and move through them again.",
			"While disabled, try to activate a button and the entry with the screen reader's gesture or key.",
		},
		Expect: []string{
			"Disabled controls are read with \"dimmed\", \"unavailable\" or \"disabled\" (the platform's word).",
			"Activating one does nothing, and the entry takes no focus for typing.",
			"The app bar's title is read as before; its icons are read as unavailable.",
		},
	},
	{
		ID: "A11Y-08", Area: "a11y", Title: "A screen reader hears selected and excluded chips", Platforms: native, Refs: []string{"#1383", "#1348"}, Special: true,
		Where: companion + " (a11y build) → Chips & layout",
		Steps: []string{
			"With the screen reader on, move to Rock and activate it twice, listening after each.",
			"On the Locale tab choose de-DE, come back and move to Rock again; then choose System.",
		},
		Expect: []string{
			"After the first activation Rock is read as selected (the platform's word for it).",
			"After the second it is read as \"Rock, excluded\".",
			"In German it is read as \"Rock, ausgeschlossen\".",
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
	{
		ID: "CH-07", Area: "charts", Title: "A chart follows a light/dark switch", Refs: []string{"#1381"},
		Where:  gallery + " → Theme page (choose System), then Charts → Basic",
		Per:    map[string]string{"web": "DevTools → Rendering → Emulate prefers-color-scheme switches it too."},
		Steps:  []string{"With a chart showing, switch the OS (or browser) between light and dark, twice."},
		Expect: []string{"Each chart redraws in the new theme's colours with the rest of the window: no dark chart in a light window or the reverse."},
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
	{
		ID: "TB-03", Area: "tables", Title: "Group headers: tap, arrows, redraw and scroll bars", Special: true,
		Refs:  []string{"#1252", "#1254", "#1256", "#1258"},
		Where: gallery + " → Data Grid → Grouping",
		Steps: []string{
			"Press \"Group by Sector\".",
			"Tap (or click) anywhere on a group's header row, then again.",
			"Collapse several groups, scroll, then expand them again; press \"Collapse All\" and \"Expand All\".",
			"Make the window small enough that both scroll bars show, and scroll down and sideways.",
		},
		Expect: []string{
			"Each tap on a header row collapses or expands its group, also on a phone (#1252).",
			"The arrows are the theme's grey chevrons, pointing down when open and sideways when closed; never a blue play-button emoji (#1256).",
			"After every collapse, expand and scroll, each header sits on its own group's first row; no header is missing or drawn over the wrong rows (#1254).",
			"The scroll bars are whole bars, not pieces between the group headers (#1258).",
		},
	},
	{
		ID: "TB-04", Area: "tables", Title: "A pinned-right column leaves the scroll bars and last column reachable", Refs: []string{"#1257"},
		Where: gallery + " → Data Grid → Columns (Volume is pinned to the right)",
		Steps: []string{
			"Make the window small enough that both scroll bars show.",
			"Scroll down, then scroll right to the end.",
		},
		Expect: []string{
			"The vertical scroll bar is drawn whole beside the pinned Volume column, not hidden under it or showing as dashes between rows.",
			"The pinned column stops above the horizontal scroll bar.",
			"Scrolled to the end, the last unpinned column shows in full, clear of the pinned one.",
		},
	},
	{
		ID: "TB-05", Area: "tables", Title: "A tap on a column header sorts, also on a phone", Refs: []string{"#1268", "#1275"},
		Where: gallery + " → Data Grid → Sort & Filter",
		Steps: []string{
			"Tap (or click) the Price header three times.",
			"Desktop: Shift-click the Change % header.",
			"Drag the border between two headers to resize a column.",
		},
		Expect: []string{
			"Each tap sorts once: ascending, descending, then unsorted, with the header's arrow showing which. In rc.2 a tap on a phone did nothing. This is the default order, which a column's SortingOrder can now change (#1275).",
			"Shift-click adds Change % as a second sort column.",
			"Resizing a column, or a drag starting on a header, does not sort.",
		},
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
		ID: "MD-01", Area: "media", Title: "Picture and sound start together", Platforms: desktop, Refs: []string{"#787", "#504", "#1162"}, Special: true,
		Where: gallery + " → Media Player (the bundled sample, or Open file… with a video that has speech)",
		Steps: []string{
			"Open the page with speakers on; watch the first two seconds closely. Seek forward and back.",
			"Pause, then seek a few times while paused.",
		},
		Expect: []string{
			"The first frames do not run ahead of the sound; lips and voice line up from the start (#787).",
			"After a seek, picture and sound resume together within a moment.",
			"Each seek while paused shows the frame it landed on (#1162).",
		},
	},
	{
		ID: "MD-02", Area: "media", Title: "No audio device", Platforms: desktop, Refs: []string{"#690"}, Special: true,
		Where:  gallery + " → Media Player",
		Setup:  "Disable every audio output: unplug or disable the sound device, or on Linux run without a sound server.",
		Steps:  []string{"Open the Media Player page."},
		Expect: []string{"The page shows \"Cannot play …\" with the audio error at once; the app does not hang or play silently."},
	},
	{
		ID: "MD-03", Area: "media", Title: "Every bundled audio format plays", Platforms: native, Refs: []string{"gallery#28", "#842", "#852", "#1020"},
		Where: gallery + " → Audio Player",
		Steps: []string{
			"With sound on, pick each entry of Bundled clip in turn: MP3, AAC (m4a), Ogg Vorbis, Opus, FLAC, WAV.",
			"Let one of them play to its end, then press play again.",
			"Phone: press \"Open file…\" and pick an audio file from the device's storage; do it five times in a row.",
		},
		Expect: []string{
			"Each plays the same rising scale from its first note, twelve notes one second apart, and shows 0:12 as its length.",
			"At the end the event log shows \"ended\" and the position reads 0:12 / 0:12. Play then starts it again from the first note, and the app does not freeze (#842).",
			"Every picked file plays, and the app never crashes while picking (#852, #1020).",
		},
	},
	{
		ID: "MD-04", Area: "media", Title: "Audio transport: seek, pause, stop, volume", Platforms: native, Refs: []string{"#831", "#833", "#855", "#1032", "#1336", "#1340"},
		Where: gallery + " → Audio Player, any bundled clip",
		Steps: []string{
			"While it plays, seek to about 0:05.",
			"Pause, seek to about 0:08, then play.",
			"Stop, then play.",
			"Move the volume control down and back up.",
			"Seek to the last half second of the clip while it plays.",
		},
		Expect: []string{
			"After the first seek the sixth note plays (the scale has one note a second), with no snippet of the old position first (#855), and from the start of the note, not partway in (#1336).",
			"After the paused seek, play resumes at the ninth note, not where it was paused.",
			"Stop returns to 0:00, and play starts again from the first note at once, without holding at an earlier position (#833).",
			"The volume follows the control at once (#1032).",
			"The last half second is heard before the clip ends; it is not silent (#1340).",
		},
	},
	{
		ID: "MD-05", Area: "media", Title: "Audio loop", Platforms: native, Refs: []string{"#502", "#527"},
		Where: gallery + " → Audio Player",
		Steps: []string{
			"Turn Loop on, pick a clip and let it reach the end twice.",
			"Turn Loop off while it plays.",
		},
		Expect: []string{
			"With Loop on, the scale starts again right after its last note, and the log shows no \"ended\".",
			"With Loop off, it plays to the end once more and the log shows \"ended\".",
		},
	},
	{
		ID: "MD-06", Area: "media", Title: "Audio over the network, and a dropped connection", Platforms: native, Refs: []string{"#839"}, Special: true,
		Where: gallery + " → Audio Player, URL field",
		Setup: "On the computer, serve the gallery's clips: cd <gallery>/pages/media/audio && python3 -m http.server 8000. A phone must be on the same network, and the URL uses the computer's address on it.",
		Steps: []string{
			"Enter http://<computer>:8000/scale.mp3 and press Play URL.",
			"Seek within it.",
			"Play it again and, about three seconds in, stop the server (Ctrl+C).",
		},
		Expect: []string{
			"It plays and seeks like the bundled clip.",
			"When the server stops, playback stops where the data ran out and the log shows an error. It does not show \"ended\", and the position does not jump to 0:12 (#839).",
		},
	},
	{
		ID: "MD-07", Area: "media", Title: "Audio keeps playing in the background", Platforms: mobile, Refs: []string{"#672", "#673", "#846", "#1052"}, Special: true,
		Where: gallery + " → Audio Player",
		Steps: []string{
			"Turn Loop on and play a clip.",
			"Press Home and wait a minute; then lock the screen and wait another minute.",
			"Come back to the gallery.",
		},
		Expect: []string{
			"The scale is heard the whole time, with the app in the background and with the screen locked.",
			"Back in the app, the position has moved on, never past the clip's end, and the controls respond (#846).",
		},
		Per: map[string]string{
			"ios":     "Also with the silent switch on (or the Control Centre silent mode): a player keeps sounding, as music apps do.",
			"android": "If you can, take a phone call while it plays; after the call the music sounds as before, not like a call (#1052).",
		},
	},
	{
		ID: "MD-08", Area: "media", Title: "Seeking an MP3 over http, also where the server ignores ranges", Platforms: native, Refs: []string{"#1098", "#872"}, Special: true,
		Where: gallery + " → Audio Player, URL field",
		Setup: "Serve the gallery's clips as in MD-06 (python3 -m http.server answers every request with the whole file, as a server that ignores Range does). For the https leg, any https URL of a CBR MP3 will do, for example https://raw.githubusercontent.com/belluikit/gallery/main/pages/media/audio/scale.mp3.",
		Steps: []string{
			"Enter http://<computer>:8000/scale.mp3 and press Play URL.",
			"While it plays, seek to about 0:08, then to about 0:02, then to about 0:11.",
			"Pause, seek to about 0:05, play.",
			"Repeat with the https URL.",
			"Optional, if a long CBR MP3 is at hand (several minutes, on a server that honours ranges): play it over http and seek near its end.",
		},
		Expect: []string{
			"Each seek lands on the note it was asked for (the ninth, third and twelfth note; after the paused seek, the sixth), and the shown position matches it.",
			"No seek fails; in rc.2 some seeks on such a server failed with \"seek failed: -38\".",
			"The app keeps responding during every seek (#872).",
			"The long file's seek near the end starts playing within a moment, without first downloading the whole file (#1098).",
		},
	},
	{
		ID: "MD-09", Area: "media", Title: "Seeking, pausing and changing songs do not click", Platforms: native, Refs: []string{"#1228", "#1273"}, Special: true,
		Where: gallery + " → Audio Player, any bundled clip, with headphones",
		Steps: []string{
			"Seek back and forth a dozen times while it plays, including right at the start of the clip.",
			"Pause and play several times in mid-note.",
			"While it plays, pick a different clip in Bundled clip; do it several times.",
		},
		Expect: []string{
			"Each seek, pause, resume and change of clip is clean: the sound fades out and in over a few milliseconds, with no click or pop.",
			"Android: the next clip does not start with a fragment of the end of the last one (#1228).",
		},
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
	{
		ID: "CLI-05", Area: "cli", Title: "Packages carry the FFmpeg libraries", Platforms: native, Refs: []string{"#1101", "#1122"}, Special: true,
		Where: "The gallery packaged by bellui package for the platform",
		Per: map[string]string{
			"desktop": "macOS: bellui package -os darwin, then otool -L <App>.app/Contents/MacOS/<binary> | grep av. Linux: bellui package -os linux, then look in the package's lib/<app id>/. Windows: the FFmpeg DLLs lie beside the .exe.",
			"android": "unzip -l <gallery>.apk | grep goffmpeg",
			"ios":     "ls <App>.app/Frameworks",
		},
		Steps: []string{
			"Package the gallery and list the libraries as described for this platform.",
			"Copy the package to another folder (desktop) or install it (phone), start it, and open Media Player or Audio Player.",
		},
		Expect: []string{
			"The FFmpeg libraries (libavformat, libavcodec and the rest) are inside the package. On macOS otool lists them under @executable_path or @rpath into Contents/Frameworks, never a path into the Go module cache.",
			"Android: each lib/<abi>/ holds the libraries next to the app's own library.",
			"The app starts and plays; in rc.1, an Android app stopped at launch with \"dlopen failed: library libavformat-goffmpeg.so not found\".",
		},
	},
	{
		ID: "CLI-06", Area: "cli", Title: "Android store build", Platforms: []string{"android"}, Refs: []string{"#793", "#889", "#935", "#1115"}, Special: true,
		Where: "The companion app, packaged by setup, and bellui release -os android",
		Steps: []string{
			"unzip -l Release_Check.apk | grep '\\.so' and zipalign -c -P 16 -v 4 Release_Check.apk.",
			"Install it, open it and look at the size under Settings → Apps → Release Check → Storage.",
			"Run bellui release -os android for the companion with a test keystore, then bundletool dump config --bundle=<app>.aab.",
			"On Android 16 if available: open the Locale tab's confirm dialog and press Back; press Back again.",
			"Plug in a hardware keyboard (or run hack/keyprobe/keyboard.sh) while the companion shows a non-default locale chosen on the Locale tab.",
		},
		Expect: []string{
			"zipalign reports \"Verification successful\". The app's size is about the size of its library once, not twice (#889).",
			"The bundle config shows PAGE_ALIGNMENT_16K (#1115).",
			"The first Back closes the dialog, the second leaves the app (#935).",
			"Plugging in the keyboard does not restart the app: the chosen locale is still shown (#793).",
		},
	},
	{
		ID: "CLI-07", Area: "cli", Title: "iOS packages for the App Store", Platforms: []string{"ios"}, Refs: []string{"#932", "#1117", "#1249"},
		Where: "The gallery packaged by setup",
		Steps: []string{
			"ls <App>.app/PrivacyInfo.xcprivacy and plutil -p <App>.app/Info.plist | grep -A3 UIApplicationSceneManifest.",
			"Install and start it on a device with the newest iOS available (iOS 27 if possible).",
			"If the keychain holds two \"Apple Distribution\" certificates: bellui release -os ios -profile <profile> for the companion.",
		},
		Expect: []string{
			"The privacy manifest is there, and the Info.plist declares a scene manifest (#932, #1117).",
			"The app starts and draws; it is not black (#1117).",
			"The release signs and names the identity it used, instead of codesign's \"ambiguous\" error (#1249).",
		},
	},
	{
		ID: "CLI-08", Area: "cli", Title: "Linux web views bundle Chromium 156", Platforms: desktop, Refs: []string{"#1112"},
		Where: "Linux only: hack/webviewprobe",
		Steps: []string{"In hack/webviewprobe run: bellui package -os linux. Then start the packaged probe."},
		Expect: []string{
			"The command prints \"bundled CEF 156.0.30…\" (Chromium 156), with no checksum error.",
			"The probe's web view draws its page.",
		},
	},
	{
		ID: "CLI-09", Area: "cli", Title: "Each command's --help names its own targets", Platforms: desktop, Refs: []string{"#1369"},
		Where:  "A terminal, with the bellui command from setup",
		Steps:  []string{"Run bellui build --help, bellui package --help, bellui release --help and bellui install --help, and read the --target/--os line of each."},
		Expect: []string{"package and release name web, wasm and iossimulator; install names only devices (android…, ios, iossimulator); build names the bellui targets and says any other GOOS goes to Go."},
	},
	{
		ID: "CLI-10", Area: "cli", Title: "bellui translate creates a missing catalog folder", Platforms: desktop, Refs: []string{"#1368"},
		Where: "A copy of the companion app in a temporary folder (it has no translations/ folder)",
		Setup: "cp -r hack/releasecheck/app /tmp/rc-app && cd /tmp/rc-app",
		Steps: []string{"Run bellui translate translations/en.pxf, then ls -l translations."},
		Expect: []string{
			"It succeeds and creates translations/en.pxf; no error names a temporary file.",
			"The catalog is readable by everyone (-rw-r--r--).",
		},
	},
	{
		ID: "CLI-11", Area: "cli", Title: "A phone opens a served app over https", Platforms: []string{"web"}, Refs: []string{"#1359", "#1358"}, Special: true,
		Where: "The companion app served from this computer, opened on a phone on the same network",
		Steps: []string{
			"Run bellui serve in the companion app's folder and open the printed network address (http://<this computer's address>:8080) on the phone.",
			"Stop it, run bellui serve --https, open the https network address it prints on the phone and accept the certificate warning once.",
			"Optional, with cloudflared installed: bellui serve --tunnel and open the trycloudflare.com address it prints.",
		},
		Expect: []string{
			"Over plain http the page says \"WebGPU needs a secure page: open this app over https, or at localhost on this computer.\", not that the browser lacks WebGPU.",
			"Over https (and through the tunnel) the app starts and draws on the phone.",
			"serve --https prints each network address and the certificate's SHA-256 fingerprint; Ctrl-C stops it, and the tunnel with it.",
		},
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
	{
		ID: "ED-06", Area: "edge", Title: "A covered window catches up", Platforms: desktop, Refs: []string{"#864", "#1019"},
		Where: companion + " → Disabled",
		Steps: []string{
			"Press \"Disable everything in 3 seconds\" and at once cover the window completely with another window (or minimise it).",
			"After five seconds, uncover it.",
		},
		Expect: []string{
			"As soon as it shows, the window shows everything disabled; it does not wait for a mouse move.",
			"No \"draw was refused\" error appears in the terminal (#1019).",
		},
	},
	{
		ID: "ED-07", Area: "edge", Title: "iOS Display Zoom", Platforms: []string{"ios"}, Refs: []string{"#1016", "#1238"},
		Where: gallery,
		Setup: "Settings → Display & Brightness → Display Zoom → Larger Text, then restart the gallery.",
		Steps: []string{"Open the gallery; open its navigation drawer; rotate the phone."},
		Expect: []string{
			"The app fills the whole screen, with no black bands (#1016).",
			"The drawer reaches the screen's edge and leaves a strip to tap beside it (#1238).",
		},
	},
}
