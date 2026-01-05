// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2025 a1s Authors

package view

import (
	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

// AccountInfo displays AWS account information in a table format.
type AccountInfo struct {
	*tview.Table

	profile   string
	region    string
	accountID string
	version   string
}

// NewAccountInfo creates a new account info display component.
func NewAccountInfo() *AccountInfo {
	a := &AccountInfo{
		Table: tview.NewTable(),
	}

	a.SetBorder(true)
	a.SetBorderColor(tcell.ColorDarkCyan)
	a.SetBorderPadding(0, 0, 1, 1)

	return a
}

// Init initializes the table display.
func (a *AccountInfo) Init() error {
	a.SetSelectable(false, false)
	a.refresh()
	return nil
}

// SetInfo updates the displayed account information.
func (a *AccountInfo) SetInfo(profile, region, accountID, version string) {
	a.profile = profile
	a.region = region
	a.accountID = accountID
	a.version = version
	a.refresh()
}

// refresh rebuilds the table display.
func (a *AccountInfo) refresh() {
	a.Clear()

	// Compact two-line format to fit in header
	// Line 1: profile @ region
	// Line 2: account (version)
	profile := a.profile
	if profile == "" {
		profile = "default"
	}
	region := a.region
	if region == "" {
		region = "us-east-1"
	}

	line1 := "[::b]" + profile + "[@::]" + region + "[-:-:-]"
	cell1 := tview.NewTableCell(line1).
		SetTextColor(tcell.ColorDarkCyan).
		SetAlign(tview.AlignLeft).
		SetSelectable(false)
	a.SetCell(0, 0, cell1)

	account := a.accountID
	if account == "" {
		account = "..."
	}
	line2 := account + " [gray](v" + a.version + ")[-]"
	cell2 := tview.NewTableCell(line2).
		SetTextColor(tcell.ColorWhite).
		SetAlign(tview.AlignLeft).
		SetSelectable(false)
	a.SetCell(1, 0, cell2)
}
