package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

// ResourceSchema holds parsed schema information for a CloudFormation resource type
type ResourceSchema struct {
	TypeName                        string
	ReadOnlyProperties              []string // property names (not full paths)
	CreateOnlyProperties            []string
	ConditionalCreateOnlyProperties []string
}

// cfSchemaJSON represents the subset of CloudFormation schema we need to parse
type cfSchemaJSON struct {
	ReadOnlyProperties              []string `json:"readOnlyProperties"`
	CreateOnlyProperties            []string `json:"createOnlyProperties"`
	ConditionalCreateOnlyProperties []string `json:"conditionalCreateOnlyProperties"`
}

// GetResourceSchema fetches the CloudFormation schema for a resource type
// and parses out the property classifications.
func GetResourceSchema(ctx context.Context, client *cloudformation.Client, typeName string) (*ResourceSchema, error) {
	output, err := client.DescribeType(ctx, &cloudformation.DescribeTypeInput{
		Type:     types.RegistryTypeResource,
		TypeName: aws.String(typeName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe type %s: %w", typeName, err)
	}

	if output.Schema == nil {
		return nil, fmt.Errorf("no schema available for type %s", typeName)
	}

	// Parse the schema JSON
	var schemaData cfSchemaJSON
	if err := json.Unmarshal([]byte(*output.Schema), &schemaData); err != nil {
		return nil, fmt.Errorf("failed to parse schema for %s: %w", typeName, err)
	}

	return &ResourceSchema{
		TypeName:                        typeName,
		ReadOnlyProperties:              extractPropertyNames(schemaData.ReadOnlyProperties),
		CreateOnlyProperties:            extractPropertyNames(schemaData.CreateOnlyProperties),
		ConditionalCreateOnlyProperties: extractPropertyNames(schemaData.ConditionalCreateOnlyProperties),
	}, nil
}

// extractPropertyNames converts JSON pointer paths to simple property names
// Example: "/properties/InstanceId" -> "InstanceId"
func extractPropertyNames(paths []string) []string {
	names := make([]string, 0, len(paths))
	for _, path := range paths {
		name := extractPropertyName(path)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// extractPropertyName extracts the property name from a JSON pointer path
// Example: "/properties/InstanceId" -> "InstanceId"
// Example: "/properties/VpcConfig/SecurityGroupIds" -> "VpcConfig"
func extractPropertyName(path string) string {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	// Split by slash
	parts := strings.Split(path, "/")

	// We expect paths like "properties/PropertyName" or "properties/PropertyName/SubPath"
	// Extract the top-level property name after "properties"
	if len(parts) >= 2 && parts[0] == "properties" {
		return parts[1]
	}

	return ""
}

// FilterEditableProperties takes a resource's properties and returns only the editable ones
// by removing read-only and create-only properties.
func FilterEditableProperties(props map[string]interface{}, schema *ResourceSchema) map[string]interface{} {
	if props == nil {
		return nil
	}

	// Build a set of non-editable properties
	nonEditable := make(map[string]bool)
	for _, prop := range schema.ReadOnlyProperties {
		nonEditable[prop] = true
	}
	for _, prop := range schema.CreateOnlyProperties {
		nonEditable[prop] = true
	}
	for _, prop := range schema.ConditionalCreateOnlyProperties {
		nonEditable[prop] = true
	}

	// Filter out non-editable properties
	editable := make(map[string]interface{})
	for key, value := range props {
		if !nonEditable[key] {
			editable[key] = value
		}
	}

	return editable
}
