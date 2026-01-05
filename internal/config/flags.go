package config

import (
	"github.com/a1s/a1s/internal/config/data"
)

// DefaultRefreshRate is the default data refresh interval in seconds.
const DefaultRefreshRate = 2.0

// DefaultLogLevel is the default logging level.
const DefaultLogLevel = "info"

// NewFlags creates a new Flags instance with default values set.
func NewFlags() *data.Flags {
	refreshRate := float32(DefaultRefreshRate)
	logLevel := DefaultLogLevel
	logFile := AppLogFile
	headless := false
	command := ""
	readOnly := false
	write := false
	profile := ""
	region := ""
	allRegions := false

	return &data.Flags{
		RefreshRate: &refreshRate,
		LogLevel:    &logLevel,
		LogFile:     &logFile,
		Headless:    &headless,
		Command:     &command,
		ReadOnly:    &readOnly,
		Write:       &write,
		Profile:     &profile,
		Region:      &region,
		AllRegions:  &allRegions,
	}
}

// IsBoolSet returns true if a bool pointer is non-nil and true.
func IsBoolSet(b *bool) bool {
	return b != nil && *b
}

// IsStringSet returns true if a string pointer is non-nil and non-empty.
func IsStringSet(s *string) bool {
	return s != nil && *s != ""
}
