package main

import (
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"

	"github.com/a1s/a1s/internal/aws"
	"github.com/a1s/a1s/internal/config"
	"github.com/a1s/a1s/internal/config/data"
	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/view"
)

const (
	appName    = "a1s"
	appVersion = "0.1.0"
)

var (
	a1sFlags *data.Flags
	rootCmd  = &cobra.Command{
		Use:   appName,
		Short: "A graphical CLI for AWS management",
		Long:  `a1s is a terminal-based UI for managing AWS resources, inspired by k9s.`,
		RunE:  run,
	}
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s version %s\n", appName, appVersion)
		},
	}
)

func init() {
	a1sFlags = config.NewFlags()
	initA1sFlags()
	rootCmd.AddCommand(versionCmd)
}

func initA1sFlags() {
	rootCmd.Flags().Float32VarP(a1sFlags.RefreshRate, "refresh", "r", 2.0, "Refresh rate in seconds")
	rootCmd.Flags().StringVarP(a1sFlags.LogLevel, "logLevel", "l", "info", "Log level (debug, info, warn, error)")
	rootCmd.Flags().StringVar(a1sFlags.LogFile, "logFile", "", "Log file path")
	rootCmd.Flags().StringVarP(a1sFlags.Command, "command", "c", "", "Startup command/view")
	rootCmd.Flags().BoolVar(a1sFlags.ReadOnly, "readonly", false, "Enable read-only mode")
	rootCmd.Flags().BoolVar(a1sFlags.Write, "write", false, "Enable write mode (overrides readonly)")
	rootCmd.Flags().BoolVar(a1sFlags.Headless, "headless", false, "Run in headless mode")

	// AWS-specific flags
	rootCmd.Flags().StringVar(a1sFlags.Profile, "profile", "", "AWS profile to use")
	rootCmd.Flags().StringVar(a1sFlags.Region, "region", "", "AWS region to use")
	rootCmd.Flags().BoolVarP(a1sFlags.AllRegions, "all-regions", "A", false, "Show resources from all regions")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// 1. Initialize locations
	if err := config.InitLocs(); err != nil {
		return fmt.Errorf("failed to initialize locations: %w", err)
	}

	// 2. Initialize log location
	if err := config.InitLogLoc(); err != nil {
		return fmt.Errorf("failed to initialize log location: %w", err)
	}

	// 3. Load AWS profile settings
	awsSettings, err := aws.NewProfileManager()
	if err != nil {
		return fmt.Errorf("failed to load AWS profiles: %w", err)
	}

	// 4. Create and load configuration
	cfg := config.NewConfig(awsSettings)
	if err := cfg.Load(config.AppConfigFile, false); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// 5. Apply CLI overrides
	cfg.A1s.Override(a1sFlags)

	// 6. Refine configuration (apply precedence logic)
	if err := cfg.Refine(a1sFlags, awsSettings); err != nil {
		return fmt.Errorf("failed to refine configuration: %w", err)
	}

	// 7. Save configuration
	_ = cfg.Save(false)

	// 8. Create AWS client
	profile := cfg.A1s.ActiveProfile()
	region := cfg.A1s.ActiveRegion()
	if region == "" {
		region = aws.DefaultRegion
	}

	clientCfg := &aws.ClientConfig{
		Profile: profile,
		Region:  region,
		Timeout: 30 * time.Second,
	}

	apiClient, err := aws.NewAPIClient(awsSettings, clientCfg)
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	// 9. Create factory from client
	factory := dao.NewFactory(apiClient)

	// 10. Create and initialize the TUI application
	app := view.NewApp(cfg, appVersion)
	app.SetFactory(factory)

	if err := app.Init(); err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}

	// 11. Check connectivity and get account info
	accountID := ""
	if apiClient.CheckConnectivity() {
		accountID = apiClient.AccountID()
	}

	// 12. Set account info
	app.SetAccountInfo(
		profile,
		region,
		accountID,
		appVersion,
	)

	// 13. Run the application
	return app.Run()
}
