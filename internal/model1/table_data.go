package model1

import "sync"

// TableData tracks resource data for tabular display.
type TableData struct {
	header    Header
	rowEvents *RowEvents
	namespace string
	errMsg    string
	mx        sync.RWMutex
}

// NewTableData returns a new table.
func NewTableData() *TableData {
	return &TableData{
		rowEvents: NewRowEvents(10),
	}
}

// Header returns the table header.
func (t *TableData) Header() Header {
	t.mx.RLock()
	defer t.mx.RUnlock()
	return t.header
}

// SetHeader sets the table header.
func (t *TableData) SetHeader(h Header) {
	t.mx.Lock()
	defer t.mx.Unlock()
	t.header = h
}

// RowEvents returns the row events.
func (t *TableData) RowEvents() *RowEvents {
	t.mx.RLock()
	defer t.mx.RUnlock()
	return t.rowEvents
}

// Namespace returns the table namespace.
func (t *TableData) Namespace() string {
	t.mx.RLock()
	defer t.mx.RUnlock()
	return t.namespace
}

// SetNamespace sets the table namespace.
func (t *TableData) SetNamespace(ns string) {
	t.mx.Lock()
	defer t.mx.Unlock()
	t.namespace = ns
}

// Empty returns true if no data is available.
func (t *TableData) Empty() bool {
	t.mx.RLock()
	defer t.mx.RUnlock()
	return t.rowEvents.Empty()
}

// RowCount returns the number of rows.
func (t *TableData) RowCount() int {
	t.mx.RLock()
	defer t.mx.RUnlock()
	return t.rowEvents.Count()
}

// Clone returns a shallow copy of the table data.
func (t *TableData) Clone() *TableData {
	t.mx.RLock()
	defer t.mx.RUnlock()

	return &TableData{
		header:    t.header,
		rowEvents: t.rowEvents,
		namespace: t.namespace,
		errMsg:    t.errMsg,
	}
}

// SetError sets an error message to display instead of data.
func (t *TableData) SetError(msg string) {
	t.mx.Lock()
	defer t.mx.Unlock()
	t.errMsg = msg
}

// Error returns the error message, if any.
func (t *TableData) Error() string {
	t.mx.RLock()
	defer t.mx.RUnlock()
	return t.errMsg
}

// HasError returns true if there's an error message.
func (t *TableData) HasError() bool {
	t.mx.RLock()
	defer t.mx.RUnlock()
	return t.errMsg != ""
}
