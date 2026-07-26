// Package container provides containers that are used to lay out and organise applications.
package container

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
)

// New returns a new Container instance holding the specified CanvasObjects which will be laid out according to the specified Layout.
//
// Since: 2.0
func New(layout core.Layout, objects ...core.CanvasObject) *bellui.Container {
	return &bellui.Container{Layout: layout, Objects: objects}
}

// NewWithoutLayout returns a new Container instance holding the specified CanvasObjects that are manually arranged.
//
// Since: 2.0
func NewWithoutLayout(objects ...core.CanvasObject) *bellui.Container {
	return &bellui.Container{Objects: objects}
}
