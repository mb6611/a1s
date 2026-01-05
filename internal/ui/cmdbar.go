// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package ui

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

// Default command suggestions for AWS resources
var defaultCommands = []string{
	"ec2",
	"s3",
	"vpc",
	"sg",
	"iam",
	"role",
	"eks",
	"vol",
	"profile",
	"region",
}

// CmdBar is a bordered command/filter input bar at the top of the app.
// Uses a TextView for ghost-text autocomplete like k9s.
type CmdBar struct {
	*tview.TextView

	mode              IndicatorMode
	cmdFn             func(string)
	filterFn          func(string)
	cancelFn          func()
	activeFn          func(bool)
	isActive          bool
	filterText        string
	text              []rune
	suggestions       []string
	suggestionIdx     int
	currentSuggestion string
	commands          []string
	mx                sync.RWMutex
}

// NewCmdBar creates a new command bar.
func NewCmdBar() *CmdBar {
	c := &CmdBar{
		TextView:      tview.NewTextView(),
		mode:          ModeNormal,
		commands:      defaultCommands,
		suggestionIdx: -1,
		text:          make([]rune, 0),
	}

	// Style the text view
	c.SetBorder(true)
	c.SetBorderColor(tcell.ColorDarkCyan)
	c.SetBackgroundColor(tcell.ColorDefault)
	c.SetTextColor(tcell.ColorWhite)
	c.SetDynamicColors(true)
	c.SetWrap(false)

	// Set up keyboard handler
	c.SetInputCapture(c.keyboard)

	// Show initial prompt
	c.render()

	return c
}

// keyboard handles all keyboard input for the command bar.
func (c *CmdBar) keyboard(evt *tcell.EventKey) *tcell.EventKey {
	if !c.isActive {
		return evt
	}

	switch evt.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		c.mx.Lock()
		if len(c.text) > 0 {
			c.text = c.text[:len(c.text)-1]
		}
		c.mx.Unlock()
		c.updateSuggestions()
		c.render()
		return nil

	case tcell.KeyDelete:
		c.mx.Lock()
		if len(c.text) > 0 {
			c.text = c.text[:len(c.text)-1]
		}
		c.mx.Unlock()
		c.updateSuggestions()
		c.render()
		return nil

	case tcell.KeyEnter:
		c.execute()
		return nil

	case tcell.KeyEsc:
		c.cancel()
		return nil

	case tcell.KeyTab, tcell.KeyRight:
		// Accept current suggestion
		if c.currentSuggestion != "" {
			c.mx.Lock()
			c.text = []rune(c.currentSuggestion)
			c.mx.Unlock()
			c.clearSuggestions()
			c.render()
		}
		return nil

	case tcell.KeyUp:
		// Cycle to previous suggestion
		c.mx.Lock()
		if len(c.suggestions) > 0 {
			c.suggestionIdx--
			if c.suggestionIdx < 0 {
				c.suggestionIdx = len(c.suggestions) - 1
			}
			c.currentSuggestion = c.suggestions[c.suggestionIdx]
		}
		c.mx.Unlock()
		c.render()
		return nil

	case tcell.KeyDown:
		// Cycle to next suggestion
		c.mx.Lock()
		if len(c.suggestions) > 0 {
			c.suggestionIdx++
			if c.suggestionIdx >= len(c.suggestions) {
				c.suggestionIdx = 0
			}
			c.currentSuggestion = c.suggestions[c.suggestionIdx]
		}
		c.mx.Unlock()
		c.render()
		return nil

	case tcell.KeyCtrlU, tcell.KeyCtrlW:
		// Clear the line
		c.mx.Lock()
		c.text = c.text[:0]
		c.mx.Unlock()
		c.clearSuggestions()
		c.render()
		return nil

	case tcell.KeyRune:
		r := evt.Rune()
		c.mx.Lock()
		c.text = append(c.text, r)
		c.mx.Unlock()
		c.updateSuggestions()
		c.render()
		// Also trigger filter callback for live filtering
		if c.mode == ModeFilter && c.filterFn != nil {
			c.filterFn(c.GetText())
		}
		return nil
	}

	return evt
}

// render updates the display with current text and ghost suggestion.
func (c *CmdBar) render() {
	c.mx.RLock()
	text := string(c.text)
	suggestion := c.currentSuggestion
	mode := c.mode
	c.mx.RUnlock()

	c.Clear()

	var icon, prefix string
	switch mode {
	case ModeCommand:
		icon = IndicatorCommand
		prefix = ":"
	case ModeFilter:
		icon = IndicatorFilter
		prefix = "/"
	default:
		icon = IndicatorNormal
		prefix = ">"
	}

	// Build the display string with ghost text
	var display string
	if suggestion != "" && strings.HasPrefix(suggestion, text) && len(suggestion) > len(text) {
		// Show typed text in white, remainder as ghost text in gray
		ghost := suggestion[len(text):]
		display = fmt.Sprintf("%s%s [::b]%s[gray::]%s[-::]", icon, prefix, text, ghost)
	} else {
		display = fmt.Sprintf("%s%s [::b]%s", icon, prefix, text)
	}

	fmt.Fprint(c.TextView, display)
}

// getSuggestions returns matching commands for the given text.
func (c *CmdBar) getSuggestions(text string) []string {
	if text == "" {
		return nil
	}

	text = strings.ToLower(text)
	var matches []string
	for _, cmd := range c.commands {
		if strings.HasPrefix(cmd, text) {
			matches = append(matches, cmd)
		}
	}
	sort.Strings(matches)
	return matches
}

// updateSuggestions updates the suggestion list based on current text.
func (c *CmdBar) updateSuggestions() {
	c.mx.Lock()
	defer c.mx.Unlock()

	text := string(c.text)
	if c.mode != ModeCommand || text == "" {
		c.suggestions = nil
		c.suggestionIdx = -1
		c.currentSuggestion = ""
		return
	}

	c.suggestions = c.getSuggestions(text)
	c.suggestionIdx = 0

	if len(c.suggestions) > 0 {
		c.currentSuggestion = c.suggestions[0]
	} else {
		c.currentSuggestion = ""
	}
}

// clearSuggestions clears all suggestions.
func (c *CmdBar) clearSuggestions() {
	c.mx.Lock()
	defer c.mx.Unlock()

	c.suggestions = nil
	c.suggestionIdx = -1
	c.currentSuggestion = ""
}

// AddCommands adds additional commands to the suggestion list.
func (c *CmdBar) AddCommands(cmds []string) {
	c.mx.Lock()
	defer c.mx.Unlock()

	for _, cmd := range cmds {
		found := false
		for _, existing := range c.commands {
			if existing == cmd {
				found = true
				break
			}
		}
		if !found {
			c.commands = append(c.commands, cmd)
		}
	}
	sort.Strings(c.commands)
}

// SetCommands sets the full list of available commands.
func (c *CmdBar) SetCommands(cmds []string) {
	c.mx.Lock()
	defer c.mx.Unlock()

	c.commands = cmds
	sort.Strings(c.commands)
}

// GetText returns the current input text.
func (c *CmdBar) GetText() string {
	c.mx.RLock()
	defer c.mx.RUnlock()
	return string(c.text)
}

// SetText sets the input text.
func (c *CmdBar) SetText(s string) {
	c.mx.Lock()
	c.text = []rune(s)
	c.mx.Unlock()
	c.render()
}

// Activate enters command or filter mode.
func (c *CmdBar) Activate(mode IndicatorMode) {
	c.mx.Lock()
	c.mode = mode
	c.isActive = true
	c.text = c.text[:0]
	c.mx.Unlock()
	c.clearSuggestions()
	c.render()

	if c.activeFn != nil {
		c.activeFn(true)
	}
}

// Deactivate exits input mode and returns to normal.
func (c *CmdBar) Deactivate() {
	c.mx.Lock()
	c.isActive = false
	c.mode = ModeNormal
	c.text = c.text[:0]
	c.mx.Unlock()
	c.clearSuggestions()
	c.render()

	if c.activeFn != nil {
		c.activeFn(false)
	}
}

// execute runs the command or confirms the filter.
func (c *CmdBar) execute() {
	text := c.GetText()

	switch c.mode {
	case ModeCommand:
		if c.cmdFn != nil && text != "" {
			c.cmdFn(":" + text)
		}
	case ModeFilter:
		c.filterText = text
		// Filter is already applied via ChangedFunc
	}

	c.Deactivate()
}

// cancel aborts the current input.
func (c *CmdBar) cancel() {
	if c.mode == ModeFilter && c.cancelFn != nil {
		c.cancelFn()
	}
	c.Deactivate()
}

// IsActive returns whether the command bar is accepting input.
func (c *CmdBar) IsActive() bool {
	return c.isActive
}

// Mode returns the current mode.
func (c *CmdBar) Mode() IndicatorMode {
	return c.mode
}

// SetCommandFn sets the callback for command execution.
func (c *CmdBar) SetCommandFn(fn func(string)) {
	c.cmdFn = fn
}

// SetFilterFn sets the callback for filter text changes.
func (c *CmdBar) SetFilterFn(fn func(string)) {
	c.filterFn = fn
}

// SetCancelFn sets the callback for when filter is cancelled.
func (c *CmdBar) SetCancelFn(fn func()) {
	c.cancelFn = fn
}

// SetActiveFn sets the callback for when active state changes.
func (c *CmdBar) SetActiveFn(fn func(bool)) {
	c.activeFn = fn
}

// GetFilterText returns the current filter text (if filter was confirmed).
func (c *CmdBar) GetFilterText() string {
	return c.filterText
}

// ClearFilter clears the filter text.
func (c *CmdBar) ClearFilter() {
	c.filterText = ""
	if c.filterFn != nil {
		c.filterFn("")
	}
}

// UpdatePrompt updates the display to show current state.
func (c *CmdBar) UpdatePrompt(resource, region string, count int) {
	if !c.isActive {
		// Re-render to show current state
		c.render()
	}
}
