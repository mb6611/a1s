package config

import (
	"os"
	"path/filepath"

	"github.com/a1s/a1s/internal/config/data"
)

const AppName = "a1s"

var (
	// AppConfigDir is ~/.config/a1s
	AppConfigDir string

	// AppDataDir is ~/.local/share/a1s
	AppDataDir string

	// AppStateDir is ~/.local/state/a1s
	AppStateDir string

	// AppConfigFile is ~/.config/a1s/a1s.yaml
	AppConfigFile string

	// AppHotkeysFile is ~/.config/a1s/hotkeys.yaml
	AppHotkeysFile string

	// AppAliasesFile is ~/.config/a1s/aliases.yaml
	AppAliasesFile string

	// AppSkinsDir is ~/.config/a1s/skins
	AppSkinsDir string

	// AppProfilesDir is ~/.local/share/a1s/profiles
	AppProfilesDir string

	// AppLogFile is ~/.local/state/a1s/a1s.log
	AppLogFile string

	// AppDumpsDir is ~/.local/state/a1s/screen-dumps
	AppDumpsDir string
)

// InitLocs initializes all application directory paths.
// It respects XDG environment variables if set.
func InitLocs() error {
	home := userHomeDir()

	// Determine base directories respecting XDG standards
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}

	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = filepath.Join(home, ".local", "state")
	}

	// Set application directories
	AppConfigDir = filepath.Join(configHome, AppName)
	AppDataDir = filepath.Join(dataHome, AppName)
	AppStateDir = filepath.Join(stateHome, AppName)

	// Set application files
	AppConfigFile = filepath.Join(AppConfigDir, "a1s.yaml")
	AppHotkeysFile = filepath.Join(AppConfigDir, "hotkeys.yaml")
	AppAliasesFile = filepath.Join(AppConfigDir, "aliases.yaml")
	AppSkinsDir = filepath.Join(AppConfigDir, "skins")

	// Set data and state directories
	AppProfilesDir = filepath.Join(AppDataDir, "profiles")
	AppLogFile = filepath.Join(AppStateDir, "a1s.log")
	AppDumpsDir = filepath.Join(AppStateDir, "screen-dumps")

	// Set default profiles directory in data package to avoid circular import
	data.SetDefaultProfilesDir(AppProfilesDir)

	// Create all directories
	dirs := []string{
		AppConfigDir,
		AppDataDir,
		AppStateDir,
		AppSkinsDir,
		AppProfilesDir,
		AppDumpsDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}

	return nil
}

// InitLogLoc ensures the log directory exists
func InitLogLoc() error {
	logDir := filepath.Dir(AppLogFile)
	return os.MkdirAll(logDir, 0700)
}

// userHomeDir returns the user's home directory
func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return home
}
