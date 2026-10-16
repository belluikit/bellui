package embedded

// Event is the general type of all embedded device events.
//
// Since: 1.0
type Event interface {
	isEvent()
}
