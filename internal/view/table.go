// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package view

import (
	"context"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/ui"
	"github.com/derailed/tcell/v2"
)

// EnvFunc returns environment context.
type EnvFunc func() Env

// Env provides environment/context information.
type Env interface {
	GetProfile() string
	GetRegion() string
}

// Table wraps ui.ResourceTable with view-layer functionality.
type Table struct {
	*ui.ResourceTable

	rid     *dao.ResourceID
	envFn   EnvFunc
	enterFn func(*tcell.EventKey) *tcell.EventKey
}

// NewTable creates a new table view.
func NewTable(rid *dao.ResourceID) *Table {
	return &Table{
		ResourceTable: ui.NewResourceTable(rid),
		rid:           rid,
	}
}

// Init initializes the table view.
func (t *Table) Init(ctx context.Context) error {
	if t.ResourceTable == nil {
		t.ResourceTable = ui.NewResourceTable(t.rid)
	}

	if err := t.ResourceTable.Init(ctx); err != nil {
		return err
	}

	t.bindKeys(t.Actions())
	return nil
}

// Start begins the table lifecycle.
func (t *Table) Start() {
	// Lifecycle hook - can be extended
}

// Stop ends the table lifecycle.
func (t *Table) Stop() {
	// Lifecycle hook - can be extended
}

// SetEnvFn sets the environment function.
func (t *Table) SetEnvFn(fn EnvFunc) {
	t.envFn = fn
}

// SetEnterFn sets the enter key handler.
func (t *Table) SetEnterFn(fn func(*tcell.EventKey) *tcell.EventKey) {
	t.enterFn = fn
}

// SetFilter sets the table filter.
func (t *Table) SetFilter(filter string) {
	if t.ResourceTable != nil {
		t.ResourceTable.SetFilter(filter)
	}
}

// ClearFilter clears the table filter.
func (t *Table) ClearFilter() {
	if t.ResourceTable != nil {
		t.ResourceTable.ClearFilter()
	}
}

// Name returns the resource ID as a string.
func (t *Table) Name() string {
	if t.rid == nil {
		return ""
	}
	return t.rid.String()
}

// GetResourceID returns the resource identifier.
func (t *Table) GetResourceID() *dao.ResourceID {
	return t.rid
}

// bindKeys adds view-specific key bindings.
func (t *Table) bindKeys(aa *ui.KeyActions) {
	if aa == nil {
		return
	}

	aa.Bulk(ui.KeyMap{
		tcell.KeyEnter: ui.NewKeyAction("Describe", t.enterCmd, true),
		ui.KeyY:        ui.NewKeyAction("Copy ARN", t.cpyCmd, true),
		tcell.KeyCtrlR: ui.NewKeyAction("Refresh", t.refreshCmd, true),
	})
}

// enterCmd handles the enter key event.
func (t *Table) enterCmd(evt *tcell.EventKey) *tcell.EventKey {
	if t.enterFn != nil {
		return t.enterFn(evt)
	}
	return nil
}

// cpyCmd copies the ARN to clipboard.
func (t *Table) cpyCmd(evt *tcell.EventKey) *tcell.EventKey {
	// Placeholder - clipboard integration to be implemented
	return nil
}

// refreshCmd refreshes the table data.
func (t *Table) refreshCmd(evt *tcell.EventKey) *tcell.EventKey {
	// Placeholder - refresh logic to be implemented
	return nil
}
