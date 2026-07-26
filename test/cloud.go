package test

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
)

type mockCloud struct {
	configured bool
}

func (c *mockCloud) Cleanup(_ bellui.App) {
	c.configured = false
}

func (c *mockCloud) ProviderDescription() string {
	return "Mock cloud implementation"
}

func (c *mockCloud) ProviderIcon() bellui.Resource {
	return theme.ComputerIcon()
}

func (c *mockCloud) ProviderName() string {
	return "mock"
}

func (c *mockCloud) Setup(_ bellui.App) error {
	c.configured = true
	return nil
}
