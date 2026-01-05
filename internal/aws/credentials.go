package aws

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

type CredentialSource int

const (
	CredentialSourceSharedCredentials CredentialSource = iota
	CredentialSourceSharedConfig
	CredentialSourceEnvironment
)

type CredentialInfo struct {
	Profile         string
	Source          CredentialSource
	HasAccessKey    bool
	HasSecretKey    bool
	HasSessionToken bool
	RoleARN         string
	SourceProfile   string
}

type CredentialDiscovery struct {
	credentialsPath string
	configPath      string
}

// NewCredentialDiscovery creates a new CredentialDiscovery instance with default AWS paths.
func NewCredentialDiscovery() *CredentialDiscovery {
	return &CredentialDiscovery{
		credentialsPath: filepath.Join(expandHomeDir("~"), ".aws", "credentials"),
		configPath:      filepath.Join(expandHomeDir("~"), ".aws", "config"),
	}
}

// DiscoverProfiles discovers and returns all available profiles from both credentials and config files.
// Profiles from config file are prefixed with "profile " except for [default].
// Returns an empty list if neither file exists, not an error.
func (d *CredentialDiscovery) DiscoverProfiles() ([]string, error) {
	profileMap := make(map[string]bool)

	// Load profiles from credentials file
	if _, err := os.Stat(d.credentialsPath); err == nil {
		credFile, err := ini.Load(d.credentialsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load credentials file: %w", err)
		}

		for _, section := range credFile.Sections() {
			name := section.Name()
			if name != "DEFAULT" {
				profileMap[name] = true
			}
		}
	}

	// Load profiles from config file
	if _, err := os.Stat(d.configPath); err == nil {
		configFile, err := ini.Load(d.configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}

		for _, section := range configFile.Sections() {
			name := section.Name()
			if name == "DEFAULT" {
				profileMap["default"] = true
			} else if strings.HasPrefix(name, "profile ") {
				// Remove "profile " prefix
				profileName := strings.TrimPrefix(name, "profile ")
				profileMap[profileName] = true
			}
		}
	}

	// Convert map to sorted slice
	var profiles []string
	for profile := range profileMap {
		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// GetDefaultProfile returns the default profile to use.
// First checks AWS_PROFILE environment variable, then returns "default".
func (d *CredentialDiscovery) GetDefaultProfile() (string, error) {
	if profile := os.Getenv("AWS_PROFILE"); profile != "" {
		return profile, nil
	}
	return "default", nil
}

// GetCredentialInfo retrieves credential information for a given profile.
// It checks both the credentials and config files, with credentials file taking precedence.
func (d *CredentialDiscovery) GetCredentialInfo(profile string) (*CredentialInfo, error) {
	info := &CredentialInfo{
		Profile: profile,
	}

	// Check credentials file first
	if _, err := os.Stat(d.credentialsPath); err == nil {
		credFile, err := ini.Load(d.credentialsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load credentials file: %w", err)
		}

		section, err := credFile.GetSection(profile)
		if err == nil {
			info.Source = CredentialSourceSharedCredentials
			info.HasAccessKey = section.HasKey("aws_access_key_id")
			info.HasSecretKey = section.HasKey("aws_secret_access_key")
			info.HasSessionToken = section.HasKey("aws_session_token")

			if section.HasKey("role_arn") {
				info.RoleARN = section.Key("role_arn").String()
			}
			if section.HasKey("source_profile") {
				info.SourceProfile = section.Key("source_profile").String()
			}

			return info, nil
		}
	}

	// Check config file
	if _, err := os.Stat(d.configPath); err == nil {
		configFile, err := ini.Load(d.configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}

		var section *ini.Section
		if profile == "default" {
			section, err = configFile.GetSection("DEFAULT")
		} else {
			section, err = configFile.GetSection("profile " + profile)
		}

		if err == nil {
			info.Source = CredentialSourceSharedConfig
			info.HasAccessKey = section.HasKey("aws_access_key_id")
			info.HasSecretKey = section.HasKey("aws_secret_access_key")
			info.HasSessionToken = section.HasKey("aws_session_token")

			if section.HasKey("role_arn") {
				info.RoleARN = section.Key("role_arn").String()
			}
			if section.HasKey("source_profile") {
				info.SourceProfile = section.Key("source_profile").String()
			}

			return info, nil
		}
	}

	return nil, fmt.Errorf("profile %q not found in credentials or config files", profile)
}

// expandHomeDir expands ~ to the user's home directory.
func expandHomeDir(path string) string {
	if strings.HasPrefix(path, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, strings.TrimPrefix(path, "~"))
	}
	return path
}
