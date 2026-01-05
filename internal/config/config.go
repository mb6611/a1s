package config

import (
	"fmt"
	"os"
	"sync"

	"github.com/a1s/a1s/internal/aws"
	"github.com/a1s/a1s/internal/config/data"
)

// Config is the root configuration for the application.
type Config struct {
	A1s      *A1s             `yaml:"a1s"`
	conn     aws.Connection
	settings aws.ProfileSettings
	mx       sync.RWMutex
}

// NewConfig creates a new Config with the given profile settings.
func NewConfig(settings aws.ProfileSettings) *Config {
	return &Config{
		A1s:      NewA1s(),
		settings: settings,
	}
}

// Load loads the configuration from the given path.
// If the file doesn't exist, the current config is kept.
func (c *Config) Load(path string, force bool) error {
	c.mx.Lock()
	defer c.mx.Unlock()

	// If file doesn't exist and force is false, keep current config
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if !force {
			return nil
		}
		return fmt.Errorf("config file does not exist: %s", path)
	}

	// Load YAML into config
	if err := data.LoadYAML(path, c); err != nil {
		return fmt.Errorf("failed to load config from %s: %w", path, err)
	}

	// Validate loaded config
	if c.A1s != nil {
		c.A1s.Validate()
	}

	return nil
}

// Save saves the configuration to the given path.
// If force is false, only saves if the file already exists.
func (c *Config) Save(force bool) error {
	c.mx.RLock()
	defer c.mx.RUnlock()

	// Determine save path
	path := AppConfigFile
	if path == "" {
		return fmt.Errorf("no config file path configured")
	}

	// Check if file exists
	_, err := os.Stat(path)
	fileExists := err == nil

	// If force is false and file doesn't exist, skip save
	if !force && !fileExists {
		return nil
	}

	// Save config as YAML
	if err := data.SaveYAML(path, c); err != nil {
		return fmt.Errorf("failed to save config to %s: %w", path, err)
	}

	return nil
}

// Refine applies CLI flags and AWS settings to determine the final configuration.
// This implements the configuration precedence logic:
// - Profile: CLI --profile > config defaultProfile > AWS default
// - Region: CLI --all-regions > CLI --region > profile config > profile default
func (c *Config) Refine(flags *data.Flags, settings aws.ProfileSettings) error {
	c.mx.Lock()
	defer c.mx.Unlock()

	if c.A1s == nil {
		return fmt.Errorf("config.A1s is nil")
	}

	// Update settings
	c.settings = settings

	// Determine profile using precedence: CLI > config default > AWS default
	profile := ""
	if flags != nil && flags.Profile != nil && *flags.Profile != "" {
		profile = *flags.Profile
	} else if c.A1s.DefaultProfile != "" {
		profile = c.A1s.DefaultProfile
	} else {
		// Use AWS default profile
		awsDefault, err := settings.CurrentProfileName()
		if err != nil {
			return fmt.Errorf("failed to get AWS default profile: %w", err)
		}
		profile = awsDefault
	}

	// Verify profile exists
	_, err := settings.GetProfile(profile)
	if err != nil {
		return fmt.Errorf("profile %q not found: %w", profile, err)
	}

	// Determine region using precedence
	region := ""
	if flags != nil && flags.AllRegions != nil && *flags.AllRegions {
		// --all-regions takes highest precedence, sets to "all"
		region = "all"
	} else if flags != nil && flags.Region != nil && *flags.Region != "" {
		// CLI --region takes next precedence
		region = *flags.Region
	} else if c.A1s.DefaultRegion != "" {
		// Config default region
		region = c.A1s.DefaultRegion
	} else {
		// Fall back to profile's default region
		profileData, err := settings.GetProfile(profile)
		if err != nil {
			return fmt.Errorf("failed to get profile data: %w", err)
		}
		region = profileData.DefaultRegion
	}

	// Ensure region is set
	if region == "" {
		return fmt.Errorf("no region configured for profile %q", profile)
	}

	// Activate the profile/region combination
	if _, err := c.A1s.ActivateProfile(profile, region); err != nil {
		return fmt.Errorf("failed to activate profile %q with region %q: %w", profile, region, err)
	}

	// Apply CLI flag overrides
	if flags != nil {
		c.A1s.Override(flags)
	}

	return nil
}

// Connection returns the AWS connection.
func (c *Config) Connection() aws.Connection {
	c.mx.RLock()
	defer c.mx.RUnlock()
	return c.conn
}

// SetConnection sets the AWS connection.
func (c *Config) SetConnection(conn aws.Connection) {
	c.mx.Lock()
	defer c.mx.Unlock()
	c.conn = conn
}

// Settings returns the AWS profile settings.
func (c *Config) Settings() aws.ProfileSettings {
	c.mx.RLock()
	defer c.mx.RUnlock()
	return c.settings
}
