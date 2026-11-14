package core

// FitWidther is implemented by an object whose minimum width is far below the
// width it would like, as text that wraps or truncates is: its MinSize is one
// character or an ellipsis wide. FitWidth is the width it would like, such as
// its text on one line.
//
// A horizontal box gives a child it does not stretch its FitWidth, and takes
// width back toward the child's MinSize only when the row runs out of room
// (#1195). CenterLayout and a FlexVBoxLayout that does not stretch its
// children give it its FitWidth up to their own width (#1198), and so does
// RowWrapLayout (#1199). FormLayout widens its label column toward a label's
// FitWidth, to no more than half its width (#1202). A FitWidth at or below the MinSize width asks for
// nothing more.
// Editable text does not implement it: an entry would grow as it is typed in.
//
// Since: 1.3
type FitWidther interface {
	FitWidth() float32
}
