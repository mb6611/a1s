// Package data provides helpers for configuration file operations and path management.
package data

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

// invalidPathCharsRX matches characters not allowed in file paths
var invalidPathCharsRX = regexp.MustCompile(`[:/]+`)

// SanitizeFileName replaces invalid characters in a filename
func SanitizeFileName(name string) string {
	return invalidPathCharsRX.ReplaceAllString(name, "-")
}

// SanitizeProfileSubpath creates a safe subpath for profile/region combinations
// Returns: "{profile}-{region}" with sanitized components
func SanitizeProfileSubpath(profile, region string) string {
	return SanitizeFileName(profile) + "-" + SanitizeFileName(region)
}

// EnsureDirPath creates a directory path if it doesn't exist
// Returns the path for convenience
func EnsureDirPath(path string, perm os.FileMode) (string, error) {
	if err := os.MkdirAll(path, perm); err != nil {
		return "", fmt.Errorf("failed to create directory %q: %w", path, err)
	}
	return path, nil
}

// EnsureFullPath ensures both the directory and parent directories exist
func EnsureFullPath(path string, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("failed to create full path for %q: %w", path, err)
	}
	return nil
}

// SaveYAML saves a struct to a YAML file
func SaveYAML(path string, data interface{}) error {
	if err := EnsureFullPath(path, 0700); err != nil {
		return err
	}

	bytes, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	if err := os.WriteFile(path, bytes, 0600); err != nil {
		return fmt.Errorf("failed to write YAML file %q: %w", path, err)
	}

	return nil
}

// LoadYAML loads a YAML file into a struct
func LoadYAML(path string, data interface{}) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read YAML file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(bytes, data); err != nil {
		return fmt.Errorf("failed to unmarshal YAML from %q: %w", path, err)
	}

	return nil
}

// MustLoadYAML loads a YAML file, returns error if file doesn't exist
func MustLoadYAML(path string, data interface{}) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("YAML file %q does not exist", path)
		}
		return fmt.Errorf("failed to stat YAML file %q: %w", path, err)
	}

	return LoadYAML(path, data)
}
