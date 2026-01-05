// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
)

// Cloud Control errors
var (
	ErrResourceNotSupported  = errors.New("resource type not supported by Cloud Control API")
	ErrGetResourceFailed     = errors.New("failed to get resource")
	ErrUpdateResourceFailed  = errors.New("failed to update resource")
)

// GetResourceState fetches the current state of a resource via Cloud Control API.
// Returns the resource properties as a map.
func GetResourceState(ctx context.Context, client *cloudcontrol.Client, typeName, identifier string) (map[string]interface{}, error) {
	if client == nil {
		return nil, errors.New("cloudcontrol client is nil")
	}

	input := &cloudcontrol.GetResourceInput{
		TypeName:   &typeName,
		Identifier: &identifier,
	}

	result, err := client.GetResource(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrGetResourceFailed, typeName, err)
	}

	if result.ResourceDescription == nil || result.ResourceDescription.Properties == nil {
		return nil, fmt.Errorf("%w: no properties returned for %s", ErrGetResourceFailed, typeName)
	}

	// Parse the JSON properties
	var props map[string]interface{}
	if err := json.Unmarshal([]byte(*result.ResourceDescription.Properties), &props); err != nil {
		return nil, fmt.Errorf("failed to parse resource properties: %w", err)
	}

	return props, nil
}

// UpdateResourceState updates a resource using a JSON Patch document.
// The patchDocument should be a RFC 6902 JSON Patch array.
func UpdateResourceState(ctx context.Context, client *cloudcontrol.Client, typeName, identifier, patchDocument string) error {
	if client == nil {
		return errors.New("cloudcontrol client is nil")
	}

	input := &cloudcontrol.UpdateResourceInput{
		TypeName:      &typeName,
		Identifier:    &identifier,
		PatchDocument: &patchDocument,
	}

	result, err := client.UpdateResource(ctx, input)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrUpdateResourceFailed, typeName, err)
	}

	// Check operation status
	if result.ProgressEvent != nil && result.ProgressEvent.ErrorCode != "" {
		return fmt.Errorf("%w: %s: %s", ErrUpdateResourceFailed,
			result.ProgressEvent.ErrorCode,
			safeString(result.ProgressEvent.StatusMessage))
	}

	return nil
}

// ExtractIdentifier extracts the Cloud Control identifier from a resource path.
// Different resources use different identifier formats:
// - EC2 instances: instance-id (e.g., "i-1234567890abcdef0")
// - S3 buckets: bucket-name
// - IAM roles: role-name
// - EKS clusters: cluster-name
// - VPC resources: resource-id
//
// Path format is typically "region/id" for regional resources or just "id" for global ones.
func ExtractIdentifier(resourceType, path string) string {
	// Strip region prefix if present (format: "us-east-1/i-123456")
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

// safeString safely dereferences a string pointer
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
