package data

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// defaultProfilesDir is set by the config package during initialization.
// This avoids a circular import between data and config packages.
var defaultProfilesDir string

// SetDefaultProfilesDir sets the default profiles directory.
// This should be called by the config package during initialization.
func SetDefaultProfilesDir(dir string) {
	defaultProfilesDir = dir
}

// Dir manages the profile configuration directory structure.
// It handles loading and saving profile-specific configurations.
type Dir struct {
	root string        // Base directory for all profiles
	mx   sync.RWMutex
}

// NewDir creates a new Dir using the default profiles directory.
// Note: SetDefaultProfilesDir must be called before using NewDir.
func NewDir() *Dir {
	return &Dir{
		root: defaultProfilesDir,
	}
}

// NewDirAt creates a new Dir at the specified root path.
func NewDirAt(root string) *Dir {
	return &Dir{
		root: root,
	}
}

// ProfilePath returns the path to a profile's configuration directory.
// Returns: {root}/{profile}-{region}/
func (d *Dir) ProfilePath(profile, region string) string {
	d.mx.RLock()
	defer d.mx.RUnlock()

	subpath := SanitizeProfileSubpath(profile, region)
	return filepath.Join(d.root, subpath)
}

// ConfigPath returns the path to a profile's config.yaml file.
// Returns: {root}/{profile}-{region}/config.yaml
func (d *Dir) ConfigPath(profile, region string) string {
	return filepath.Join(d.ProfilePath(profile, region), "config.yaml")
}

// HotkeysPath returns the path to a profile's hotkeys.yaml file.
// Returns: {root}/{profile}-{region}/hotkeys.yaml
func (d *Dir) HotkeysPath(profile, region string) string {
	return filepath.Join(d.ProfilePath(profile, region), "hotkeys.yaml")
}

// AliasesPath returns the path to a profile's aliases.yaml file.
// Returns: {root}/{profile}-{region}/aliases.yaml
func (d *Dir) AliasesPath(profile, region string) string {
	return filepath.Join(d.ProfilePath(profile, region), "aliases.yaml")
}

// EnsureProfileDir creates the profile directory if it doesn't exist.
func (d *Dir) EnsureProfileDir(profile, region string) error {
	d.mx.RLock()
	profilePath := d.ProfilePath(profile, region)
	d.mx.RUnlock()

	_, err := EnsureDirPath(profilePath, 0700)
	return err
}

// Load loads the configuration for a profile/region combination.
// Creates a new default config if the file doesn't exist.
func (d *Dir) Load(profile, region string) (*Config, error) {
	d.mx.RLock()
	configPath := d.ConfigPath(profile, region)
	d.mx.RUnlock()

	ctx := NewProfileContext(profile, region)
	cfg := &Config{
		Context: ctx,
	}

	// Try to load existing config file
	if err := LoadYAML(configPath, ctx); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// File doesn't exist, return default config
			ctx.Validate()
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to load profile config: %w", err)
	}

	ctx.Validate()
	return cfg, nil
}

// Save saves the configuration for a profile/region combination.
func (d *Dir) Save(cfg *Config) error {
	if cfg == nil || cfg.Context == nil {
		return fmt.Errorf("cannot save nil config or context")
	}

	d.mx.RLock()
	configPath := d.ConfigPath(cfg.Context.ProfileName, cfg.Context.Region)
	d.mx.RUnlock()

	// Ensure directory exists
	if err := d.EnsureProfileDir(cfg.Context.ProfileName, cfg.Context.Region); err != nil {
		return fmt.Errorf("failed to ensure profile directory: %w", err)
	}

	// Save config
	if err := SaveYAML(configPath, cfg.Context); err != nil {
		return fmt.Errorf("failed to save profile config: %w", err)
	}

	return nil
}

// ListProfiles returns all profile/region combinations that have saved configs.
// Returns a slice of ProfileContext with just profile and region filled in.
func (d *Dir) ListProfiles() ([]*ProfileContext, error) {
	d.mx.RLock()
	root := d.root
	d.mx.RUnlock()

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("failed to read profiles directory: %w", err)
	}

	var profiles []*ProfileContext

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if config.yaml exists in this directory
		configPath := filepath.Join(root, entry.Name(), "config.yaml")
		if _, err := os.Stat(configPath); err != nil {
			if os.IsNotExist(err) {
				continue // No config.yaml, skip this directory
			}
			return nil, fmt.Errorf("failed to stat config file: %w", err)
		}

		// Parse profile and region from directory name
		// Format is expected to be "{profile}-{region}"
		dirName := entry.Name()
		profile, region := parseProfileSubpath(dirName)

		if profile != "" && region != "" {
			ctx := NewProfileContext(profile, region)
			profiles = append(profiles, ctx)
		}
	}

	return profiles, nil
}

// parseProfileSubpath parses a sanitized profile subpath back into profile and region.
// Expects format: "{profile}-{region}" where components were sanitized.
// Returns the last component as region and everything before the last "-" as profile.
func parseProfileSubpath(subpath string) (string, string) {
	// Find the last occurrence of "-"
	lastIdx := -1
	for i := len(subpath) - 1; i >= 0; i-- {
		if subpath[i] == '-' {
			lastIdx = i
			break
		}
	}

	if lastIdx <= 0 {
		// No valid separator found, can't parse
		return "", ""
	}

	profile := subpath[:lastIdx]
	region := subpath[lastIdx+1:]

	// Both parts must be non-empty
	if profile == "" || region == "" {
		return "", ""
	}

	return profile, region
}
