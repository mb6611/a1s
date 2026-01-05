package aws

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/ini.v1"
)

type ProfileSettings interface {
	CurrentProfileName() (string, error)
	CurrentRegion() (string, error)
	ProfileNames() (map[string]struct{}, error)
	RegionsForProfile(profile string) ([]string, error)
	GetProfile(name string) (*Profile, error)
	SetActiveProfile(profile, region string) error
}

type Profile struct {
	Name          string
	DefaultRegion string
	Regions       []string
	AccountID     string
	RoleARN       string
	SourceProfile string
}

type ProfileManager struct {
	profiles      map[string]*Profile
	activeProfile string
	activeRegion  string
	mx            sync.RWMutex
}

// commonRegions is a list of common AWS regions for fallback usage.
var commonRegions = []string{
	"us-east-1",
	"us-east-2",
	"us-west-1",
	"us-west-2",
	"eu-west-1",
	"eu-west-2",
	"eu-west-3",
	"eu-central-1",
	"eu-north-1",
	"ap-northeast-1",
	"ap-northeast-2",
	"ap-northeast-3",
	"ap-southeast-1",
	"ap-southeast-2",
	"ap-south-1",
	"ca-central-1",
	"sa-east-1",
}

// NewProfileManager creates a new ProfileManager instance and initializes it with profiles
// from credentials and config files.
func NewProfileManager() (*ProfileManager, error) {
	m := &ProfileManager{
		profiles: make(map[string]*Profile),
	}

	discovery := NewCredentialDiscovery()

	// Discover all available profiles
	discoveredProfiles, err := discovery.DiscoverProfiles()
	if err != nil {
		return nil, fmt.Errorf("failed to discover profiles: %w", err)
	}

	if len(discoveredProfiles) == 0 {
		return nil, fmt.Errorf("no AWS profiles found")
	}

	// Load profile details
	for _, profileName := range discoveredProfiles {
		profile := &Profile{
			Name:    profileName,
			Regions: make([]string, 0),
		}

		// Get credential info for role and source profile information
		credInfo, err := discovery.GetCredentialInfo(profileName)
		if err != nil {
			return nil, fmt.Errorf("failed to get credential info for profile %q: %w", profileName, err)
		}

		profile.RoleARN = credInfo.RoleARN
		profile.SourceProfile = credInfo.SourceProfile

		// Load region and other config from config file (optional)
		_ = m.loadConfigFile(profileName, profile)  // Ignore error, config is optional

		// Default to us-east-1 if no region configured
		if profile.DefaultRegion == "" {
			profile.DefaultRegion = "us-east-1"
		}

		m.profiles[profileName] = profile
	}

	// Set active profile
	defaultProfile, err := discovery.GetDefaultProfile()
	if err != nil {
		return nil, fmt.Errorf("failed to get default profile: %w", err)
	}

	// Verify default profile exists
	if _, exists := m.profiles[defaultProfile]; !exists {
		return nil, fmt.Errorf("default profile %q not found", defaultProfile)
	}

	m.activeProfile = defaultProfile
	m.activeRegion = m.profiles[defaultProfile].DefaultRegion

	return m, nil
}

// loadConfigFile loads profile configuration from the AWS config file.
func (m *ProfileManager) loadConfigFile(profileName string, profile *Profile) error {
	configPath := filepath.Join(expandHomeDir("~"), ".aws", "config")

	// Config file is optional - if it doesn't exist, just return nil
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to access config file: %w", err)
	}

	configFile, err := ini.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	var section *ini.Section
	if profileName == "default" {
		section, err = configFile.GetSection("DEFAULT")
	} else {
		section, err = configFile.GetSection("profile " + profileName)
	}

	if err != nil {
		return fmt.Errorf("profile section not found in config file: %w", err)
	}

	// Load region if present (optional)
	if section.HasKey("region") {
		profile.DefaultRegion = section.Key("region").String()
	}

	// Load account ID if present
	if section.HasKey("account_id") {
		profile.AccountID = section.Key("account_id").String()
	}

	// Load role_arn if present and not already set
	if profile.RoleARN == "" && section.HasKey("role_arn") {
		profile.RoleARN = section.Key("role_arn").String()
	}

	// Load source_profile if present and not already set
	if profile.SourceProfile == "" && section.HasKey("source_profile") {
		profile.SourceProfile = section.Key("source_profile").String()
	}

	return nil
}

// CurrentProfileName returns the name of the currently active profile.
func (m *ProfileManager) CurrentProfileName() (string, error) {
	m.mx.RLock()
	defer m.mx.RUnlock()

	if m.activeProfile == "" {
		return "", fmt.Errorf("no active profile set")
	}

	return m.activeProfile, nil
}

// CurrentRegion returns the region of the currently active profile.
func (m *ProfileManager) CurrentRegion() (string, error) {
	m.mx.RLock()
	defer m.mx.RUnlock()

	if m.activeRegion == "" {
		return "", fmt.Errorf("no active region set")
	}

	return m.activeRegion, nil
}

// ProfileNames returns a map of all available profile names.
func (m *ProfileManager) ProfileNames() (map[string]struct{}, error) {
	m.mx.RLock()
	defer m.mx.RUnlock()

	names := make(map[string]struct{})
	for profileName := range m.profiles {
		names[profileName] = struct{}{}
	}

	return names, nil
}

// RegionsForProfile returns the list of regions available for a profile.
// If the profile has explicit regions configured, those are returned.
// Otherwise, a list of common AWS regions is returned.
func (m *ProfileManager) RegionsForProfile(profile string) ([]string, error) {
	m.mx.RLock()
	defer m.mx.RUnlock()

	p, exists := m.profiles[profile]
	if !exists {
		return nil, fmt.Errorf("profile %q not found", profile)
	}

	if len(p.Regions) > 0 {
		result := make([]string, len(p.Regions))
		copy(result, p.Regions)
		return result, nil
	}

	// Return common regions as fallback
	return commonRegions, nil
}

// GetProfile retrieves a profile by name.
func (m *ProfileManager) GetProfile(name string) (*Profile, error) {
	m.mx.RLock()
	defer m.mx.RUnlock()

	p, exists := m.profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile %q not found", name)
	}

	// Return a copy to prevent external modifications
	profileCopy := &Profile{
		Name:          p.Name,
		DefaultRegion: p.DefaultRegion,
		AccountID:     p.AccountID,
		RoleARN:       p.RoleARN,
		SourceProfile: p.SourceProfile,
	}

	// Copy regions slice
	profileCopy.Regions = make([]string, len(p.Regions))
	copy(profileCopy.Regions, p.Regions)

	return profileCopy, nil
}

// SetActiveProfile sets the currently active profile and region.
// Returns an error if the profile does not exist.
func (m *ProfileManager) SetActiveProfile(profile, region string) error {
	m.mx.Lock()
	defer m.mx.Unlock()

	p, exists := m.profiles[profile]
	if !exists {
		return fmt.Errorf("profile %q not found", profile)
	}

	// If region is empty, use profile's default region
	if region == "" {
		region = p.DefaultRegion
	}

	m.activeProfile = profile
	m.activeRegion = region

	return nil
}
