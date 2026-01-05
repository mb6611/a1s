// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package view

import (
	"context"
	"time"

	"github.com/a1s/a1s/internal/aws"
	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/ui"
	"github.com/derailed/tcell/v2"
)

// EC2Instance represents an EC2 instance view with instance management actions.
type EC2Instance struct {
	*Browser
}

// NewEC2Instance returns a new EC2 instance view.
func NewEC2Instance() *EC2Instance {
	rid := &dao.ResourceID{
		Service:  "ec2",
		Resource: "instance",
	}
	return &EC2Instance{
		Browser: NewBrowser(rid),
	}
}

// Init initializes the EC2 instance view.
func (e *EC2Instance) Init(ctx context.Context) error {
	if err := e.Browser.Init(ctx); err != nil {
		return err
	}

	e.bindEC2Keys(e.Actions())
	return nil
}

// Name returns the component name for breadcrumbs.
func (e *EC2Instance) Name() string {
	return "ec2-instance"
}

// bindEC2Keys sets up EC2 instance-specific key bindings.
// Note: Start/Stop/Reboot/Terminate are handled by the action registry in ui/ec2_actions.go
func (e *EC2Instance) bindEC2Keys(aa *ui.KeyActions) {
	aa.Bulk(ui.KeyMap{
		ui.KeyC:      ui.NewKeyAction("Connect (SSH/SSM)", e.connectCmd, true),
		ui.KeyShiftS: ui.NewKeyAction("Setup SSM", e.setupSSMCmd, true),
		ui.KeyL:      ui.NewKeyAction("View Logs", e.logsCmd, true),
	})
}

// connectCmd initiates SSH or SSM connection to the selected instance.
func (e *EC2Instance) connectCmd(*tcell.EventKey) *tcell.EventKey {
	instanceID := e.GetSelectedItem()
	if instanceID == "" {
		return nil
	}

	e.mx.RLock()
	app := e.app
	factory := e.factory
	region := e.region
	e.mx.RUnlock()

	if app == nil || factory == nil {
		return nil
	}

	// Get region from model if available
	if model := e.GetModel(); model != nil {
		if ns := model.GetNamespace(); ns != "" && ns != "*" && ns != "all" {
			region = ns
		}
	}
	if region == "" {
		region = factory.Region()
	}
	if region == "" {
		region = aws.DefaultRegion
	}

	// Show connection method dialog
	// EC2IC (Instance Connect) is default - works without direct network access
	dialog := ui.NewDialog(app.Content, "connect-dialog")
	dialog.SetMessage("Connect to " + instanceID)
	dialog.SetButtons([]string{"EC2 Connect", "SSH", "SSM", "Cancel"})
	dialog.SetButtonHandler(func(idx int, label string) {
		switch idx {
		case 0: // EC2 Instance Connect (tunnels through AWS)
			e.connectEC2IC(instanceID, region)
		case 1: // Direct SSH (requires public IP + network access)
			e.connectSSH(instanceID, region)
		case 2: // SSM (requires SSM agent)
			e.connectSSM(instanceID, region)
		}
	})
	dialog.Show()

	return nil
}

// connectEC2IC starts an EC2 Instance Connect session.
// This tunnels through AWS and works without direct network access.
func (e *EC2Instance) connectEC2IC(instanceID, region string) {
	e.mx.RLock()
	app := e.app
	e.mx.RUnlock()

	if app == nil {
		return
	}

	app.Flash().Infof("Connecting via EC2 Instance Connect to %s...", instanceID)

	// Suspend TUI and run EC2 Instance Connect
	suspended := app.Suspend(func() {
		if err := aws.ExecEC2IC(instanceID, region); err != nil {
			// Error will be shown in terminal
		}
	})

	if !suspended {
		app.Flash().Errf("Failed to suspend application")
	}
}

// connectSSM starts an SSM session to the instance.
func (e *EC2Instance) connectSSM(instanceID, region string) {
	e.mx.RLock()
	app := e.app
	e.mx.RUnlock()

	if app == nil {
		return
	}

	app.Flash().Infof("Starting SSM session to %s...", instanceID)

	// Suspend TUI and run SSM session
	suspended := app.Suspend(func() {
		if err := aws.ExecSSM(instanceID, region); err != nil {
			// Error will be shown after resume
		}
	})

	if !suspended {
		app.Flash().Err(nil)
		app.Flash().Errf("Failed to suspend application for SSM session")
	}
}

// connectSSH starts an SSH session to the instance.
func (e *EC2Instance) connectSSH(instanceID, region string) {
	e.mx.RLock()
	app := e.app
	factory := e.factory
	e.mx.RUnlock()

	if app == nil || factory == nil {
		return
	}

	client := factory.Client()
	if client == nil {
		app.Flash().Errf("Failed to get AWS client")
		return
	}

	ec2Client := client.EC2(region)
	if ec2Client == nil {
		app.Flash().Errf("Failed to get EC2 client")
		return
	}

	// Get public IP
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	publicIP, err := aws.GetInstancePublicIP(ctx, ec2Client, instanceID)
	if err != nil {
		app.Flash().Errf("Cannot SSH: %v", err)
		return
	}

	// Detect SSH user
	sshUser := aws.DetectSSHUser(ctx, ec2Client, instanceID)

	app.Flash().Infof("Connecting via SSH to %s@%s...", sshUser, publicIP)

	// Build SSH config
	cfg := aws.DefaultSSHConfig()
	cfg.User = sshUser

	// Suspend TUI and run SSH
	suspended := app.Suspend(func() {
		if err := aws.ExecSSH(publicIP, cfg); err != nil {
			// Error will be shown after resume
		}
	})

	if !suspended {
		app.Flash().Errf("Failed to suspend application for SSH session")
	}
}

// logsCmd shows CloudWatch logs for the selected instance.
func (e *EC2Instance) logsCmd(*tcell.EventKey) *tcell.EventKey {
	path := e.GetSelectedItem()
	if path == "" {
		return nil
	}
	// TODO: Implement CloudWatch logs view
	return nil
}

// setupSSMCmd enables SSM access on the selected instance.
func (e *EC2Instance) setupSSMCmd(*tcell.EventKey) *tcell.EventKey {
	instanceID := e.GetSelectedItem()
	if instanceID == "" {
		return nil
	}

	e.mx.RLock()
	app := e.app
	factory := e.factory
	region := e.region
	e.mx.RUnlock()

	if app == nil || factory == nil {
		return nil
	}

	// Get region from model if available
	if model := e.GetModel(); model != nil {
		if ns := model.GetNamespace(); ns != "" && ns != "*" && ns != "all" {
			region = ns
		}
	}
	if region == "" {
		region = factory.Region()
	}
	if region == "" {
		region = aws.DefaultRegion
	}

	// Show confirmation dialog
	confirm := ui.NewConfirm(app.Content)
	confirm.SetMessage("Setup SSM access for " + instanceID + "?\n\nThis will attach an IAM role with SSM permissions.")
	confirm.SetOnConfirm(func() {
		e.doSetupSSM(instanceID, region)
	})
	confirm.Show()

	return nil
}

// doSetupSSM performs the SSM setup.
func (e *EC2Instance) doSetupSSM(instanceID, region string) {
	e.mx.RLock()
	app := e.app
	factory := e.factory
	e.mx.RUnlock()

	if app == nil || factory == nil {
		return
	}

	client := factory.Client()
	if client == nil {
		app.Flash().Errf("Failed to get AWS client")
		return
	}

	ec2Client := client.EC2(region)
	if ec2Client == nil {
		app.Flash().Errf("Failed to get EC2 client")
		return
	}

	iamClient := client.IAM()
	if iamClient == nil {
		app.Flash().Errf("Failed to get IAM client")
		return
	}

	app.Flash().Infof("Setting up SSM access for %s...", instanceID)

	// Run setup in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		result, err := aws.SetupSSMAccess(ctx, ec2Client, iamClient, instanceID)

		app.QueueUpdateDraw(func() {
			if err != nil {
				app.Flash().Errf("SSM setup failed: %v", err)
			} else {
				app.Flash().Infof("%s", result.Message)
			}
		})
	}()
}
