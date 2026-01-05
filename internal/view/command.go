// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package view

import (
	"context"
	"fmt"
	"strings"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/ui"
)

// defaultAliases defines command shortcuts for common AWS resources.
var defaultAliases = map[string]string{
	"ec2":  "ec2/instance",
	"i":    "ec2/instance",
	"s3":   "s3/bucket",
	"vpc":  "vpc/vpc",
	"sg":   "vpc/securitygroup",
	"iam":  "iam/user",
	"role": "iam/role",
	"eks":  "eks/cluster",
	"vol":  "ec2/volume",
}

// awsCommands defines valid AWS service commands.
var awsCommands = map[string]bool{
	"ec2":     true,
	"s3":      true,
	"vpc":     true,
	"iam":     true,
	"eks":     true,
	"profile": true,
	"region":  true,
}

// Command handles user command interpretation and execution.
type Command struct {
	app     *App
	aliases map[string]string
}

// NewCommand creates a new command interpreter.
func NewCommand(app *App) *Command {
	return &Command{
		app:     app,
		aliases: make(map[string]string),
	}
}

// Init initializes the command interpreter with default aliases.
func (c *Command) Init() error {
	for k, v := range defaultAliases {
		c.aliases[k] = v
	}
	return nil
}

// Run parses and executes a command.
func (c *Command) Run(cmd string) error {
	if cmd == "" {
		return c.defaultCmd()
	}

	// Remove leading colon if present
	cmd = strings.TrimPrefix(cmd, ":")
	cmd = strings.TrimSpace(cmd)

	// Parse command and arguments
	cmdName, args := c.parseCommand(cmd)

	// Resolve alias
	cmdName = c.resolveAlias(cmdName)

	// Route to appropriate handler
	switch cmdName {
	case "profile":
		if len(args) == 0 {
			// No args - show profile switcher view
			return c.profileView()
		}
		// With args - switch directly
		return c.profileCmd(args[0])

	case "region":
		if len(args) == 0 {
			// TODO: Show region picker view
			return fmt.Errorf("region command requires a region name")
		}
		return c.regionCmd(args[0])

	default:
		// Assume it's a resource command
		return c.resourceCmd(cmdName)
	}
}

// defaultCmd executes the default command (EC2 instances).
func (c *Command) defaultCmd() error {
	return c.resourceCmd("ec2/instance")
}

// profileView shows the profile switcher view.
func (c *Command) profileView() error {
	view := NewProfileSwitcher(c.app)
	view.SetFactory(c.app.GetFactory())

	ctx := context.Background()
	if err := view.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize profile view: %w", err)
	}

	c.app.Flash().Info("Select a profile...")
	c.app.Content.Push("profile", view)
	c.app.SetFocus(view)
	view.Start()

	return nil
}

// profileCmd switches the AWS profile.
func (c *Command) profileCmd(profile string) error {
	if err := c.app.SwitchProfile(profile); err != nil {
		return err
	}

	c.app.Flash().Infof("Switched to profile: %s", profile)

	// Refresh current view to load resources with new profile
	c.app.RefreshCurrentView()

	return nil
}

// regionCmd switches the AWS region.
func (c *Command) regionCmd(region string) error {
	if err := c.app.SwitchRegion(region); err != nil {
		return err
	}

	c.app.Flash().Infof("Switched to region: %s", region)

	// Refresh current view to load resources from new region
	c.app.RefreshCurrentView()

	return nil
}

// resourceCmd navigates to a resource view.
func (c *Command) resourceCmd(rid string) error {
	// Parse resource ID (e.g., "ec2/instance")
	parts := strings.Split(rid, "/")
	if len(parts) < 1 {
		return fmt.Errorf("invalid resource ID: %s", rid)
	}

	service := parts[0]
	var resourceType string
	if len(parts) > 1 {
		resourceType = parts[1]
	}

	// Validate service
	if !awsCommands[service] {
		return fmt.Errorf("unknown service: %s", service)
	}

	if resourceType == "" {
		return fmt.Errorf("resource type required for service: %s", service)
	}

	// Create specialized view based on resource type
	var view ui.Component
	var browser *Browser
	switch rid {
	case "ec2/instance":
		ec2View := NewEC2Instance()
		browser = ec2View.Browser
		view = ec2View
	case "s3/bucket":
		s3View := NewS3Browser()
		browser = s3View.Browser
		view = s3View
	case "vpc/securitygroup":
		sgView := NewSecurityGroup()
		browser = sgView.Browser
		view = sgView
	default:
		// Fall back to generic browser
		resourceID := &dao.ResourceID{
			Service:  service,
			Resource: resourceType,
		}
		browser = NewBrowser(resourceID)
		view = browser
	}

	// Set factory and navigation functions on browser
	if browser != nil {
		browser.SetApp(c.app)
		if c.app.GetFactory() != nil {
			browser.SetFactory(c.app.GetFactory())
		}
		// Set navigation callbacks for describe view
		browser.SetPushFn(func(name string, comp ui.Component) {
			c.app.Content.Push(name, comp)
			c.app.SetFocus(comp)
		})
		browser.SetPopFn(func() {
			c.app.Content.Pop()
		})
	}

	// Initialize and show the view
	ctx := context.Background()
	if err := view.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize view: %w", err)
	}

	c.app.Flash().Infof("Navigating to %s...", rid)
	c.app.Content.Push(rid, view)

	// Set focus to the view so keyboard navigation works
	c.app.SetFocus(view)

	// Start the view to load data
	view.Start()

	return nil
}

// parseCommand parses a command string into command name and arguments.
func (c *Command) parseCommand(cmd string) (string, []string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "", nil
	}

	cmdName := parts[0]
	args := parts[1:]

	return cmdName, args
}

// resolveAlias resolves a command alias to its full form.
func (c *Command) resolveAlias(cmd string) string {
	if alias, ok := c.aliases[cmd]; ok {
		return alias
	}
	return cmd
}
