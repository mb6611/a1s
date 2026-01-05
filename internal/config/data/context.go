package data

import "sync"

// ProfileContext represents the configuration context for a specific profile/region combination.
// It stores per-profile settings that override global configuration.
type ProfileContext struct {
	ProfileName  string       `yaml:"profile"`
	Region       string       `yaml:"region"`
	ReadOnly     *bool        `yaml:"readOnly,omitempty"`
	Skin         string       `yaml:"skin,omitempty"`
	View         *View        `yaml:"view,omitempty"`
	FeatureGates FeatureGates `yaml:"featureGates,omitempty"`
	mx           sync.RWMutex `yaml:"-"`
}

// NewProfileContext creates a new ProfileContext with default settings.
func NewProfileContext(profile, region string) *ProfileContext {
	return &ProfileContext{
		ProfileName:  profile,
		Region:       region,
		ReadOnly:     nil,
		Skin:         "",
		View:         nil,
		FeatureGates: NewFeatureGates(),
	}
}

// Validate ensures the ProfileContext has valid settings.
func (c *ProfileContext) Validate() {
	c.mx.Lock()
	defer c.mx.Unlock()

	if c.View != nil {
		c.View.Validate()
	}
}

// GetView returns the current view, creating a default if nil.
func (c *ProfileContext) GetView() *View {
	c.mx.RLock()
	defer c.mx.RUnlock()

	if c.View == nil {
		return NewView()
	}
	return c.View
}

// SetView sets the current view.
func (c *ProfileContext) SetView(v *View) {
	c.mx.Lock()
	defer c.mx.Unlock()

	c.View = v
}

// IsReadOnly returns whether this context is in read-only mode.
// Returns false if ReadOnly is nil.
func (c *ProfileContext) IsReadOnly() bool {
	c.mx.RLock()
	defer c.mx.RUnlock()

	if c.ReadOnly == nil {
		return false
	}
	return *c.ReadOnly
}

// SetReadOnly sets the read-only mode for this context.
func (c *ProfileContext) SetReadOnly(ro bool) {
	c.mx.Lock()
	defer c.mx.Unlock()

	c.ReadOnly = &ro
}

// ContextName returns the context name in format "profile-region".
func (c *ProfileContext) ContextName() string {
	c.mx.RLock()
	defer c.mx.RUnlock()

	return SanitizeProfileSubpath(c.ProfileName, c.Region)
}
