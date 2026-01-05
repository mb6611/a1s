// Package data provides configuration data types and interfaces for the a1s application.
package data

import (
	"github.com/a1s/a1s/internal/aws"
)

// AWSProfileSettings defines the interface for AWS profile configuration management.
// It wraps the aws.ProfileSettings interface to provide profile and region information.
type AWSProfileSettings interface {
	CurrentProfileName() (string, error)
	CurrentRegion() (string, error)
	ProfileNames() (map[string]struct{}, error)
	GetProfile(name string) (*aws.Profile, error)
	SetActiveProfile(profile, region string) error
}

// Flags represents CLI command-line flags for the a1s application.
type Flags struct {
	RefreshRate *float32 // Refresh rate in seconds
	LogLevel    *string  // Log level (e.g., debug, info, warn, error)
	LogFile     *string  // Path to log file
	Headless    *bool    // Run in headless mode (no TUI)
	Command     *string  // Command to execute
	ReadOnly    *bool    // Run in read-only mode
	Write       *bool    // Enable write operations
	Profile     *string  // AWS profile to use
	Region      *string  // AWS region to use
	AllRegions  *bool    // Query all regions
}

// UI represents user interface configuration settings.
type UI struct {
	EnableMouse bool   `yaml:"enableMouse"`
	Headless    bool   `yaml:"headless"`
	Logoless    bool   `yaml:"logoless"`
	Crumbsless  bool   `yaml:"crumbsless"`
	Skin        string `yaml:"skin"`
}

// Logger represents logging configuration settings.
type Logger struct {
	Tail         int `yaml:"tail"`
	Buffer       int `yaml:"buffer"`
	SinceSeconds int `yaml:"sinceSeconds"`
}

// Logger configuration constants.
const (
	DefaultLoggerTail   = 100
	DefaultLoggerBuffer = 5000
)

// NewFlags creates a new Flags instance with all pointer fields initialized.
// All pointers are allocated but their values are not set.
func NewFlags() *Flags {
	return &Flags{
		RefreshRate: new(float32),
		LogLevel:    new(string),
		LogFile:     new(string),
		Headless:    new(bool),
		Command:     new(string),
		ReadOnly:    new(bool),
		Write:       new(bool),
		Profile:     new(string),
		Region:      new(string),
		AllRegions:  new(bool),
	}
}
