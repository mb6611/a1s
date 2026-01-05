package config

import (
	"os"
	"sort"
	"sync"

	"github.com/a1s/a1s/internal/config/data"
)

// HotKey represents a single hotkey binding.
type HotKey struct {
	ShortCut    string `yaml:"shortCut"`
	Override    bool   `yaml:"override"`
	Description string `yaml:"description"`
	Command     string `yaml:"command"`
	KeepHistory bool   `yaml:"keepHistory"`
}

// HotKeys represents the hotkeys configuration.
type HotKeys struct {
	HotKey map[string]HotKey `yaml:"hotKeys"`
	mx     sync.RWMutex      `yaml:"-"`
}

// NewHotKeys creates an empty HotKeys configuration.
func NewHotKeys() *HotKeys {
	return &HotKeys{
		HotKey: make(map[string]HotKey),
	}
}

// Load loads hotkeys from the default config file.
// Returns an empty HotKeys if the file doesn't exist.
func (h *HotKeys) Load() error {
	return h.LoadFrom(AppHotkeysFile)
}

// LoadFrom loads hotkeys from a specific file path.
func (h *HotKeys) LoadFrom(path string) error {
	h.mx.Lock()
	defer h.mx.Unlock()

	// If file doesn't exist, return with empty hotkeys
	if _, err := os.Stat(path); os.IsNotExist(err) {
		h.HotKey = make(map[string]HotKey)
		return nil
	}

	if err := data.LoadYAML(path, h); err != nil {
		return err
	}

	// Ensure map is initialized
	if h.HotKey == nil {
		h.HotKey = make(map[string]HotKey)
	}

	return nil
}

// Save saves hotkeys to the default config file.
func (h *HotKeys) Save() error {
	return h.SaveTo(AppHotkeysFile)
}

// SaveTo saves hotkeys to a specific file path.
func (h *HotKeys) SaveTo(path string) error {
	h.mx.RLock()
	defer h.mx.RUnlock()

	return data.SaveYAML(path, h)
}

// Merge merges another HotKeys into this one.
// Keys in other override existing keys.
func (h *HotKeys) Merge(other *HotKeys) {
	h.mx.Lock()
	defer h.mx.Unlock()

	other.mx.RLock()
	defer other.mx.RUnlock()

	for key, hk := range other.HotKey {
		h.HotKey[key] = hk
	}
}

// Get returns a hotkey by name, or nil if not found.
func (h *HotKeys) Get(name string) *HotKey {
	h.mx.RLock()
	defer h.mx.RUnlock()

	hk, ok := h.HotKey[name]
	if !ok {
		return nil
	}

	return &hk
}

// Set sets a hotkey by name.
func (h *HotKeys) Set(name string, hk HotKey) {
	h.mx.Lock()
	defer h.mx.Unlock()

	h.HotKey[name] = hk
}

// Delete removes a hotkey by name.
func (h *HotKeys) Delete(name string) {
	h.mx.Lock()
	defer h.mx.Unlock()

	delete(h.HotKey, name)
}

// Names returns all hotkey names.
func (h *HotKeys) Names() []string {
	h.mx.RLock()
	defer h.mx.RUnlock()

	names := make([]string, 0, len(h.HotKey))
	for name := range h.HotKey {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}
