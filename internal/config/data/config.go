package data

import "sync"

// Config represents a profile-specific configuration loaded from disk.
// This is the data structure for ~/.local/share/a1s/profiles/{profile}-{region}/config.yaml
type Config struct {
	Context *ProfileContext `yaml:"a1s"`
	mx      sync.RWMutex    `yaml:"-"`
}

// NewConfig creates a new Config with the given profile context.
func NewConfig(ctx *ProfileContext) *Config {
	return &Config{
		Context: ctx,
	}
}

// NewEmptyConfig creates a Config with nil context.
func NewEmptyConfig() *Config {
	return &Config{}
}

// GetContext returns the profile context, thread-safe.
func (c *Config) GetContext() *ProfileContext {
	c.mx.RLock()
	defer c.mx.RUnlock()

	return c.Context
}

// SetContext sets the profile context, thread-safe.
func (c *Config) SetContext(ctx *ProfileContext) {
	c.mx.Lock()
	defer c.mx.Unlock()

	c.Context = ctx
}

// Validate ensures the Config has valid settings.
func (c *Config) Validate() {
	c.mx.Lock()
	defer c.mx.Unlock()

	if c.Context != nil {
		c.Context.Validate()
	}
}

// Save writes the config to disk at the given path.
func (c *Config) Save(path string) error {
	c.mx.RLock()
	defer c.mx.RUnlock()

	return SaveYAML(path, c)
}

// Load reads the config from disk at the given path.
// Creates a new config if the file doesn't exist.
func (c *Config) Load(path string) error {
	c.mx.Lock()
	defer c.mx.Unlock()

	if err := LoadYAML(path, c); err != nil {
		return err
	}

	return nil
}
