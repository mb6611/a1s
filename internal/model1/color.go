package model1

import "github.com/gdamore/tcell/v2"

var (
	// ModColor row modified color
	ModColor tcell.Color = tcell.ColorYellow

	// AddColor row added color
	AddColor tcell.Color = tcell.ColorBlue

	// PendingColor row pending color
	PendingColor tcell.Color = tcell.ColorDarkCyan

	// ErrColor row error color
	ErrColor tcell.Color = tcell.ColorRed

	// StdColor row default color
	StdColor tcell.Color = tcell.ColorWhite

	// HighlightColor row highlight color
	HighlightColor tcell.Color = tcell.ColorAqua

	// KillColor row deleted/terminated color
	KillColor tcell.Color = tcell.ColorGray

	// CompletedColor row completed color
	CompletedColor tcell.Color = tcell.ColorGreen
)

// DefaultColorer set the default table row colors
func DefaultColorer(region string, h Header, re *RowEvent) tcell.Color {
	if !IsValid(region, h, re.Row) {
		return ErrColor
	}

	switch re.Kind {
	case EventAdd:
		return AddColor
	case EventUpdate:
		return ModColor
	case EventDelete:
		return KillColor
	default:
		return StdColor
	}
}
