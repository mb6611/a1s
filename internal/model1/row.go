package model1

// Row represents a collection of columns
type Row struct {
	ID     string
	Fields Fields
}

func NewRow(size int) Row {
	return Row{Fields: make([]string, size)}
}

func (r Row) Customize(cols []int) Row {
	out := NewRow(len(cols))
	r.Fields.Customize(cols, out.Fields)
	out.ID = r.ID
	return out
}

func (r Row) Diff(ro Row, ageCol int) bool {
	if r.ID != ro.ID {
		return true
	}
	return r.Fields.Diff(ro.Fields, ageCol)
}

func (r Row) Clone() Row {
	return Row{
		ID:     r.ID,
		Fields: r.Fields.Clone(),
	}
}

func (r Row) Len() int {
	return len(r.Fields)
}

// Rows represents a collection of rows
type Rows []Row

func (r Rows) Clone() Rows {
	out := make(Rows, len(r))
	for i, row := range r {
		out[i] = row.Clone()
	}
	return out
}
