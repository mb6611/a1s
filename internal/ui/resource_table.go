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

// ResourceTable is a table view for displaying AWS resources.
type ResourceTable struct {
	*tview.Table

	resourceID  *dao.ResourceID
	actions     *KeyActions
	model       Tabular
	header      model1.Header
	sortColName string
	filterText  string
	fullData    *model1.TableData
	isUpdating  bool
	marks       map[string]struct{}
	mx          sync.RWMutex
}

// NewResourceTable creates a new resource table.
func NewResourceTable(rid *dao.ResourceID) *ResourceTable {
	r := &ResourceTable{
		Table:      tview.NewTable(),
		resourceID: rid,
		actions:    NewKeyActions(),
		marks:      make(map[string]struct{}),
	}

	// Style the table
	r.SetBorder(true)
	r.SetBorderAttributes(tcell.AttrBold)
	r.SetBorderPadding(0, 0, 1, 1)
	r.SetBorderColor(tcell.ColorWhite)
	r.SetBackgroundColor(tcell.ColorDefault)
	r.SetFixed(1, 0)
	r.SetSelectable(true, false)

	return r
}

// Init initializes the resource table.
func (r *ResourceTable) Init(ctx context.Context) error {
	r.Select(1, 0)

	// Set initial title
	if r.resourceID != nil {
		r.SetTitle(fmt.Sprintf(" %s(all)[0] ", r.resourceID.String()))
	}

	// Show initial loading message
	r.showNoData("Loading...")

	// Set up keyboard handler
	r.SetInputCapture(r.keyboard)

	r.bindKeys()
	return nil
}

// keyboard handles input.
func (r *ResourceTable) keyboard(evt *tcell.EventKey) *tcell.EventKey {
	key := evt.Key()
	row, col := r.GetSelection()
	rowCount := r.GetRowCount()

	// Handle vim-style navigation
	if key == tcell.KeyRune {
		switch evt.Rune() {
		case 'j': // Down
			if row < rowCount-1 {
				r.Select(row+1, col)
			}
			return nil
		case 'k': // Up
			if row > 1 {
				r.Select(row-1, col)
			}
			return nil
		case 'g': // Go to top
			if rowCount > 1 {
				r.Select(1, col)
			}
			return nil
		case 'G': // Go to bottom
			if rowCount > 1 {
				r.Select(rowCount-1, col)
			}
			return nil
		}
	}

	// Handle arrow keys
	switch key {
	case tcell.KeyDown:
		if row < rowCount-1 {
			r.Select(row+1, col)
		}
		return nil
	case tcell.KeyUp:
		if row > 1 {
			r.Select(row-1, col)
		}
		return nil
	case tcell.KeyHome:
		if rowCount > 1 {
			r.Select(1, col)
		}
		return nil
	case tcell.KeyEnd:
		if rowCount > 1 {
			r.Select(rowCount-1, col)
		}
		return nil
	}

	// Check registered action handlers
	actionKey := key
	if key == tcell.KeyRune {
		actionKey = tcell.Key(evt.Rune())
	}
	if action, ok := r.actions.Get(actionKey); ok {
		return action.Action(evt)
	}

	return evt
}

// bindKeys sets up key bindings.
func (r *ResourceTable) bindKeys() {
	r.actions.Bulk(KeyMap{
		tcell.KeyCtrlS: NewKeyAction("Sort", r.sortHandler, true),
		tcell.KeyEnter: NewKeyAction("Select", r.selectHandler, true),
	})
}

// sortHandler cycles through sort columns.
func (r *ResourceTable) sortHandler(evt *tcell.EventKey) *tcell.EventKey {
	r.mx.Lock()
	defer r.mx.Unlock()

	if r.header == nil || len(r.header) == 0 {
		return nil
	}

	currentIdx := -1
	for i, col := range r.header {
		if col.Name == r.sortColName {
			currentIdx = i
			break
		}
	}

	nextIdx := (currentIdx + 1) % len(r.header)
	r.sortColName = r.header[nextIdx].Name

	go r.refresh()
	return nil
}

// selectHandler handles row selection.
func (r *ResourceTable) selectHandler(evt *tcell.EventKey) *tcell.EventKey {
	return nil
}

// showNoData displays a message when there's no data.
func (r *ResourceTable) showNoData(msg string) {
	r.showMessage(msg, tcell.ColorGray)
}

// showError displays an error message in red.
func (r *ResourceTable) showError(msg string) {
	r.showMessage(msg, tcell.ColorRed)
}

// showMessage displays a centered message with the given color.
func (r *ResourceTable) showMessage(msg string, color tcell.Color) {
	r.Clear()
	cell := tview.NewTableCell(msg)
	cell.SetTextColor(color)
	cell.SetAlign(tview.AlignCenter)
	cell.SetSelectable(false)
	r.SetCell(0, 0, cell)
}

// SetModel sets the data model.
func (r *ResourceTable) SetModel(m Tabular) {
	r.mx.Lock()
	defer r.mx.Unlock()

	if r.model != nil {
		r.model.RemoveListener(r)
	}
	r.model = m
	if m != nil {
		m.AddListener(r)
	}
}

// GetModel returns the current model.
func (r *ResourceTable) GetModel() Tabular {
	r.mx.RLock()
	defer r.mx.RUnlock()
	return r.model
}

// ResourceID returns the resource identifier.
func (r *ResourceTable) ResourceID() *dao.ResourceID {
	return r.resourceID
}

// Actions returns key actions.
func (r *ResourceTable) Actions() *KeyActions {
	return r.actions
}

// Hints returns menu hints.
func (r *ResourceTable) Hints() MenuHints {
	return r.actions.Hints()
}

// GetSelectedItem returns the selected row's ID.
func (r *ResourceTable) GetSelectedItem() string {
	row, _ := r.GetSelection()
	if row == 0 {
		return ""
	}

	cell := r.GetCell(row, 0)
	if cell == nil {
		return ""
	}

	if ref := cell.GetReference(); ref != nil {
		if id, ok := ref.(string); ok {
			return id
		}
	}
	return cell.Text
}

// GetSelectedRowIndex returns the current selection index.
func (r *ResourceTable) GetSelectedRowIndex() int {
	row, _ := r.GetSelection()
	return row
}

// SetFilter sets the current filter text.
func (r *ResourceTable) SetFilter(filter string) {
	r.mx.Lock()
	r.filterText = filter
	r.mx.Unlock()
	r.applyFilter()
}

// ClearFilter clears the filter.
func (r *ResourceTable) ClearFilter() {
	r.SetFilter("")
}

// applyFilter filters data based on current filter text.
func (r *ResourceTable) applyFilter() {
	r.mx.RLock()
	data := r.fullData
	filter := strings.ToLower(r.filterText)
	r.mx.RUnlock()

	if data == nil {
		return
	}

	if filter == "" {
		r.renderData(data)
		return
	}

	// Filter rows
	filtered := model1.NewTableData()
	filtered.SetHeader(data.Header())
	filtered.SetNamespace(data.Namespace())

	rowEvents := data.RowEvents()
	if rowEvents != nil {
		rowEvents.Range(func(idx int, re model1.RowEvent) bool {
			for _, field := range re.Row.Fields {
				if strings.Contains(strings.ToLower(field), filter) {
					filtered.RowEvents().Add(re)
					break
				}
			}
			return true
		})
	}

	r.renderData(filtered)
}

// renderData renders the given data to the table.
func (r *ResourceTable) renderData(data *model1.TableData) {
	if data == nil || data.Empty() {
		r.showNoData("No matching resources")
		r.updateTitle()
		return
	}

	r.Clear()

	header := data.Header()
	r.buildHeader(header)

	rowEvents := data.RowEvents()
	if rowEvents != nil {
		rowEvents.Range(func(idx int, re model1.RowEvent) bool {
			r.buildRow(re.Row, header, idx+1)
			return true
		})
	}

	r.updateTitle()

	if r.GetRowCount() > 1 {
		r.Select(1, 0)
	}
}

// buildHeader builds the header row.
func (r *ResourceTable) buildHeader(header model1.Header) {
	r.mx.Lock()
	r.header = header
	r.mx.Unlock()

	for col, h := range header {
		cell := tview.NewTableCell(h.Name)
		cell.SetTextColor(tcell.ColorYellow)
		cell.SetBackgroundColor(tcell.ColorDefault)
		cell.SetAlign(h.Align)
		cell.SetExpansion(1)
		cell.SetSelectable(false)

		if h.Name == r.sortColName {
			cell.SetText(h.Name + " ▼")
			cell.SetAttributes(tcell.AttrBold)
		}

		r.SetCell(0, col, cell)
	}
}

// buildRow builds a data row.
func (r *ResourceTable) buildRow(row model1.Row, header model1.Header, rowIdx int) {
	for col, field := range row.Fields {
		if col >= len(header) {
			break
		}

		cell := tview.NewTableCell(field)
		cell.SetBackgroundColor(tcell.ColorDefault)
		cell.SetAlign(header[col].Align)
		cell.SetExpansion(1)

		// Apply color based on column name and value
		color := r.cellColor(header[col].Name, field)
		cell.SetTextColor(color)

		if col == 0 {
			cell.SetReference(row.ID)
		}

		r.SetCell(rowIdx, col, cell)
	}
}

// cellColor returns the appropriate color for a cell based on column and value.
func (r *ResourceTable) cellColor(colName, value string) tcell.Color {
	colUpper := strings.ToUpper(colName)
	valLower := strings.ToLower(value)

	// Status/State columns
	if colUpper == "STATE" || colUpper == "STATUS" {
		switch valLower {
		case "running", "active", "available", "attached", "enabled", "in-use", "completed":
			return tcell.ColorGreen
		case "stopped", "terminated", "failed", "error", "deleted", "detached":
			return tcell.ColorRed
		case "pending", "starting", "stopping", "updating", "creating", "deleting", "modifying":
			return tcell.ColorYellow
		case "shutting-down":
			return tcell.ColorOrange
		}
	}

	// Name column - slightly brighter
	if colUpper == "NAME" {
		if value != "" && value != "-" {
			return tcell.ColorAqua
		}
	}

	// ID column
	if colUpper == "ID" {
		return tcell.ColorSteelBlue
	}

	// Default
	return tcell.ColorWhite
}

// updateTitle updates the border title.
func (r *ResourceTable) updateTitle() {
	region := "all"
	if r.model != nil {
		ns := r.model.GetNamespace()
		if ns != "" && ns != "*" {
			region = ns
		}
	} else if r.fullData != nil {
		ns := r.fullData.Namespace()
		if ns != "" && ns != "*" {
			region = ns
		}
	}

	count := "0"
	if r.GetRowCount() > 1 {
		count = fmt.Sprintf("%d", r.GetRowCount()-1)
	}

	resource := r.resourceID.String()
	title := fmt.Sprintf(" %s(%s)[%s] ", resource, region, count)
	r.SetTitle(title)
}

// UpdateUI updates the table from TableData.
func (r *ResourceTable) UpdateUI(data *model1.TableData) {
	r.mx.Lock()
	if r.isUpdating {
		r.mx.Unlock()
		return
	}
	r.isUpdating = true
	r.fullData = data
	filter := r.filterText
	r.mx.Unlock()

	defer func() {
		r.mx.Lock()
		r.isUpdating = false
		r.mx.Unlock()
	}()

	// Check for error message first
	if data != nil && data.HasError() {
		r.showError(data.Error())
		r.updateTitle()
		return
	}

	if data == nil || data.Empty() {
		r.showNoData("No resources found")
		r.updateTitle()
		return
	}

	if filter != "" {
		r.applyFilter()
		return
	}

	r.renderData(data)
}

// refresh triggers a table refresh.
func (r *ResourceTable) refresh() {
	if r.model != nil {
		data := r.model.Peek()
		r.UpdateUI(data)
	}
}

// TableDataChanged implements TableListener.
func (r *ResourceTable) TableDataChanged(data *model1.TableData) {
	r.UpdateUI(data)
}

// TableLoadFailed implements TableListener.
func (r *ResourceTable) TableLoadFailed(err error) {
	r.Clear()
	title := fmt.Sprintf(" [Error] %s: %v ", r.resourceID.String(), err)
	r.SetTitle(title)
}

// TableNoData implements TableListener.
func (r *ResourceTable) TableNoData(data *model1.TableData) {
	r.showNoData("No resources found")
	r.updateTitle()
}

// ClearMarks clears all marks.
func (r *ResourceTable) ClearMarks() {
	r.mx.Lock()
	defer r.mx.Unlock()
	r.marks = make(map[string]struct{})
}

// ToggleMark toggles mark on current selection.
func (r *ResourceTable) ToggleMark() {
	item := r.GetSelectedItem()
	if item == "" {
		return
	}

	r.mx.Lock()
	defer r.mx.Unlock()

	if _, ok := r.marks[item]; ok {
		delete(r.marks, item)
	} else {
		r.marks[item] = struct{}{}
	}
}

// IsMarked checks if an item is marked.
func (r *ResourceTable) IsMarked(item string) bool {
	r.mx.RLock()
	defer r.mx.RUnlock()
	_, ok := r.marks[item]
	return ok
}

// GetMarked returns all marked items.
func (r *ResourceTable) GetMarked() []string {
	r.mx.RLock()
	defer r.mx.RUnlock()

	marked := make([]string, 0, len(r.marks))
	for k := range r.marks {
		marked = append(marked, k)
	}
	return marked
}
