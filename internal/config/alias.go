package config

import (
	"os"
	"sync"

	"github.com/a1s/a1s/internal/config/data"
)

// Aliases represents the alias configuration.
type Aliases struct {
	Alias map[string]string `yaml:"aliases"`
	mx    sync.RWMutex      `yaml:"-"`
}

// DefaultAliases are the built-in aliases for AWS resources.
var DefaultAliases = map[string]string{
	// EC2
	"ec2":      "ec2/instance",
	"i":        "ec2/instance",
	"vol":      "ec2/volume",
	"ebs":      "ec2/volume",
	"sg":       "ec2/security-group",

	// VPC
	"vpc":      "vpc/vpc",
	"subnet":   "vpc/subnet",
	"sn":       "vpc/subnet",
	"igw":      "vpc/internet-gateway",
	"nat":      "vpc/nat-gateway",
	"rt":       "vpc/route-table",
	"acl":      "vpc/network-acl",

	// S3
	"s3":     "s3/bucket",
	"bucket": "s3/bucket",

	// IAM
	"iam":    "iam/user",
	"user":   "iam/user",
	"role":   "iam/role",
	"policy": "iam/policy",

	// EKS
	"eks":       "eks/cluster",
	"cluster":   "eks/cluster",
	"ng":        "eks/nodegroup",
	"nodegroup": "eks/nodegroup",

	// k9s compatibility
	"ctx": "profile",  // Context = Profile
	"ns":  "region",   // Namespace = Region
}

// NewAliases creates an Aliases with default aliases loaded.
func NewAliases() *Aliases {
	a := &Aliases{
		Alias: make(map[string]string),
	}
	// Copy default aliases
	for k, v := range DefaultAliases {
		a.Alias[k] = v
	}
	return a
}

// Load loads aliases from the default config file.
// Merges with default aliases, with file aliases taking precedence.
func (a *Aliases) Load() error {
	return a.LoadFrom(AppAliasesFile)
}

// LoadFrom loads aliases from a specific file path.
func (a *Aliases) LoadFrom(path string) error {
	a.mx.Lock()
	defer a.mx.Unlock()

	// If file doesn't exist, just use defaults
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	loaded := &Aliases{
		Alias: make(map[string]string),
	}
	if err := data.LoadYAML(path, loaded); err != nil {
		return err
	}

	// Merge loaded aliases into current (loaded takes precedence)
	for k, v := range loaded.Alias {
		a.Alias[k] = v
	}

	return nil
}

// Save saves aliases to the default config file.
func (a *Aliases) Save() error {
	return a.SaveTo(AppAliasesFile)
}

// SaveTo saves aliases to a specific file path.
func (a *Aliases) SaveTo(path string) error {
	a.mx.RLock()
	defer a.mx.RUnlock()

	return data.SaveYAML(path, a)
}

// Merge merges another Aliases into this one.
// Keys in other override existing keys.
func (a *Aliases) Merge(other *Aliases) {
	a.mx.Lock()
	defer a.mx.Unlock()

	other.mx.RLock()
	defer other.mx.RUnlock()

	for k, v := range other.Alias {
		a.Alias[k] = v
	}
}

// Get returns the resource for an alias, or the original if not found.
func (a *Aliases) Get(alias string) string {
	a.mx.RLock()
	defer a.mx.RUnlock()

	if resource, ok := a.Alias[alias]; ok {
		return resource
	}
	return alias
}

// Set sets an alias.
func (a *Aliases) Set(alias, resource string) {
	a.mx.Lock()
	defer a.mx.Unlock()

	a.Alias[alias] = resource
}

// Delete removes an alias.
func (a *Aliases) Delete(alias string) {
	a.mx.Lock()
	defer a.mx.Unlock()

	delete(a.Alias, alias)
}

// All returns a copy of all aliases.
func (a *Aliases) All() map[string]string {
	a.mx.RLock()
	defer a.mx.RUnlock()

	result := make(map[string]string)
	for k, v := range a.Alias {
		result[k] = v
	}
	return result
}
