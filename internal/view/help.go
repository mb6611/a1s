// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package view

import (
	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

// HelpBind represents a single keybinding.
type HelpBind struct {
	Key  string
	Desc string
}

// Help displays a full-screen help view with keybindings (k9s style).
type Help struct {
	*tview.Table
	closeFn func()
}

// NewHelp creates a new help view.
func NewHelp() *Help {
	h := &Help{
		Table: tview.NewTable(),
	}
	h.build()
	return h
}

// SetCloseFn sets the callback when help is closed.
func (h *Help) SetCloseFn(fn func()) {
	h.closeFn = fn
}

// build constructs the help UI.
func (h *Help) build() {
	h.SetBorder(true)
	h.SetTitle(" Help ")
	h.SetTitleAlign(tview.AlignCenter)
	h.SetBorderColor(tcell.ColorYellow)
	h.SetBackgroundColor(tcell.ColorDefault)
	h.SetSelectable(false, false)

	h.populateHelp()

	h.SetInputCapture(func(evt *tcell.EventKey) *tcell.EventKey {
		switch evt.Key() {
		case tcell.KeyEsc, tcell.KeyEnter:
			if h.closeFn != nil {
				h.closeFn()
			}
			return nil
		}
		if evt.Rune() == '?' || evt.Rune() == 'q' {
			if h.closeFn != nil {
				h.closeFn()
			}
			return nil
		}
		return evt
	})
}

// populateHelp fills the help table with keybindings in k9s-style 4-column layout.
func (h *Help) populateHelp() {
	// Column 1: Resources
	col1 := []HelpBind{
		{":ec2", "EC2"},
		{":s3", "S3"},
		{":sg", "SecGroups"},
		{":vpc", "VPCs"},
		{":subnet", "Subnets"},
		{":iam", "Users"},
		{":role", "Roles"},
		{":policy", "Policies"},
		{":eks", "EKS"},
		{":vol", "Volumes"},
	}

	// Column 2: General
	col2 := []HelpBind{
		{"<:>", "Command"},
		{"</>", "Filter"},
		{"<?>", "Help"},
		{"<esc>", "Back"},
		{"<q>", "Quit"},
		{"<r>", "Refresh"},
	}

	// Column 3: Navigation
	col3 := []HelpBind{
		{"<j>", "Down"},
		{"<k>", "Up"},
		{"<g>", "Top"},
		{"<G>", "Bottom"},
		{"<enter>", "Select"},
		{"<d>", "Describe"},
		{"<e>", "Edit"},
		{"<y>", "YAML"},
	}

	// Column 4: Actions
	col4 := []HelpBind{
		{"<s>", "Stop"},
		{"<C-s>", "Start"},
		{"<C-r>", "Reboot"},
		{"<c>", "Connect"},
		{"<S>", "SSM"},
		{"<C-d>", "Delete"},
		{"<bksp>", "Back"},
	}

	columns := [][]HelpBind{col1, col2, col3, col4}
	headers := []string{"RESOURCES", "GENERAL", "NAVIGATION", "ACTIONS"}

	// Find max rows
	maxRows := 0
	for _, col := range columns {
		if len(col) > maxRows {
			maxRows = len(col)
		}
	}

	// Each logical column = 2 table columns (key + desc) + 1 spacer
	// colWidth = 3 (key, desc, spacer)
	colWidth := 3
	for colIdx, col := range columns {
		baseCol := colIdx * colWidth

		// Header
		header := tview.NewTableCell(headers[colIdx]).
			SetTextColor(tcell.ColorAqua).
			SetAttributes(tcell.AttrBold).
			SetSelectable(false)
		h.SetCell(0, baseCol, header)

		// Bindings
		for rowIdx, bind := range col {
			row := rowIdx + 1

			// Key cell (yellow)
			keyCell := tview.NewTableCell(bind.Key).
				SetTextColor(tcell.ColorYellow).
				SetSelectable(false)
			h.SetCell(row, baseCol, keyCell)

			// Desc cell (white) with expansion to fill space
			descCell := tview.NewTableCell(bind.Desc).
				SetTextColor(tcell.ColorWhite).
				SetSelectable(false).
				SetExpansion(1)
			h.SetCell(row, baseCol+1, descCell)
		}

		// Spacer column between logical columns (except last)
		if colIdx < len(columns)-1 {
			for row := 0; row <= maxRows; row++ {
				spacer := tview.NewTableCell("").
					SetSelectable(false).
					SetExpansion(1)
				h.SetCell(row, baseCol+2, spacer)
			}
		}
	}

	// Footer
	footer := tview.NewTableCell("<esc> to close").
		SetTextColor(tcell.ColorGray).
		SetSelectable(false)
	h.SetCell(maxRows+2, 0, footer)
}
