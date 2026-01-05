package model1

import "fmt"

// RowEvent tracks resource instance events
type RowEvent struct {
	Kind   ResEvent
	Row    Row
	Deltas DeltaRow
}

func NewRowEvent(kind ResEvent, row Row) RowEvent {
	return RowEvent{
		Kind: kind,
		Row:  row,
	}
}

func NewRowEventWithDeltas(row Row, delta DeltaRow) RowEvent {
	return RowEvent{
		Kind:   EventUpdate,
		Row:    row,
		Deltas: delta,
	}
}

func (r RowEvent) Clone() RowEvent {
	return RowEvent{
		Kind:   r.Kind,
		Row:    r.Row.Clone(),
		Deltas: r.Deltas.Clone(),
	}
}

func (r RowEvent) Customize(cols []int) RowEvent {
	delta := r.Deltas
	if !r.Deltas.IsBlank() {
		delta = make(DeltaRow, len(cols))
		r.Deltas.Customize(cols, delta)
	}
	return RowEvent{
		Kind:   r.Kind,
		Deltas: delta,
		Row:    r.Row.Customize(cols),
	}
}

func (r RowEvent) Diff(re RowEvent, ageCol int) bool {
	if r.Kind != re.Kind {
		return true
	}
	if r.Deltas.Diff(re.Deltas, ageCol) {
		return true
	}
	return r.Row.Diff(re.Row, ageCol)
}

// RowEvents a collection of row events
type RowEvents struct {
	events []RowEvent
	index  map[string]int
}

func NewRowEvents(size int) *RowEvents {
	return &RowEvents{
		events: make([]RowEvent, 0, size),
		index:  make(map[string]int, size),
	}
}

func (r *RowEvents) reindex() {
	for i, e := range r.events {
		r.index[e.Row.ID] = i
	}
}

func (r *RowEvents) At(i int) (RowEvent, bool) {
	if i < 0 || i >= len(r.events) {
		return RowEvent{}, false
	}
	return r.events[i], true
}

func (r *RowEvents) Set(i int, re RowEvent) {
	r.events[i] = re
	r.index[re.Row.ID] = i
}

func (r *RowEvents) Add(re RowEvent) {
	r.events = append(r.events, re)
	r.index[re.Row.ID] = len(r.events) - 1
}

func (r *RowEvents) Len() int {
	return len(r.events)
}

func (r *RowEvents) Empty() bool {
	return len(r.events) == 0
}

func (r *RowEvents) Clear() {
	r.events = r.events[:0]
	for k := range r.index {
		delete(r.index, k)
	}
}

func (r *RowEvents) Get(id string) (RowEvent, bool) {
	i, ok := r.index[id]
	if !ok {
		return RowEvent{}, false
	}
	return r.At(i)
}

func (r *RowEvents) FindIndex(id string) (int, bool) {
	i, ok := r.index[id]
	return i, ok
}

func (r *RowEvents) Upsert(re RowEvent) {
	if idx, ok := r.FindIndex(re.Row.ID); ok {
		r.events[idx] = re
	} else {
		r.Add(re)
	}
}

func (r *RowEvents) Delete(fqn string) error {
	victim, ok := r.FindIndex(fqn)
	if !ok {
		return fmt.Errorf("unable to delete row with id: %q", fqn)
	}
	r.events = append(r.events[0:victim], r.events[victim+1:]...)
	delete(r.index, fqn)
	r.reindex()
	return nil
}

func (r *RowEvents) Clone() *RowEvents {
	re := make([]RowEvent, 0, len(r.events))
	for _, e := range r.events {
		re = append(re, e.Clone())
	}
	out := NewRowEvents(len(re))
	for _, e := range re {
		out.Add(e)
	}
	return out
}

func (r *RowEvents) Range(f func(int, RowEvent) bool) {
	for i, e := range r.events {
		if !f(i, e) {
			return
		}
	}
}

// Count returns the number of events.
func (r *RowEvents) Count() int {
	return len(r.events)
}
