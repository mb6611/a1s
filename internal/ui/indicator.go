// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package ui

import (
	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

// IndicatorMode represents the current input mode.
type IndicatorMode int

const (
	// ModeNormal is the default navigation mode.
	ModeNormal IndicatorMode = iota
	// ModeCommand is for entering commands (: prefix).
	ModeCommand
	// ModeFilter is for filtering resources (/ prefix).
	ModeFilter
)

// Mode indicators (emoji/icons).
const (
	IndicatorNormal  = "🐵"
	IndicatorCommand = "🐵"
	IndicatorFilter  = "🔍"
)

// CmdIndicator displays the current mode and accepts command/filter input.
type CmdIndicator struct {
	*tview.TextView

	mode      IndicatorMode
	text      string
	active    bool
	activeFn  func(bool)
	changeFn  func(string)
	executeFn func(string)
	cancelFn  func()
}

// NewCmdIndicator creates a new command indicator.
func NewCmdIndicator() *CmdIndicator {
	c := &CmdIndicator{
		TextView: tview.NewTextView(),
		mode:     ModeNormal,
	}

	c.SetDynamicColors(true)
	c.SetBackgroundColor(tcell.ColorDefault)
	c.SetTextColor(tcell.ColorWhite)
	c.refresh()

	return c
}

// SetActiveFn sets the callback when active state changes.
func (c *CmdIndicator) SetActiveFn(fn func(bool)) {
	c.activeFn = fn
}

// SetChangeFn sets the callback when text changes.
func (c *CmdIndicator) SetChangeFn(fn func(string)) {
	c.changeFn = fn
}

// SetExecuteFn sets the callback when command is executed.
func (c *CmdIndicator) SetExecuteFn(fn func(string)) {
	c.executeFn = fn
}

// SetCancelFn sets the callback when input is cancelled.
func (c *CmdIndicator) SetCancelFn(fn func()) {
	c.cancelFn = fn
}

// Activate enters command or filter mode.
func (c *CmdIndicator) Activate(mode IndicatorMode) {
	c.mode = mode
	c.text = ""
	c.active = true
	c.refresh()
	if c.activeFn != nil {
		c.activeFn(true)
	}
}

// Deactivate exits input mode.
func (c *CmdIndicator) Deactivate() {
	c.active = false
	c.mode = ModeNormal
	c.text = ""
	c.refresh()
	if c.activeFn != nil {
		c.activeFn(false)
	}
}

// IsActive returns whether input mode is active.
func (c *CmdIndicator) IsActive() bool {
	return c.active
}

// Mode returns the current mode.
func (c *CmdIndicator) Mode() IndicatorMode {
	return c.mode
}

// Text returns the current input text.
func (c *CmdIndicator) Text() string {
	return c.text
}

// SetText sets the input text.
func (c *CmdIndicator) SetText(text string) {
	c.text = text
	c.refresh()
}

// HandleKey processes keyboard input when active.
func (c *CmdIndicator) HandleKey(evt *tcell.EventKey) *tcell.EventKey {
	if !c.active {
		return evt
	}

	switch evt.Key() {
	case tcell.KeyEsc:
		c.Deactivate()
		if c.cancelFn != nil {
			c.cancelFn()
		}
		return nil

	case tcell.KeyEnter:
		text := c.text
		mode := c.mode
		c.Deactivate()
		if c.executeFn != nil && text != "" {
			if mode == ModeCommand {
				c.executeFn(":" + text)
			} else {
				c.executeFn(text)
			}
		}
		return nil

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(c.text) > 0 {
			c.text = c.text[:len(c.text)-1]
			c.refresh()
			if c.changeFn != nil {
				c.changeFn(c.text)
			}
		}
		return nil

	case tcell.KeyRune:
		c.text += string(evt.Rune())
		c.refresh()
		if c.changeFn != nil {
			c.changeFn(c.text)
		}
		return nil
	}

	return evt
}

// refresh updates the display.
func (c *CmdIndicator) refresh() {
	var indicator string
	var prefix string

	switch c.mode {
	case ModeCommand:
		indicator = IndicatorCommand
		prefix = ":"
	case ModeFilter:
		indicator = IndicatorFilter
		prefix = "/"
	default:
		indicator = IndicatorNormal
		prefix = ""
	}

	cursor := ""
	if c.active {
		cursor = "[black:white] [-:-]" // Block cursor with inverted colors
	}

	// Use simple format: emoji + prefix + text + cursor
	if c.active {
		display := indicator + prefix + c.text + cursor
		c.TextView.SetText(display)
	} else if prefix != "" || c.text != "" {
		display := indicator + prefix + c.text
		c.TextView.SetText(display)
	} else {
		c.TextView.SetText(indicator + ">")
	}
}

// Reset clears the indicator to normal mode.
func (c *CmdIndicator) Reset() {
	c.mode = ModeNormal
	c.text = ""
	c.active = false
	c.refresh()
}
