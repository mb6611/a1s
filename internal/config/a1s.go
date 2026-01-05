package config

import (
	"fmt"
	"sync"
	"time"

	"github.com/a1s/a1s/internal/config/data"
)

// Default values
const (
	DefaultAPITimeout = 30 * time.Second
	DefaultView       = "ec2"
)

// A1s represents the a1s global configuration.
type A1s struct {
	RefreshRate    float32     `yaml:"refreshRate"`
	APITimeout     string      `yaml:"apiTimeout"`
	ReadOnly       bool        `yaml:"readOnly"`
	DefaultView    string      `yaml:"defaultView"`
	DefaultProfile string      `yaml:"defaultProfile"`
	DefaultRegion  string      `yaml:"defaultRegion"`
	UI             data.UI     `yaml:"ui"`
	Logger         data.Logger `yaml:"logger"`

	// Internal state (not serialized)
	activeProfile string
	activeRegion  string
	activeConfig  *data.Config
	dir           *data.Dir
	mx            sync.RWMutex
}

// NewA1s creates an A1s with default settings.
func NewA1s() *A1s {
	return &A1s{
		RefreshRate: DefaultRefreshRate,
		APITimeout:  DefaultAPITimeout.String(),
		ReadOnly:    false,
		DefaultView: DefaultView,
		dir:         data.NewDir(),
	}
}

// Validate ensures A1s has valid settings.
func (a *A1s) Validate() {
	a.mx.Lock()
	defer a.mx.Unlock()

	if a.RefreshRate <= 0 {
		a.RefreshRate = DefaultRefreshRate
	}

	if a.APITimeout == "" {
		a.APITimeout = DefaultAPITimeout.String()
	}

	if a.DefaultView == "" {
		a.DefaultView = DefaultView
	}
}

// ActiveProfile returns the currently active AWS profile.
func (a *A1s) ActiveProfile() string {
	a.mx.RLock()
	defer a.mx.RUnlock()
	return a.activeProfile
}

// ActiveRegion returns the currently active AWS region.
func (a *A1s) ActiveRegion() string {
	a.mx.RLock()
	defer a.mx.RUnlock()
	return a.activeRegion
}

// ActiveConfig returns the current profile-specific configuration.
func (a *A1s) ActiveConfig() *data.Config {
	a.mx.RLock()
	defer a.mx.RUnlock()
	return a.activeConfig
}

// ActivateProfile activates a profile/region combination and loads its config.
func (a *A1s) ActivateProfile(profile, region string) (*data.ProfileContext, error) {
	if profile == "" {
		return nil, fmt.Errorf("profile cannot be empty")
	}
	if region == "" {
		return nil, fmt.Errorf("region cannot be empty")
	}

	a.mx.Lock()
	defer a.mx.Unlock()

	// Load profile-specific configuration
	cfg, err := a.dir.Load(profile, region)
	if err != nil {
		return nil, fmt.Errorf("failed to load config for profile %q: %w", profile, err)
	}

	a.activeProfile = profile
	a.activeRegion = region
	a.activeConfig = cfg

	ctx := data.NewProfileContext(profile, region)
	return ctx, nil
}

// SwitchProfile switches to a different profile, keeping current region if possible.
func (a *A1s) SwitchProfile(profile string) (*data.ProfileContext, error) {
	if profile == "" {
		return nil, fmt.Errorf("profile cannot be empty")
	}

	a.mx.Lock()
	currentRegion := a.activeRegion
	if currentRegion == "" {
		currentRegion = a.DefaultRegion
	}
	a.mx.Unlock()

	if currentRegion == "" {
		return nil, fmt.Errorf("cannot switch profile: no active or default region configured")
	}

	return a.ActivateProfile(profile, currentRegion)
}

// SwitchRegion switches to a different region within the current profile.
func (a *A1s) SwitchRegion(region string) (*data.ProfileContext, error) {
	if region == "" {
		return nil, fmt.Errorf("region cannot be empty")
	}

	a.mx.Lock()
	currentProfile := a.activeProfile
	a.mx.Unlock()

	if currentProfile == "" {
		return nil, fmt.Errorf("no active profile set")
	}

	return a.ActivateProfile(currentProfile, region)
}

// Override applies CLI flag overrides to the configuration.
func (a *A1s) Override(flags *data.Flags) {
	if flags == nil {
		return
	}

	a.mx.Lock()
	defer a.mx.Unlock()

	if flags.RefreshRate != nil {
		a.RefreshRate = *flags.RefreshRate
	}

	if flags.ReadOnly != nil {
		a.ReadOnly = *flags.ReadOnly
	}

	// Write flag overrides ReadOnly
	if flags.Write != nil && *flags.Write {
		a.ReadOnly = false
	}

	if flags.Profile != nil && *flags.Profile != "" {
		a.DefaultProfile = *flags.Profile
	}

	if flags.Region != nil && *flags.Region != "" {
		a.DefaultRegion = *flags.Region
	}
}

// GetAPITimeout returns the parsed API timeout duration.
func (a *A1s) GetAPITimeout() (time.Duration, error) {
	a.mx.RLock()
	timeoutStr := a.APITimeout
	a.mx.RUnlock()

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return 0, fmt.Errorf("invalid API timeout %q: %w", timeoutStr, err)
	}

	return timeout, nil
}

// setActiveConfig sets the active configuration (internal).
func (a *A1s) setActiveConfig(cfg *data.Config) {
	a.mx.Lock()
	defer a.mx.Unlock()
	a.activeConfig = cfg
}
