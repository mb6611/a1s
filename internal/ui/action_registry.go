// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package ui

import (
	"context"

	"github.com/a1s/a1s/internal/aws"
	"github.com/a1s/a1s/internal/dao"
	"github.com/derailed/tcell/v2"
)

// ResourceAction represents an action that can be performed on a resource.
type ResourceAction struct {
	Key         tcell.Key                                                                        // Key binding
	Name        string                                                                           // Display name
	Description string                                                                           // Short description
	Dangerous   bool                                                                             // Requires confirmation
	Handler     func(ctx context.Context, client aws.Connection, region, identifier string) error
}

// ActionRegistry maps resource types to their available actions.
var ActionRegistry = map[string][]ResourceAction{}

// RegisterActions registers actions for a resource type.
func RegisterActions(resourceType string, actions []ResourceAction) {
	ActionRegistry[resourceType] = actions
}

// GetActions returns available actions for a resource type.
func GetActions(rid *dao.ResourceID) []ResourceAction {
	if rid == nil {
		return nil
	}
	return ActionRegistry[rid.String()]
}

// GetAction returns a specific action by key for a resource type.
func GetAction(rid *dao.ResourceID, key tcell.Key) *ResourceAction {
	actions := GetActions(rid)
	for i := range actions {
		if actions[i].Key == key {
			return &actions[i]
		}
	}
	return nil
}
