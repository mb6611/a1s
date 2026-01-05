package model1

import (
	"fmt"
	"reflect"
)

const ageCol = "AGE"

// Attrs represents column attributes
type Attrs struct {
	Align     int           // tview alignment
	Wide      bool          // Hidden in narrow view
	Time      bool          // Age column
	Capacity  bool          // Numeric (right-align)
	Hide      bool          // Always hidden
	Decorator DecoratorFunc
}

func (a Attrs) Merge(b Attrs) Attrs {
	if a.Align == 0 {
		a.Align = b.Align
	}
	if !a.Hide {
		a.Hide = b.Hide
	}
	if !a.Wide {
		a.Wide = b.Wide
	}
	if !a.Time {
		a.Time = b.Time
	}
	if !a.Capacity {
		a.Capacity = b.Capacity
	}
	if a.Decorator == nil {
		a.Decorator = b.Decorator
	}
	return a
}

// HeaderColumn represents a table header column
type HeaderColumn struct {
	Name string
	Attrs
}

func (h HeaderColumn) String() string {
	return fmt.Sprintf("%s [%d::%t::%t]", h.Name, h.Align, h.Wide, h.Time)
}

func (h HeaderColumn) Clone() HeaderColumn {
	return h
}

// Header represents a table header (slice of columns)
type Header []HeaderColumn

func (h Header) Clone() Header {
	he := make(Header, 0, len(h))
	for _, c := range h {
		he = append(he, c.Clone())
	}
	return he
}

func (h Header) Diff(header Header) bool {
	if len(h) != len(header) {
		return true
	}
	return !reflect.DeepEqual(h, header)
}

func (h Header) IndexOf(colName string, includeWide bool) (int, bool) {
	for i, c := range h {
		if c.Wide && !includeWide {
			continue
		}
		if c.Name == colName {
			return i, true
		}
	}
	return -1, false
}

func (h Header) HasAge() bool {
	_, ok := h.IndexOf(ageCol, true)
	return ok
}

func (h Header) IsTimeCol(col int) bool {
	if col < 0 || col >= len(h) {
		return false
	}
	return h[col].Time
}

func (h Header) IsCapacityCol(col int) bool {
	if col < 0 || col >= len(h) {
		return false
	}
	return h[col].Capacity
}

func (h Header) ColumnNames(wide bool) []string {
	if len(h) == 0 {
		return nil
	}
	cc := make([]string, 0, len(h))
	for _, c := range h {
		if !wide && c.Wide {
			continue
		}
		cc = append(cc, c.Name)
	}
	return cc
}
