package mobile

import (
	"os"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/async"
	_ "github.com/belluikit/bellui/test"
)

// This file is deliberately free of build tags: it owns TestMain, which
// installs the mobile test app every test in this package depends on.
// (canvas_test.go is excluded on windows+ci and must not host TestMain.)

var d *driver

func TestMain(m *testing.M) {
	currentApp := bellui.CurrentApp()
	tester := newTestMobileApp()
	d = tester.Driver().(*driver)
	d.queuedFuncs = async.NewUnboundedChan[func()]()
	bellui.SetCurrentApp(tester)

	waitForStart := make(chan struct{})
	go func() {
		// Wait for app loop to be running (plus a moment in case of scheduling switches).
		<-waitForStart

		// Just like the GLFW tests, wait a short while for the driver to start
		time.Sleep(time.Millisecond * 100)

		ret := m.Run()
		bellui.SetCurrentApp(currentApp)
		os.Exit(ret)
	}()

	close(waitForStart) // Signal that execution can continue.
	tester.Run()
}

type mobileApp struct {
	bellui.App
	driver bellui.Driver
}

func (a *mobileApp) Driver() bellui.Driver {
	return a.driver
}

func (a *mobileApp) Run() {
	// This is an incomplete driver loop - our CI does not currently support booting the mobile graphics
	// TODO replace with a full mobileApp.Run() once that is resolved
	async.SetMainGoroutine()

	for fn := range d.queuedFuncs.Out() {
		fn()
	}
}

func newTestMobileApp() bellui.App {
	return &mobileApp{
		App:    bellui.CurrentApp(),
		driver: NewGoMobileDriver(),
	}
}
