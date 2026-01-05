// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package ui

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

const (
	menuIndexFmt = " [yellow::b]<%d>[white::-] %s "
	menuPlainFmt = " [yellow::b]<%s>[white::-] %s "
	maxRows      = 6
)

// Menu presents menu options.
type Menu struct {
	*tview.Table
}

// NewMenu returns a new menu.
func NewMenu() *Menu {
	m := &Menu{
		Table: tview.NewTable(),
	}
	m.SetBackgroundColor(tcell.ColorDefault)
	m.SetBorderPadding(0, 0, 1, 1)

	return m
}

// HydrateMenu populate menu ui from hints.
func (m *Menu) HydrateMenu(hh MenuHints) {
	m.Clear()
	sort.Sort(hh)

	table := make([]MenuHints, maxRows+1)
	colCount := (len(hh) / maxRows) + 1
	for row := range maxRows {
		table[row] = make(MenuHints, colCount)
	}
	out := m.buildMenuTable(hh, table, colCount)

	for row := range out {
		for col := range len(out[row]) {
			c := tview.NewTableCell(out[row][col])
			if out[row][col] == "" {
				c = tview.NewTableCell("")
			}
			c.SetBackgroundColor(tcell.ColorDefault)
			m.SetCell(row, col, c)
		}
	}
}

func (m *Menu) buildMenuTable(hh MenuHints, table []MenuHints, colCount int) [][]string {
	var row, col int
	maxKeys := make([]int, colCount)

	for _, h := range hh {
		if !h.Visible {
			continue
		}

		if maxKeys[col] < len(h.Mnemonic) {
			maxKeys[col] = len(h.Mnemonic)
		}
		table[row][col] = h
		row++
		if row >= maxRows {
			row, col = 0, col+1
		}
	}

	out := make([][]string, len(table))
	for r := range out {
		out[r] = make([]string, len(table[r]))
	}
	m.layout(table, maxKeys, out)

	return out
}

func (m *Menu) layout(table []MenuHints, mm []int, out [][]string) {
	for r := range table {
		for c := range table[r] {
			out[r][c] = m.formatMenu(table[r][c])
		}
	}
}

func (m *Menu) formatMenu(h MenuHint) string {
	if h.Mnemonic == "" || h.Description == "" {
		return ""
	}

	i, err := strconv.Atoi(h.Mnemonic)
	if err == nil {
		return fmt.Sprintf(menuIndexFmt, i, h.Description)
	}

	return fmt.Sprintf(menuPlainFmt, h.Mnemonic, h.Description)
}

// StackPushed notifies a component was added.
func (m *Menu) StackPushed(c Component) {
	if h, ok := c.(Hinter); ok {
		m.HydrateMenu(h.Hints())
	}
}

// StackPopped notifies a component was removed.
func (m *Menu) StackPopped(_, top Component) {
	if top != nil {
		if h, ok := top.(Hinter); ok {
			m.HydrateMenu(h.Hints())
		}
	} else {
		m.Clear()
	}
}

// StackTop notifies the top component.
func (m *Menu) StackTop(t Component) {
	if h, ok := t.(Hinter); ok {
		m.HydrateMenu(h.Hints())
	}
}
