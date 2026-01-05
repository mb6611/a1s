// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package ui

import (
	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

// DialogCallback is called when dialog is dismissed.
type DialogCallback func()

// Dialog represents a generic modal dialog base.
type Dialog struct {
	*tview.Modal
	pages  *Pages
	pageID string
	onDone DialogCallback
}

// NewDialog creates a new dialog.
func NewDialog(pages *Pages, pageID string) *Dialog {
	d := &Dialog{
		Modal:  tview.NewModal(),
		pages:  pages,
		pageID: pageID,
	}

	d.SetBackgroundColor(tcell.ColorDefault)
	d.SetTextColor(tcell.ColorWhite)

	return d
}

// SetTitle sets the dialog title.
func (d *Dialog) SetTitle(title string) *Dialog {
	d.Modal.SetText(title)
	return d
}

// SetMessage sets the dialog message.
func (d *Dialog) SetMessage(msg string) *Dialog {
	d.Modal.SetText(msg)
	return d
}

// SetButtons configures dialog buttons.
func (d *Dialog) SetButtons(labels []string) *Dialog {
	d.AddButtons(labels)
	return d
}

// SetDoneCallback sets the callback for when dialog closes.
func (d *Dialog) SetDoneCallback(fn DialogCallback) *Dialog {
	d.onDone = fn
	return d
}

// SetButtonHandler sets the button click handler.
func (d *Dialog) SetButtonHandler(handler func(int, string)) *Dialog {
	d.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		d.Dismiss()
		if handler != nil {
			handler(buttonIndex, buttonLabel)
		}
	})
	return d
}

// Show displays the dialog as a modal overlay.
func (d *Dialog) Show() {
	if d.pages != nil {
		d.pages.AddPage(d.pageID, d, true, true)
	}
}

// Dismiss removes the dialog from display.
func (d *Dialog) Dismiss() {
	if d.pages != nil {
		d.pages.RemovePage(d.pageID)
	}
	if d.onDone != nil {
		d.onDone()
	}
}

// SetColors configures dialog colors.
func (d *Dialog) SetColors(text, btnBg, btnText tcell.Color) *Dialog {
	d.SetTextColor(text)
	d.SetButtonBackgroundColor(btnBg)
	d.SetButtonTextColor(btnText)
	return d
}

// PageID returns the dialog's page identifier.
func (d *Dialog) PageID() string {
	return d.pageID
}

// InfoDialog creates a simple info dialog with OK button.
func InfoDialog(pages *Pages, title, message string) *Dialog {
	return NewDialog(pages, "info-dialog").
		SetTitle(title).
		SetMessage(message).
		SetButtons([]string{"OK"}).
		SetButtonHandler(func(_ int, _ string) {})
}

// ErrorDialog creates a styled error dialog.
func ErrorDialog(pages *Pages, title, message string) *Dialog {
	return NewDialog(pages, "error-dialog").
		SetTitle(title).
		SetMessage(message).
		SetButtons([]string{"OK"}).
		SetColors(tcell.ColorRed, tcell.ColorRed, tcell.ColorWhite).
		SetButtonHandler(func(_ int, _ string) {})
}

// WarningDialog creates a styled warning dialog.
func WarningDialog(pages *Pages, title, message string) *Dialog {
	return NewDialog(pages, "warning-dialog").
		SetTitle(title).
		SetMessage(message).
		SetButtons([]string{"OK"}).
		SetColors(tcell.ColorYellow, tcell.ColorYellow, tcell.ColorBlack).
		SetButtonHandler(func(_ int, _ string) {})
}
