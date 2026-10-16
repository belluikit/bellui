package widget

import (
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/binding"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/theme"
)

var (
	_ core.Widget     = (*Label)(nil)
	_ core.Accessible = (*Label)(nil)

	// Label must NOT implement event.Hoverable directly — hover is dispatched
	// to the deepest Hoverable only, and a Hoverable Label would swallow hover
	// from enclosing rows (TreeView, List). The truncation tooltip uses
	// labelHoverOverlay instead, added only when ShowTooltipWhenTruncated is set.
	_ event.Hoverable = (*labelHoverOverlay)(nil)
)

// selectableCanvas is implemented by canvases that support a window-level selectable mode.
type selectableCanvas interface {
	SelectableMode() bellui.SelectableMode
}

// Label widget is a label component with appropriate padding and layout.
type Label struct {
	BaseWidget
	Text      string
	Alignment bellui.TextAlign // The alignment of the text
	Wrapping  bellui.TextWrap  // The wrapping of the text
	TextStyle bellui.TextStyle // The style of the label text

	// The truncation mode of the text
	//
	// Since: 1.0
	Truncation bellui.TextTruncation
	// Importance informs how the label should be styled, i.e. warning or disabled
	//
	// Since: 1.0
	Importance Importance

	// The theme size name for the text size of the label
	//
	// Since: 1.0
	SizeName bellui.ThemeSizeName

	// If set to true, Selectable indicates that this label should support select interaction
	// to allow the text to be copied.
	//
	//Since: 1.0
	Selectable bool

	// ShowTooltipWhenTruncated, when true, shows a hover tooltip with the full
	// text when the label is truncated. Only effective when Truncation is set to
	// a non-off mode (e.g. TextTruncateEllipsis or TextTruncateClip).
	//
	// Since: 1.0
	ShowTooltipWhenTruncated bool

	provider  *RichText
	binder    basicBinder
	selection *focusSelectable

	tooltipMu      sync.Mutex
	tooltipPopUp   *PopUp
	tooltipTimer   *time.Timer
	tooltipOverlay *labelHoverOverlay
}

// NewLabel creates a new label widget with the set text content
func NewLabel(text string) *Label {
	return NewLabelWithStyle(text, bellui.TextAlignLeading, bellui.TextStyle{})
}

// NewLabelWithData returns a Label widget connected to the specified data source.
//
// Since: 1.0
func NewLabelWithData(data binding.String) *Label {
	label := NewLabel("")
	label.Bind(data)

	return label
}

// NewLabelWithStyle creates a new label widget with the set text content
func NewLabelWithStyle(text string, alignment bellui.TextAlign, style bellui.TextStyle) *Label {
	l := &Label{
		Text:      text,
		Alignment: alignment,
		TextStyle: style,
	}

	l.ExtendBaseWidget(l)
	return l
}

// AccessibleRole returns [core.RoleLabel].
func (l *Label) AccessibleRole() core.AccessibleRole { return core.RoleLabel }

// AccessibleLabel returns the label text.
func (l *Label) AccessibleLabel() string { return l.Text }

var _ core.AccessibleTexter = (*Label)(nil)

// AccessibleText returns the label text for assistive technologies (pre-rebrand tracker #556):
// together with the selection offsets below it lets a screen reader read a
// selectable label's content, selection and caret, not just its static value.
func (l *Label) AccessibleText() string { return l.Text }

// AccessibleSelection returns the user selection as a half-open rune range;
// when nothing is selected both offsets are the cursor position. Always
// (0, 0) while the label is not selectable.
func (l *Label) AccessibleSelection() (start, end int) {
	if !l.isSelectable() || l.selection == nil {
		return 0, 0
	}
	return l.selection.accessibleSelection()
}

// AccessibleCaret returns the rune offset of the selection cursor.
func (l *Label) AccessibleCaret() int {
	if !l.isSelectable() || l.selection == nil {
		return 0
	}
	return l.selection.accessibleCaret()
}

// SetAccessibleSelection selects [start, end), clamped to the content; equal
// offsets clear the selection. A no-op while the label is not selectable.
// It is safe to call from any goroutine.
func (l *Label) SetAccessibleSelection(start, end int) {
	if !l.isSelectable() || l.selection == nil {
		return
	}
	onMain(func() {
		l.selection.setAccessibleSelection(start, end, l.Wrapping)
	})
}

// ReplaceAccessibleSelection is a no-op: labels are not editable.
func (l *Label) ReplaceAccessibleSelection(string) {}

// Bind connects the specified data source to this Label.
// The current value will be displayed and any changes in the data will cause the widget to update.
//
// Since: 1.0
func (l *Label) Bind(data binding.String) {
	l.binder.SetCallback(l.updateFromData) // This could only be done once, maybe in ExtendBaseWidget?
	l.binder.Bind(data)
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer
func (l *Label) CreateRenderer() core.WidgetRenderer {
	l.provider = NewRichTextWithText(l.Text)
	l.ExtendBaseWidget(l)
	l.syncSegments()

	l.selection = &focusSelectable{}
	l.selection.ExtendBaseWidget(l.selection)
	l.selection.focus = l.selection
	l.selection.style = l.TextStyle
	l.selection.sizeName = l.SizeName
	l.selection.alignment = l.Alignment
	l.selection.theme = l.Theme()
	l.selection.provider = l.provider

	if l.ShowTooltipWhenTruncated {
		l.tooltipOverlay = newLabelHoverOverlay(l)
	}

	return &labelRenderer{l}
}

// MinSize returns the size that this label should not shrink below.
func (l *Label) MinSize() geo.Size {
	l.ExtendBaseWidget(l)
	return l.BaseWidget.MinSize()
}

// IntrinsicSize reports the size the label wants at unlimited width: its natural
// extent measured as if Wrapping and Truncation were off, plus padding. It
// mirrors [RichText.IntrinsicSize] (the label wraps a RichText) for content
// autosizing: a wrapping or truncating label's [Label.MinSize] reports a width of
// ~0, so use IntrinsicSize to size a container, popover or tooltip to the
// content. When Wrapping and Truncation are off it equals MinSize.
//
// Since: 1.0
func (l *Label) IntrinsicSize() geo.Size {
	l.ExtendBaseWidget(l)
	cache.Renderer(l.super()) // the RichText provider is created lazily on first render
	if l.provider == nil {
		return l.MinSize()
	}
	l.syncSegments() // push the current Text/Wrapping/Truncation/style before measuring
	return l.provider.IntrinsicSize()
}

// SyncState syncs Label's public fields to internal rendering state.
// Called automatically by BaseWidget.Refresh() before the renderer refreshes.
func (l *Label) SyncState() {
	if l.provider == nil { // not created until visible
		return
	}
	l.syncSegments()
	l.provider.Refresh()
}

// SelectedText returns the text currently selected in this Label.
// If the label is not selectable (considering both the widget property and the
// window-level [SelectableMode]) it will return an empty string.
// If there is no selection it will return the empty string.
//
// Since: 1.0
func (l *Label) SelectedText() string {
	if !l.isSelectable() || l.selection == nil {
		return ""
	}

	return l.selection.SelectedText()
}

// isSelectable returns whether this label should currently be selectable,
// considering both the widget's own Selectable property and the window-level SelectableMode.
func (l *Label) isSelectable() bool {
	c := bellui.CurrentApp().Driver().CanvasForObject(l)
	if sc, ok := c.(selectableCanvas); ok {
		switch sc.SelectableMode() {
		case bellui.SelectableAll:
			return true
		case bellui.SelectableNone:
			return false
		}
	}
	return l.Selectable
}

// SetText sets the text of the label.
// It is safe to call from any goroutine.
func (l *Label) SetText(text string) {
	onMain(func() {
		l.Text = text
		l.Refresh()
	})
}

// Unbind disconnects any configured data source from this Label.
// The current value will remain at the last value of the data source.
//
// Since: 1.0
func (l *Label) Unbind() {
	l.binder.Unbind()
}

func (l *Label) syncSegments() {
	var color bellui.ThemeColorName
	switch l.Importance {
	case LowImportance:
		color = theme.ColorNameDisabled
	case MediumImportance:
		color = theme.ColorNameForeground
	case HighImportance:
		color = theme.ColorNamePrimary
	case DangerImportance:
		color = theme.ColorNameError
	case WarningImportance:
		color = theme.ColorNameWarning
	case SuccessImportance:
		color = theme.ColorNameSuccess
	default:
		color = theme.ColorNameForeground
	}

	sizeName := l.SizeName
	if sizeName == "" {
		sizeName = theme.SizeNameText
	}
	l.provider.Wrapping = l.Wrapping
	l.provider.Truncation = l.Truncation
	seg := l.provider.Segments[0].(*TextSegment)
	newStyle := RichTextStyle{
		Alignment: l.Alignment,
		ColorName: color,
		Inline:    true,
		TextStyle: l.TextStyle,
		SizeName:  sizeName,
	}
	if seg.Text != l.Text || seg.Style != newStyle {
		l.provider.rowBoundsVersion++ // invalidate row bounds cache after content change
	}
	seg.Style = newStyle
	seg.Text = l.Text
}

func (l *Label) updateFromData(data binding.DataItem) {
	if data == nil {
		return
	}
	textSource, ok := data.(binding.String)
	if !ok {
		return
	}
	val, err := textSource.Get()
	if err != nil {
		bellui.LogError("Error getting current data value", err)
		return
	}
	l.SetText(val)
}

// ---------------------------------------------------------------------------
// Hoverable — truncation tooltip
// ---------------------------------------------------------------------------

const labelTooltipDelay = 500 * time.Millisecond

// hoverIn is called via the labelHoverOverlay when the mouse enters the
// label area. If ShowTooltipWhenTruncated is set and the text is currently
// truncated, a tooltip with the full text is shown after a short delay.
//
// Label deliberately does not implement event.Hoverable itself: hover is
// dispatched to the deepest Hoverable only, so a statically Hoverable Label
// would swallow hover events from enclosing rows (TreeView, List) even when
// the tooltip feature is disabled. The overlay is only added to the render
// tree when ShowTooltipWhenTruncated is set.
func (l *Label) hoverIn(_ *event.HoverEvent) {
	if !l.ShowTooltipWhenTruncated || l.Truncation == bellui.TextTruncateOff || l.Text == "" {
		return
	}
	if !l.isTruncated() {
		return
	}
	l.startTooltipTimer()
}

// hoverOut is called via the labelHoverOverlay when the mouse leaves the label area.
func (l *Label) hoverOut() {
	l.cancelTooltipTimer()
	l.hideTooltip()
}

// isTruncated returns true if the label's full text is wider than the
// current widget size, meaning the text is being truncated.
func (l *Label) isTruncated() bool {
	if l.Text == "" {
		return false
	}
	th := l.Theme()
	sizeName := l.SizeName
	if sizeName == "" {
		sizeName = theme.SizeNameText
	}
	textSize := bellui.MeasureText(l.Text, th.Size(sizeName), l.TextStyle)
	innerPad := th.Size(theme.SizeNameInnerPadding)
	return textSize.Width+innerPad*2 > l.Size().Width
}

func (l *Label) startTooltipTimer() {
	l.cancelTooltipTimer()
	l.tooltipMu.Lock()
	l.tooltipTimer = time.AfterFunc(labelTooltipDelay, func() {
		bellui.Do(func() { l.showTooltip() })
	})
	l.tooltipMu.Unlock()
}

func (l *Label) cancelTooltipTimer() {
	l.tooltipMu.Lock()
	if l.tooltipTimer != nil {
		l.tooltipTimer.Stop()
		l.tooltipTimer = nil
	}
	l.tooltipMu.Unlock()
}

func (l *Label) showTooltip() {
	c := bellui.CurrentApp().Driver().CanvasForObject(l)
	if c == nil {
		return
	}
	l.hideTooltip()

	th := l.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	bg := canvas.NewRectangle(th.Color(theme.ColorNameForeground, v))
	bg.CornerRadius = th.Size(theme.SizeNameInputRadius)
	fg := canvas.NewText(l.Text, th.Color(theme.ColorNameBackground, v))
	fg.TextSize = th.Size(theme.SizeNameCaptionText)
	fg.TextStyle = l.TextStyle

	pad := th.Size(theme.SizeNamePadding)
	content := &bellui.Container{
		Objects: []core.CanvasObject{bg, fg},
		Layout:  &tooltipContentLayout{padding: pad},
	}

	l.tooltipMu.Lock()
	l.tooltipPopUp = newPopUpWithBgColor(content, c, th.Color(theme.ColorNameForeground, v))
	l.tooltipMu.Unlock()

	absPos := bellui.CurrentApp().Driver().AbsolutePositionForObject(l)
	l.tooltipPopUp.ShowAtPosition(geo.NewPos(absPos.X, absPos.Y+l.Size().Height))
}

func (l *Label) hideTooltip() {
	l.tooltipMu.Lock()
	p := l.tooltipPopUp
	l.tooltipPopUp = nil
	l.tooltipMu.Unlock()
	if p != nil {
		p.Hide()
	}
}

// labelHoverOverlay is a transparent widget placed on top of the Label's
// children in the render tree. Because Bellui dispatches hover events to the
// deepest Hoverable widget at the mouse position, this overlay intercepts
// hover before the RichText provider (which is not Hoverable) absorbs the
// hit-test, and delegates to the parent Label.
type labelHoverOverlay struct {
	BaseWidget
	label *Label
}

var _ event.Hoverable = (*labelHoverOverlay)(nil)

func newLabelHoverOverlay(l *Label) *labelHoverOverlay {
	o := &labelHoverOverlay{label: l}
	o.ExtendBaseWidget(o)
	return o
}

func (o *labelHoverOverlay) CreateRenderer() core.WidgetRenderer {
	o.ExtendBaseWidget(o)
	return NewSimpleRenderer(canvas.NewRectangle(nil))
}

func (o *labelHoverOverlay) HoverIn(e *event.HoverEvent)    { o.label.hoverIn(e) }
func (o *labelHoverOverlay) HoverMoved(_ *event.HoverEvent) {}
func (o *labelHoverOverlay) HoverOut()                      { o.label.hoverOut() }

type labelRenderer struct {
	l *Label
}

func (r *labelRenderer) Destroy() {
	// Labels are recycled in List/Tree/Table rows on scroll. If a row is
	// destroyed while the truncation-tooltip timer is pending, cancel it so it
	// doesn't fire on a removed widget, and hide any tooltip already shown so
	// it doesn't orphan an overlay (the overlay's hoverOut would never run).
	if !r.l.ShowTooltipWhenTruncated {
		return
	}
	r.l.cancelTooltipTimer()
	r.l.hideTooltip()
}

func (r *labelRenderer) Layout(s geo.Size) {
	r.l.selection.Resize(s)
	r.l.provider.Resize(s)
	if r.l.tooltipOverlay != nil {
		r.l.tooltipOverlay.Resize(s)
		r.l.tooltipOverlay.Move(geo.NewPos(0, 0))
	}
}

func (r *labelRenderer) MinSize() geo.Size {
	return r.l.provider.MinSize()
}

func (r *labelRenderer) Objects() []core.CanvasObject {
	var objs []core.CanvasObject
	if r.l.isSelectable() {
		objs = append(objs, r.l.selection)
	}
	objs = append(objs, r.l.provider)
	if r.l.tooltipOverlay != nil {
		objs = append(objs, r.l.tooltipOverlay)
	}
	return objs
}

func (r *labelRenderer) Refresh() {
	r.l.syncSegments()
	r.l.provider.Refresh()

	sel := r.l.selection
	if !r.l.isSelectable() || sel == nil {
		return
	}

	sel.sizeName = r.l.SizeName
	sel.style = r.l.TextStyle
	sel.alignment = r.l.Alignment
	sel.theme = r.l.Theme()
	sel.Refresh()
}

type focusSelectable struct {
	selectable
}

// Disabled returns true so that the focus manager skips this widget during
// tab traversal, while still allowing direct focus via mouse click for
// text selection (the focus manager has a special case for disabled+SelectedText).
func (f *focusSelectable) Disabled() bool {
	return true
}

func (f *focusSelectable) Disable() {
}

func (f *focusSelectable) Enable() {
}

func (f *focusSelectable) FocusGained() {
	f.focussed = true
	f.Refresh()
}

func (f *focusSelectable) FocusLost() {
	f.focussed = false
	f.Refresh()
}

func (f *focusSelectable) TypedKey(*event.KeyEvent) {
}

func (f *focusSelectable) TypedRune(rune) {
}
