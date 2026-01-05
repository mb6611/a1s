// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package view

import (
	"context"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/ui"
	"github.com/derailed/tcell/v2"
)

// SecurityGroup represents a security group view with rule browsing.
type SecurityGroup struct {
	*Browser

	sgID     string
	ruleType string
}

// NewSecurityGroup returns a new security group view.
func NewSecurityGroup() *SecurityGroup {
	rid := &dao.ResourceID{
		Service:  "vpc",
		Resource: "securitygroup",
	}
	return &SecurityGroup{
		Browser:  NewBrowser(rid),
		ruleType: "inbound",
	}
}

// Init initializes the security group view.
func (s *SecurityGroup) Init(ctx context.Context) error {
	if err := s.Browser.Init(ctx); err != nil {
		return err
	}

	s.bindSGKeys(s.Actions())
	return nil
}

// Name returns the component name for breadcrumbs.
func (s *SecurityGroup) Name() string {
	return "security-group"
}

// SetSecurityGroupID sets the security group to view.
func (s *SecurityGroup) SetSecurityGroupID(sgID string) {
	s.sgID = sgID
}

// bindSGKeys sets up security group-specific key bindings.
func (s *SecurityGroup) bindSGKeys(aa *ui.KeyActions) {
	aa.Bulk(ui.KeyMap{
		tcell.KeyEnter: ui.NewKeyAction("View Rule Details", s.viewRuleDetails, true),
		ui.KeyI:        ui.NewKeyAction("Inbound Rules", s.inboundCmd, true),
		ui.KeyO:        ui.NewKeyAction("Outbound Rules", s.outboundCmd, true),
		ui.KeyA:        ui.NewKeyAction("Add Rule", s.addRuleCmd, true),
		ui.KeyE:        ui.NewKeyAction("Edit Rule", s.editRuleCmd, true),
		tcell.KeyCtrlD: ui.NewKeyActionWithOpts("Delete Rule", s.deleteRuleCmd, ui.ActionOpts{
			Visible:   true,
			Dangerous: true,
		}),
	})
}

// viewRuleDetails shows detailed information about the selected rule.
func (s *SecurityGroup) viewRuleDetails(*tcell.EventKey) *tcell.EventKey {
	path := s.GetSelectedItem()
	if path == "" {
		return nil
	}
	// TODO: Implement rule details view
	return nil
}

// inboundCmd switches to showing inbound rules.
func (s *SecurityGroup) inboundCmd(*tcell.EventKey) *tcell.EventKey {
	s.ruleType = "inbound"
	s.Start()
	return nil
}

// outboundCmd switches to showing outbound rules.
func (s *SecurityGroup) outboundCmd(*tcell.EventKey) *tcell.EventKey {
	s.ruleType = "outbound"
	s.Start()
	return nil
}

// addRuleCmd initiates adding a new security group rule.
func (s *SecurityGroup) addRuleCmd(*tcell.EventKey) *tcell.EventKey {
	// TODO: Implement add rule dialog
	return nil
}

// editRuleCmd initiates editing the selected security group rule.
func (s *SecurityGroup) editRuleCmd(*tcell.EventKey) *tcell.EventKey {
	path := s.GetSelectedItem()
	if path == "" {
		return nil
	}
	// TODO: Implement edit rule dialog
	return nil
}

// deleteRuleCmd initiates deletion of the selected security group rule.
func (s *SecurityGroup) deleteRuleCmd(*tcell.EventKey) *tcell.EventKey {
	path := s.GetSelectedItem()
	if path == "" {
		return nil
	}
	// TODO: Implement delete rule confirmation dialog
	return nil
}
