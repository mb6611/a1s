package model1

import "reflect"

// DeltaRow represents a collection of row deltas between old and new row
type DeltaRow []string

func NewDeltaRow(o, n Row, h Header) DeltaRow {
	deltas := make(DeltaRow, len(o.Fields))
	for i, old := range o.Fields {
		if i >= len(n.Fields) {
			continue
		}
		if old != "" && old != n.Fields[i] && !h.IsTimeCol(i) {
			deltas[i] = old
		}
	}
	return deltas
}

func (d DeltaRow) Diff(r DeltaRow, ageCol int) bool {
	if len(d) != len(r) {
		return true
	}
	if ageCol < 0 || ageCol >= len(d) {
		return !reflect.DeepEqual(d, r)
	}
	if !reflect.DeepEqual(d[:ageCol], r[:ageCol]) {
		return true
	}
	if ageCol+1 >= len(d) {
		return false
	}
	return !reflect.DeepEqual(d[ageCol+1:], r[ageCol+1:])
}

func (d DeltaRow) Customize(cols []int, out DeltaRow) {
	if d.IsBlank() {
		return
	}
	for i, c := range cols {
		if c < 0 {
			continue
		}
		if c < len(d) && i < len(out) {
			out[i] = d[c]
		}
	}
}

func (d DeltaRow) IsBlank() bool {
	if len(d) == 0 {
		return true
	}
	for _, v := range d {
		if v != "" {
			return false
		}
	}
	return true
}

func (d DeltaRow) Clone() DeltaRow {
	res := make(DeltaRow, len(d))
	copy(res, d)
	return res
}
