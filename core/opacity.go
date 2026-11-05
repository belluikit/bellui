package core

// OpacityGroup is implemented by an object whose whole subtree is drawn
// translucent. The paint walk multiplies the alpha of everything drawn for the
// object and its descendants by GroupOpacity, between 0 (not drawn) and 1
// (unchanged); nested groups multiply. Each draw is faded on its own, so where
// two descendants overlap, the one beneath shows through the one above.
//
// container.Opacity is the ready-made wrapper; a widget can implement it too.
// Change the opacity with a Refresh, as for any change to how an object draws.
type OpacityGroup interface {
	GroupOpacity() float32
}
