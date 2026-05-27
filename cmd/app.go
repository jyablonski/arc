package cmd

import (
	"github.com/jyablonski/arc/internal/deps"
	"github.com/jyablonski/arc/internal/hardware"
	"github.com/jyablonski/arc/internal/pkgmgr"
	"github.com/jyablonski/arc/internal/platform"
	"github.com/jyablonski/arc/internal/setupdeps"
	"github.com/jyablonski/arc/internal/syscontrol"
)

type App struct {
	Platform platform.OS
	PkgMgr   pkgmgr.Manager
	System   syscontrol.Controller
	Hardware hardware.Reporter
	Setup    setupdeps.Installer
	Tools    []deps.ToolStatus
}

var app = newApp(platform.Detect())

func newApp(os platform.OS) *App {
	return &App{
		Platform: os,
		PkgMgr:   pkgmgr.New(os),
		System:   syscontrol.New(os),
		Hardware: hardware.New(os),
		Setup:    setupdeps.New(os),
		Tools:    deps.Tools(os),
	}
}

func setAppForTest(testApp *App) func() {
	old := app
	app = testApp
	return func() {
		app = old
	}
}
