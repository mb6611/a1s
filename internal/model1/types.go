package model1

import (
	"context"
	"github.com/gdamore/tcell/v2"
)

const NAValue = "n/a"

// ResEvent represents a resource event type
type ResEvent int

const (
	EventUnchanged ResEvent = 1 << iota
	EventAdd
	EventUpdate
	EventDelete
	EventClear
)

// DecoratorFunc decorates a string
type DecoratorFunc func(string) string

// ColorerFunc represents a resource row colorer
type ColorerFunc func(region string, h Header, re *RowEvent) tcell.Color

// Renderer represents a resource renderer
type Renderer interface {
	IsGeneric() bool
	Render(o any, region string, row *Row) error
	Header(region string) Header
	ColorerFunc() ColorerFunc
	Healthy(ctx context.Context, o any) error
}
