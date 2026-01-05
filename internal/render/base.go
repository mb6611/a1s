package render

import (
	"context"

	"github.com/a1s/a1s/internal/model1"
)

// Base provides a base renderer implementation
type Base struct{}

// IsGeneric identifies a generic handler
func (*Base) IsGeneric() bool {
	return false
}

// ColorerFunc returns the default colorer
func (*Base) ColorerFunc() model1.ColorerFunc {
	return model1.DefaultColorer
}

// Healthy checks if the resource is healthy
func (*Base) Healthy(context.Context, any) error {
	return nil
}
