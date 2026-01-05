package model

import (
	"context"

	"github.com/a1s/a1s/internal/model1"
)

// TableModel defines the interface for a table data model that fetches data.
type TableModel interface {
	// Header returns the table header.
	Header() model1.Header

	// RowCount returns the number of rows.
	RowCount() int

	// RowEvents returns the current row events.
	RowEvents() *model1.RowEvents

	// Watch starts watching/refreshing data periodically.
	Watch(context.Context) error

	// Refresh fetches data from the source immediately.
	Refresh(context.Context) error

	// AddListener registers a table listener.
	AddListener(TableListener)

	// RemoveListener unregisters a table listener.
	RemoveListener(TableListener)
}
