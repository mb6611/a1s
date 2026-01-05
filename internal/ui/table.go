// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

const (
	// RegionTitleFmt formats the table title with resource type, region, and count.
	RegionTitleFmt = " <%s>[%s][%s] "
)

// Table represents a table view for AWS resources.
type Table struct {
	*SelectTable

	resourceID   *dao.ResourceID
	actions      *KeyActions
	model        Tabular
	header       model1.Header
	sortCol      model1.HeaderColumn
	sortColName  string
	filterText   string
	filterActive bool
	fullData     *model1.TableData // Store full data for filtering
	isUpdating   bool
	mx           sync.RWMutex
}

// NewTable returns a new table instance.
func NewTable(rid *dao.ResourceID) *Table {
	return &Table{
		SelectTable: &SelectTable{
			Table: tview.NewTable(),
			marks: make(map[string]struct{}),
		},
		resourceID: rid,
		actions:    NewKeyActions(),
	}
}

// Init initializes the table component.
func (t *Table) Init(ctx context.Context) error {
	t.SetFixed(1, 0)
	t.SetBorder(true)
	t.SetBorderAttributes(tcell.AttrBold)
	t.SetBorderPadding(0, 0, 1, 1)
	t.SetSelectable(true, false)
	t.SetBackgroundColor(tcell.ColorDefault)
	t.SetBorderColor(tcell.ColorWhite)
	t.Select(1, 0)

	// Set initial title
	if t.resourceID != nil {
		t.SetTitle(fmt.Sprintf(" <%s>[all][0] ", t.resourceID.String()))
	}

	// Show initial "loading" message
	t.showNoData("Loading...")

	// Set up keyboard handler for vim-style navigation
	t.SetInputCapture(t.keyboard)

	t.bindKeys()
	return nil
}

// keyboard handles table keyboard input.
func (t *Table) keyboard(evt *tcell.EventKey) *tcell.EventKey {
	key := evt.Key()

	// Handle filter input mode
	t.mx.RLock()
	filterActive := t.filterActive
	t.mx.RUnlock()

	if filterActive {
		return t.handleFilterInput(evt)
	}

	row, col := t.GetSelection()
	rowCount := t.GetRowCount()

	// Handle vim-style navigation
	if key == tcell.KeyRune {
		switch evt.Rune() {
		case 'j': // Down
			if row < rowCount-1 {
				t.Select(row+1, col)
			}
			return nil
		case 'k': // Up
			if row > 1 { // Skip header row
				t.Select(row-1, col)
			}
			return nil
		case 'g': // Go to top
			if rowCount > 1 {
				t.Select(1, col)
			}
			return nil
		case 'G': // Go to bottom
			if rowCount > 1 {
				t.Select(rowCount-1, col)
			}
			return nil
		}
	}

	// Handle arrow keys
	switch key {
	case tcell.KeyDown:
		if row < rowCount-1 {
			t.Select(row+1, col)
		}
		return nil
	case tcell.KeyUp:
		if row > 1 { // Skip header row
			t.Select(row-1, col)
		}
		return nil
	case tcell.KeyHome:
		if rowCount > 1 {
			t.Select(1, col)
		}
		return nil
	case tcell.KeyEnd:
		if rowCount > 1 {
			t.Select(rowCount-1, col)
		}
		return nil
	}

	// Check for registered action handlers
	actionKey := key
	if key == tcell.KeyRune {
		actionKey = tcell.Key(evt.Rune())
	}
	if action, ok := t.actions.Get(actionKey); ok {
		return action.Action(evt)
	}

	return evt
}

// handleFilterInput handles keyboard input when filter mode is active.
func (t *Table) handleFilterInput(evt *tcell.EventKey) *tcell.EventKey {
	key := evt.Key()

	switch key {
	case tcell.KeyEsc:
		// Cancel filter
		t.mx.Lock()
		t.filterActive = false
		t.filterText = ""
		t.mx.Unlock()
		t.applyFilter()
		return nil

	case tcell.KeyEnter:
		// Confirm filter
		t.mx.Lock()
		t.filterActive = false
		t.mx.Unlock()
		t.updateTitleWithFilter()
		return nil

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		// Delete character
		t.mx.Lock()
		if len(t.filterText) > 0 {
			t.filterText = t.filterText[:len(t.filterText)-1]
		}
		t.mx.Unlock()
		t.applyFilter()
		return nil

	case tcell.KeyRune:
		// Add character to filter
		t.mx.Lock()
		t.filterText += string(evt.Rune())
		t.mx.Unlock()
		t.applyFilter()
		return nil
	}

	return evt
}

// showNoData displays a message when there's no data.
func (t *Table) showNoData(msg string) {
	t.Clear()
	cell := tview.NewTableCell(msg)
	cell.SetTextColor(tcell.ColorGray)
	cell.SetAlign(tview.AlignCenter)
	cell.SetSelectable(false)
	t.SetCell(0, 0, cell)
}

// SetModel sets the table data model.
func (t *Table) SetModel(m Tabular) {
	t.mx.Lock()
	defer t.mx.Unlock()

	if t.model != nil {
		t.model.RemoveListener(t)
	}
	t.model = m
	t.SelectTable.SetModel(m)
	if m != nil {
		m.AddListener(t)
	}
}

// GetModel returns the current table model.
func (t *Table) GetModel() Tabular {
	t.mx.RLock()
	defer t.mx.RUnlock()

	return t.model
}

// ResourceID returns the resource identifier.
func (t *Table) ResourceID() *dao.ResourceID {
	return t.resourceID
}

// Actions returns the key actions.
func (t *Table) Actions() *KeyActions {
	return t.actions
}

// Hints returns menu hints for key bindings.
func (t *Table) Hints() MenuHints {
	return t.actions.Hints()
}

// bindKeys sets up common table key bindings.
func (t *Table) bindKeys() {
	t.actions.Bulk(KeyMap{
		tcell.KeyCtrlS: NewKeyAction("Sort", t.sortHandler, true),
		tcell.KeyEnter: NewKeyAction("Select", t.selectHandler, true),
		KeySlash:       NewKeyAction("Filter", t.filterHandler, true),
		tcell.KeyEsc:   NewKeyAction("Clear Filter", t.clearFilterHandler, false),
	})
}

// sortHandler handles sort key events.
func (t *Table) sortHandler(evt *tcell.EventKey) *tcell.EventKey {
	t.mx.Lock()
	defer t.mx.Unlock()

	if t.header == nil || len(t.header) == 0 {
		return nil
	}

	// Cycle through columns for sorting
	currentIdx := -1
	for i, col := range t.header {
		if col.Name == t.sortColName {
			currentIdx = i
			break
		}
	}

	nextIdx := (currentIdx + 1) % len(t.header)
	t.sortColName = t.header[nextIdx].Name
	t.sortCol = t.header[nextIdx]

	// Trigger a refresh
	go t.refresh()

	return nil
}

// selectHandler handles row selection.
func (t *Table) selectHandler(evt *tcell.EventKey) *tcell.EventKey {
	// Override in specific table implementations
	return nil
}

// filterHandler handles filter activation.
func (t *Table) filterHandler(evt *tcell.EventKey) *tcell.EventKey {
	t.mx.Lock()
	t.filterActive = true
	t.filterText = ""
	t.mx.Unlock()
	t.updateTitleWithFilter()
	return nil
}

// clearFilterHandler clears active filters.
func (t *Table) clearFilterHandler(evt *tcell.EventKey) *tcell.EventKey {
	t.mx.Lock()
	wasActive := t.filterActive || t.filterText != ""
	t.filterActive = false
	t.filterText = ""
	t.mx.Unlock()

	if wasActive {
		// Re-render with no filter
		t.applyFilter()
	}
	return nil
}

// applyFilter filters the table based on filterText.
func (t *Table) applyFilter() {
	t.mx.RLock()
	data := t.fullData
	filter := strings.ToLower(t.filterText)
	t.mx.RUnlock()

	if data == nil {
		return
	}

	if filter == "" {
		// No filter - show all data
		t.renderData(data)
		return
	}

	// Filter rows
	filtered := model1.NewTableData()
	filtered.SetHeader(data.Header())
	filtered.SetNamespace(data.Namespace())

	rowEvents := data.RowEvents()
	if rowEvents != nil {
		rowEvents.Range(func(idx int, re model1.RowEvent) bool {
			// Check if any field contains the filter text
			for _, field := range re.Row.Fields {
				if strings.Contains(strings.ToLower(field), filter) {
					filtered.RowEvents().Add(re)
					break
				}
			}
			return true
		})
	}

	t.renderData(filtered)
}

// renderData renders the given data to the table.
func (t *Table) renderData(data *model1.TableData) {
	if data == nil || data.Empty() {
		t.showNoData("No matching resources")
		t.updateTitleWithFilter()
		return
	}

	t.Clear()

	header := data.Header()
	t.buildHeader(header)

	rowEvents := data.RowEvents()
	if rowEvents != nil {
		rowEvents.Range(func(idx int, re model1.RowEvent) bool {
			t.buildRow(re.Row, header, idx+1)
			return true
		})
	}

	t.updateTitleWithFilter()

	if t.GetRowCount() > 1 {
		t.Select(1, 0)
	}
}

// updateTitleWithFilter updates title including filter text.
func (t *Table) updateTitleWithFilter() {
	t.mx.RLock()
	filter := t.filterText
	filterActive := t.filterActive
	data := t.fullData
	t.mx.RUnlock()

	region := "all"
	if t.model != nil {
		ns := t.model.GetNamespace()
		if ns != "" && ns != "*" {
			region = ns
		}
	} else if data != nil {
		ns := data.Namespace()
		if ns != "" && ns != "*" {
			region = ns
		}
	}

	count := "0"
	if data != nil {
		count = fmt.Sprintf("%d", t.GetRowCount()-1) // Minus header
	}

	resource := t.resourceID.String()
	title := fmt.Sprintf(RegionTitleFmt, resource, region, count)

	if filterActive || filter != "" {
		title = fmt.Sprintf(" <%s>[%s][%s] Filter: %s█ ", resource, region, count, filter)
	}

	t.SetTitle(title)
}

// UpdateUI updates the table display from TableData.
func (t *Table) UpdateUI(data *model1.TableData) {
	t.mx.Lock()
	if t.isUpdating {
		t.mx.Unlock()
		return
	}
	t.isUpdating = true
	// Store full data for filtering
	t.fullData = data
	filter := t.filterText
	t.mx.Unlock()

	defer func() {
		t.mx.Lock()
		t.isUpdating = false
		t.mx.Unlock()
	}()

	if data == nil || data.Empty() {
		t.showNoData("No resources found")
		t.updateTitleWithFilter()
		return
	}

	// Apply filter if active
	if filter != "" {
		t.applyFilter()
		return
	}

	t.renderData(data)
}

// buildHeader builds the table header row.
func (t *Table) buildHeader(header model1.Header) {
	t.mx.Lock()
	t.header = header
	t.mx.Unlock()

	for col, h := range header {
		cell := tview.NewTableCell(h.Name)
		cell.SetTextColor(tcell.ColorYellow)
		cell.SetBackgroundColor(tcell.ColorDefault)
		cell.SetAlign(h.Align)
		cell.SetExpansion(1)
		cell.SetSelectable(false)

		// Mark sorted column
		if h.Name == t.sortColName {
			cell.SetText(h.Name + " ▼")
			cell.SetAttributes(tcell.AttrBold)
		}

		t.SetCell(0, col, cell)
	}
}

// buildRow builds a single data row.
func (t *Table) buildRow(row model1.Row, header model1.Header, rowIdx int) {
	for col, field := range row.Fields {
		if col >= len(header) {
			break
		}

		cell := tview.NewTableCell(field)
		cell.SetTextColor(tcell.ColorWhite)
		cell.SetBackgroundColor(tcell.ColorDefault)
		cell.SetAlign(header[col].Align)
		cell.SetExpansion(1)

		// Store row ID in first column
		if col == 0 {
			cell.SetReference(row.ID)
		}

		t.SetCell(rowIdx, col, cell)
	}
}

// updateTitle updates the table title with resource info.
func (t *Table) updateTitle(data *model1.TableData) {
	region := "all"
	if t.model != nil {
		ns := t.model.GetNamespace()
		if ns != "" && ns != "*" {
			region = ns
		}
	}

	count := "0"
	if data != nil {
		count = fmt.Sprintf("%d", data.RowCount())
	}

	resource := t.resourceID.String()
	title := fmt.Sprintf(RegionTitleFmt, resource, region, count)
	t.SetTitle(title)
}

// refresh triggers a table refresh.
func (t *Table) refresh() {
	if t.model != nil {
		data := t.model.Peek()
		t.UpdateUI(data)
	}
}

// TableDataChanged implements model.TableListener.
func (t *Table) TableDataChanged(data *model1.TableData) {
	t.UpdateUI(data)
}

// TableLoadFailed implements model.TableListener.
func (t *Table) TableLoadFailed(err error) {
	t.Clear()
	// Show error in title or status
	title := fmt.Sprintf(" [Error] %s: %v ", t.resourceID.String(), err)
	t.SetTitle(title)
}

// TableNoData implements model.TableListener.
func (t *Table) TableNoData(data *model1.TableData) {
	t.showNoData("No resources found")
	t.updateTitle(data)
}
