// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2024 a1s Contributors

package view

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/a1s/a1s/internal/config"
	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/ui"
	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

const (
	// FlashDelay sets the flash auto-clear delay.
	FlashDelay = 5 * time.Second
)

// FlashLevel represents flash message severity.
type FlashLevel int

const (
	// FlashInfo represents an info message.
	FlashInfo FlashLevel = iota
	// FlashWarn represents a warning message.
	FlashWarn
	// FlashErr represents an error message.
	FlashErr
)

// Flash handles flash messages in the application.
type Flash struct {
	*tview.TextView
	app    *App
	cancel context.CancelFunc
	mx     sync.RWMutex
}

// NewFlash creates a new Flash instance.
func NewFlash(app *App) *Flash {
	f := &Flash{
		TextView: tview.NewTextView(),
		app:      app,
	}
	f.SetDynamicColors(true)
	f.SetTextAlign(tview.AlignLeft)
	f.SetBorderPadding(0, 0, 1, 1)
	return f
}

// Info displays an informational message.
func (f *Flash) Info(msg string) {
	f.setMessage(FlashInfo, msg)
}

// Infof displays a formatted informational message.
func (f *Flash) Infof(format string, args ...interface{}) {
	f.Info(fmt.Sprintf(format, args...))
}

// Warn displays a warning message.
func (f *Flash) Warn(msg string) {
	f.setMessage(FlashWarn, msg)
}

// Warnf displays a formatted warning message.
func (f *Flash) Warnf(format string, args ...interface{}) {
	f.Warn(fmt.Sprintf(format, args...))
}

// Err displays an error message.
func (f *Flash) Err(err error) {
	if err != nil {
		f.setMessage(FlashErr, err.Error())
	}
}

// Errf displays a formatted error message.
func (f *Flash) Errf(format string, args ...interface{}) {
	f.setMessage(FlashErr, fmt.Sprintf(format, args...))
}

// Clear clears the flash message.
func (f *Flash) Clear() {
	f.mx.Lock()
	if f.cancel != nil {
		f.cancel()
		f.cancel = nil
	}
	f.mx.Unlock()

	if f.app != nil {
		f.app.QueueUpdateDraw(func() {
			f.TextView.Clear()
		})
	} else {
		f.TextView.Clear()
	}
}

func (f *Flash) setMessage(level FlashLevel, msg string) {
	f.mx.Lock()
	// Cancel any existing auto-clear timer
	if f.cancel != nil {
		f.cancel()
		f.cancel = nil
	}
	f.mx.Unlock()

	if msg == "" {
		f.Clear()
		return
	}

	// Update UI with message
	updateFn := func() {
		f.TextView.Clear()
		f.SetTextColor(flashColor(level))
		fmt.Fprintf(f.TextView, "%s %s", flashPrefix(level), msg)
	}

	if f.app != nil {
		f.app.QueueUpdateDraw(updateFn)
	} else {
		updateFn()
	}

	// Start auto-clear timer
	ctx, cancel := context.WithCancel(context.Background())
	f.mx.Lock()
	f.cancel = cancel
	f.mx.Unlock()

	go f.autoClear(ctx)
}

func (f *Flash) autoClear(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(FlashDelay):
		f.Clear()
	}
}

func flashColor(level FlashLevel) tcell.Color {
	switch level {
	case FlashWarn:
		return tcell.ColorYellow
	case FlashErr:
		return tcell.ColorRed
	default:
		return tcell.ColorGreen
	}
}

func flashPrefix(level FlashLevel) string {
	switch level {
	case FlashWarn:
		return "[WARN]"
	case FlashErr:
		return "[ERROR]"
	default:
		return "[INFO]"
	}
}

// PageStack is a type alias for the view stack.
type PageStack = ui.Pages

// App represents the main application container.
type App struct {
	*tview.Application
	version     string
	Main        *tview.Pages
	Content     *PageStack
	command     *Command
	factory     dao.Factory
	cmdBar      *ui.CmdBar
	menu        *ui.Menu
	crumbs      *ui.Crumbs
	flash       *Flash
	help        *Help
	running     bool
	mx          sync.RWMutex
}

// NewApp creates a new application instance.
func NewApp(cfg *config.Config, version string) *App {
	app := &App{
		Application: tview.NewApplication(),
		version:     version,
		Main:        tview.NewPages(),
		Content:     ui.NewPages(),
	}

	app.flash = NewFlash(app)
	app.menu = ui.NewMenu()
	app.crumbs = ui.NewCrumbs()
	app.cmdBar = ui.NewCmdBar()
	app.help = NewHelp()

	// Setup keyboard handler
	app.Application.SetInputCapture(app.keyboard)

	// Setup command bar callbacks
	app.cmdBar.SetActiveFn(func(active bool) {
		if active {
			app.SetFocus(app.cmdBar)
		} else {
			app.SetFocus(app.Content)
		}
	})

	app.cmdBar.SetCommandFn(func(cmd string) {
		if err := app.command.Run(cmd); err != nil {
			app.flash.Errf("Command error: %v", err)
		}
	})

	app.cmdBar.SetFilterFn(func(text string) {
		// Apply filter to current view
		app.applyFilter(text)
	})

	app.cmdBar.SetCancelFn(func() {
		// Clear filter
		app.applyFilter("")
	})

	return app
}

// Init initializes and builds the application layout.
func (a *App) Init() error {
	// Initialize command interpreter
	a.command = NewCommand(a)
	if err := a.command.Init(); err != nil {
		return fmt.Errorf("failed to initialize command: %w", err)
	}

	// Build layout
	layout := a.buildLayout()
	a.Main.AddPage("main", layout, true, true)
	a.SetRoot(a.Main, true)
	a.SetFocus(a.Content)

	return nil
}

// Run starts the application.
func (a *App) Run() error {
	a.mx.Lock()
	a.running = true
	a.mx.Unlock()

	// Execute default command to show initial view
	if err := a.command.Run(""); err != nil {
		// Log error but don't fail - app can still run
		a.flash.Errf("Failed to run default command: %v", err)
	}

	return a.Application.Run()
}

// Stop stops the application.
func (a *App) Stop() {
	a.mx.Lock()
	defer a.mx.Unlock()

	a.running = false
	a.Application.Stop()
}

// IsRunning returns whether the application is currently running.
func (a *App) IsRunning() bool {
	a.mx.RLock()
	defer a.mx.RUnlock()

	return a.running
}

// Flash returns the flash message handler.
func (a *App) Flash() *Flash {
	return a.flash
}

// GetFactory returns the AWS factory.
func (a *App) GetFactory() dao.Factory {
	a.mx.RLock()
	defer a.mx.RUnlock()

	return a.factory
}

// SetFactory sets the AWS factory.
func (a *App) SetFactory(f dao.Factory) {
	a.mx.Lock()
	defer a.mx.Unlock()

	a.factory = f
}

// SwitchProfile switches to a different AWS profile.
func (a *App) SwitchProfile(profile string) error {
	a.mx.Lock()
	defer a.mx.Unlock()

	if a.factory == nil {
		return fmt.Errorf("factory not initialized")
	}

	if err := a.factory.SetProfile(profile); err != nil {
		return fmt.Errorf("failed to switch profile: %w", err)
	}

	// Verify connectivity with new profile
	if client := a.factory.Client(); client != nil {
		if !client.CheckConnectivity() {
			return fmt.Errorf("failed to connect with profile: %s", profile)
		}
	}

	return nil
}

// SwitchRegion switches to a different AWS region.
func (a *App) SwitchRegion(region string) error {
	a.mx.Lock()
	defer a.mx.Unlock()

	if a.factory == nil {
		return fmt.Errorf("factory not initialized")
	}

	return a.factory.SetRegion(region)
}

// QueueUpdateDraw queues a function to be executed on the UI thread.
func (a *App) QueueUpdateDraw(fn func()) {
	go a.Application.QueueUpdateDraw(fn)
}

// ClearStatus clears status messages and optionally shows the logo.
func (a *App) ClearStatus(showLogo bool) {
	a.flash.Clear()
	// TODO: Implement logo display logic
}

// SetAccountInfo sets the account information display.
// Currently a no-op since we removed the separate account info display.
func (a *App) SetAccountInfo(profile, region, accountID, version string) {
	// Account info could be shown in the flash bar or elsewhere
	a.flash.Infof("Profile: %s | Region: %s", profile, region)
}

// buildLayout creates the main UI layout.
func (a *App) buildLayout() *tview.Flex {
	// Bottom bar: flash messages and menu hints
	bottomBar := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.flash, 1, 0, false).
		AddItem(a.menu, 1, 0, false)

	// Main layout: command bar at top, content in middle, status at bottom
	main := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.cmdBar, 3, 0, false).
		AddItem(a.Content, 0, 1, true).
		AddItem(bottomBar, 2, 0, false)

	return main
}

// keyboard handles global keyboard events.
func (a *App) keyboard(evt *tcell.EventKey) *tcell.EventKey {
	// If help is showing, let it handle keys
	if name, _ := a.Content.GetFrontPage(); name == "help" {
		return evt
	}

	// If command bar is active, let it handle everything
	if a.cmdBar.IsActive() {
		return evt
	}

	// Handle global keys
	key := evt.Key()
	if key == tcell.KeyRune {
		switch evt.Rune() {
		case ':':
			a.cmdBar.Activate(ui.ModeCommand)
			return nil
		case '/':
			a.cmdBar.Activate(ui.ModeFilter)
			return nil
		case '?':
			a.showHelp()
			return nil
		case 'q':
			a.Stop()
			return nil
		}
	}

	// Handle special keys
	switch key {
	case tcell.KeyCtrlC:
		a.Stop()
		return nil
	case tcell.KeyCtrlR:
		a.refresh()
		return nil
	case tcell.KeyEsc:
		// Clear filter or go back
		if a.cmdBar.GetFilterText() != "" {
			a.cmdBar.ClearFilter()
			a.applyFilter("")
		} else {
			a.handleEscape()
		}
		return nil
	}

	// Pass to focused component
	return evt
}

// applyFilter applies filter to the current view.
func (a *App) applyFilter(filter string) {
	// Get the current view and apply filter if it supports it
	if a.Content == nil {
		return
	}

	current := a.Content.CurrentPage()
	if current == nil {
		return
	}

	// Check if the current view has a SetFilter method
	if filterable, ok := current.(interface{ SetFilter(string) }); ok {
		filterable.SetFilter(filter)
	}
}

// showHelp displays the help screen in the content area.
func (a *App) showHelp() {
	// Set close callback to remove the help page
	a.help.SetCloseFn(func() {
		a.Content.RemovePage("help")
		a.SetFocus(a.Content)
	})

	// Add help to content area (not full screen, keeps command bar visible)
	a.Content.AddPage("help", a.help, true, true)
	a.SetFocus(a.help)
}

// refresh refreshes the current view.
func (a *App) refresh() {
	a.RefreshCurrentView()
}

// RefreshCurrentView reloads data for the current view.
func (a *App) RefreshCurrentView() {
	if a.Content == nil {
		return
	}

	current := a.Content.CurrentPage()
	if current == nil {
		return
	}

	// Call Start() on the current view to reload data
	if startable, ok := current.(interface{ Start() }); ok {
		a.flash.Info("Refreshing...")
		startable.Start()
	}
}

// handleEscape handles the Escape key (go back/cancel).
func (a *App) handleEscape() {
	// If we have multiple pages, pop the top one
	if a.Content.StackSize() > 1 {
		a.Content.Pop()
	}
}
