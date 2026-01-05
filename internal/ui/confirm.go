// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package ui

import (
	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

// ConfirmFunc is called when user confirms action.
type ConfirmFunc func()

// Confirm represents a confirmation dialog.
type Confirm struct {
	*tview.Modal
	confirmed bool
	dangerous bool
	onConfirm ConfirmFunc
	onCancel  func()
	pages     *Pages
	pageID    string
}

// NewConfirm creates a new confirmation dialog.
func NewConfirm(pages *Pages) *Confirm {
	c := &Confirm{
		Modal:  tview.NewModal(),
		pages:  pages,
		pageID: "confirm-dialog",
	}

	c.SetBackgroundColor(tcell.ColorDefault)
	c.AddButtons([]string{"Yes", "No"})
	c.SetDoneFunc(c.handleButton)

	return c
}

// SetTitle sets the dialog title.
func (c *Confirm) SetTitle(title string) *Confirm {
	c.Modal.SetText(title)
	return c
}

// SetMessage sets the confirmation message.
func (c *Confirm) SetMessage(msg string) *Confirm {
	c.Modal.SetText(msg)
	return c
}

// SetDangerous styles the dialog for dangerous operations.
func (c *Confirm) SetDangerous(dangerous bool) *Confirm {
	c.dangerous = dangerous
	c.updateStyle()
	return c
}

// SetOnConfirm sets the callback for when user confirms.
func (c *Confirm) SetOnConfirm(fn ConfirmFunc) *Confirm {
	c.onConfirm = fn
	return c
}

// SetOnCancel sets the callback for when user cancels.
func (c *Confirm) SetOnCancel(fn func()) *Confirm {
	c.onCancel = fn
	return c
}

// Show displays the dialog.
func (c *Confirm) Show() {
	if c.pages != nil {
		c.pages.AddPage(c.pageID, c, true, true)
	}
}

// Dismiss removes the dialog.
func (c *Confirm) Dismiss() {
	if c.pages != nil {
		c.pages.RemovePage(c.pageID)
	}
}

// handleButton processes button clicks.
func (c *Confirm) handleButton(buttonIndex int, buttonLabel string) {
	c.Dismiss()

	switch buttonIndex {
	case 0: // Yes
		c.confirmed = true
		if c.onConfirm != nil {
			c.onConfirm()
		}
	case 1: // No
		c.confirmed = false
		if c.onCancel != nil {
			c.onCancel()
		}
	}
}

// updateStyle applies styling based on danger level.
func (c *Confirm) updateStyle() {
	if c.dangerous {
		c.SetTextColor(tcell.ColorRed)
		c.SetButtonBackgroundColor(tcell.ColorRed)
		c.SetButtonTextColor(tcell.ColorWhite)
	} else {
		c.SetTextColor(tcell.ColorWhite)
		c.SetButtonBackgroundColor(tcell.ColorBlue)
		c.SetButtonTextColor(tcell.ColorWhite)
	}
}

// IsConfirmed returns true if user confirmed the action.
func (c *Confirm) IsConfirmed() bool {
	return c.confirmed
}
