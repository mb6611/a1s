package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

func init() {
	RegisterAccessor(&IAMRoleRID, &IAMRole{})
}

// IAMRole implements the DAO for AWS IAM Roles.
type IAMRole struct {
	AWSResource
}

// List retrieves all IAM roles. IAM is global so region parameter is ignored.
func (r *IAMRole) List(ctx context.Context, region string) ([]AWSObject, error) {
	f := r.getFactory()
	if f == nil {
		return nil, fmt.Errorf("factory not initialized")
	}

	iamClient := f.Client().IAM()
	if iamClient == nil {
		return nil, fmt.Errorf("failed to get IAM client")
	}

	paginator := iam.NewListRolesPaginator(iamClient, &iam.ListRolesInput{})

	objects := make([]AWSObject, 0)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list IAM roles: %w", err)
		}

		for _, role := range page.Roles {
			obj := roleToAWSObject(role)
			objects = append(objects, obj)
		}
	}

	return objects, nil
}

// Get retrieves a single IAM role by path (role name).
func (r *IAMRole) Get(ctx context.Context, path string) (AWSObject, error) {
	roleName := parseRolePath(path)

	f := r.getFactory()
	if f == nil {
		return nil, fmt.Errorf("factory not initialized")
	}

	iamClient := f.Client().IAM()
	if iamClient == nil {
		return nil, fmt.Errorf("failed to get IAM client")
	}

	input := &iam.GetRoleInput{
		RoleName: &roleName,
	}

	result, err := iamClient.GetRole(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get IAM role %s: %w", roleName, err)
	}

	if result.Role == nil {
		return nil, fmt.Errorf("IAM role %s not found", roleName)
	}

	return roleToAWSObject(*result.Role), nil
}

// Describe returns a formatted description of an IAM role.
func (r *IAMRole) Describe(path string) (string, error) {
	obj, err := r.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	role := obj.GetRaw().(types.Role)
	roleName := parseRolePath(path)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Role Name: %s\n", obj.GetName()))
	b.WriteString(fmt.Sprintf("Role ID:   %s\n", obj.GetID()))
	b.WriteString(fmt.Sprintf("ARN:       %s\n", obj.GetARN()))

	if role.Path != nil {
		b.WriteString(fmt.Sprintf("Path:      %s\n", *role.Path))
	}

	if obj.GetCreatedAt() != nil {
		b.WriteString(fmt.Sprintf("Created:   %s\n", obj.GetCreatedAt().Format("2006-01-02 15:04:05")))
	}

	if role.Description != nil && *role.Description != "" {
		b.WriteString(fmt.Sprintf("Description: %s\n", *role.Description))
	}

	// Get trust policy
	trustPolicy, err := r.GetTrustPolicy(context.Background(), roleName)
	if err == nil && trustPolicy != "" {
		b.WriteString(fmt.Sprintf("\nTrust Policy:\n%s\n", trustPolicy))
	}

	// Get attached policies
	attachedPolicies, err := r.ListAttachedPolicies(context.Background(), roleName)
	if err == nil && len(attachedPolicies) > 0 {
		b.WriteString("\nAttached Policies:\n")
		for _, policy := range attachedPolicies {
			b.WriteString(fmt.Sprintf("  - %s\n", policy))
		}
	}

	// Get inline policies
	inlinePolicies, err := r.ListInlinePolicies(context.Background(), roleName)
	if err == nil && len(inlinePolicies) > 0 {
		b.WriteString("\nInline Policies:\n")
		for _, policy := range inlinePolicies {
			b.WriteString(fmt.Sprintf("  - %s\n", policy))
		}
	}

	if len(obj.GetTags()) > 0 {
		b.WriteString("\nTags:\n")
		for k, v := range obj.GetTags() {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	return b.String(), nil
}

// ToJSON returns a JSON representation of an IAM role.
func (r *IAMRole) ToJSON(path string) (string, error) {
	obj, err := r.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(obj.GetRaw(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal IAM role to JSON: %w", err)
	}

	return string(data), nil
}

// Delete deletes an IAM role. If force is true, detaches all policies,
// deletes inline policies, and removes from instance profiles first.
func (r *IAMRole) Delete(ctx context.Context, path string, force bool) error {
	roleName := parseRolePath(path)

	f := r.getFactory()
	if f == nil {
		return fmt.Errorf("factory not initialized")
	}

	iamClient := f.Client().IAM()
	if iamClient == nil {
		return fmt.Errorf("failed to get IAM client")
	}

	if force {
		// Detach all managed policies
		attachedPolicies, err := r.ListAttachedPolicies(ctx, roleName)
		if err != nil {
			return fmt.Errorf("failed to list attached policies: %w", err)
		}

		for _, policyArn := range attachedPolicies {
			_, err := iamClient.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
				RoleName:  &roleName,
				PolicyArn: &policyArn,
			})
			if err != nil {
				return fmt.Errorf("failed to detach policy %s: %w", policyArn, err)
			}
		}

		// Delete all inline policies
		inlinePolicies, err := r.ListInlinePolicies(ctx, roleName)
		if err != nil {
			return fmt.Errorf("failed to list inline policies: %w", err)
		}

		for _, policyName := range inlinePolicies {
			_, err := iamClient.DeleteRolePolicy(ctx, &iam.DeleteRolePolicyInput{
				RoleName:   &roleName,
				PolicyName: &policyName,
			})
			if err != nil {
				return fmt.Errorf("failed to delete inline policy %s: %w", policyName, err)
			}
		}

		// Remove from all instance profiles
		listProfilesResult, err := iamClient.ListInstanceProfilesForRole(ctx, &iam.ListInstanceProfilesForRoleInput{
			RoleName: &roleName,
		})
		if err != nil {
			return fmt.Errorf("failed to list instance profiles for role: %w", err)
		}

		if listProfilesResult.InstanceProfiles != nil {
			for _, profile := range listProfilesResult.InstanceProfiles {
				if profile.InstanceProfileName != nil {
					_, err := iamClient.RemoveRoleFromInstanceProfile(ctx, &iam.RemoveRoleFromInstanceProfileInput{
						InstanceProfileName: profile.InstanceProfileName,
						RoleName:            &roleName,
					})
					if err != nil {
						return fmt.Errorf("failed to remove role from instance profile %s: %w", *profile.InstanceProfileName, err)
					}
				}
			}
		}
	}

	// Delete the role
	_, err := iamClient.DeleteRole(ctx, &iam.DeleteRoleInput{
		RoleName: &roleName,
	})
	if err != nil {
		return fmt.Errorf("failed to delete IAM role %s: %w", roleName, err)
	}

	return nil
}

// GetTrustPolicy returns the AssumeRolePolicyDocument (URL decoded).
func (r *IAMRole) GetTrustPolicy(ctx context.Context, roleName string) (string, error) {
	f := r.getFactory()
	if f == nil {
		return "", fmt.Errorf("factory not initialized")
	}

	iamClient := f.Client().IAM()
	if iamClient == nil {
		return "", fmt.Errorf("failed to get IAM client")
	}

	result, err := iamClient.GetRole(ctx, &iam.GetRoleInput{
		RoleName: &roleName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get IAM role %s: %w", roleName, err)
	}

	if result.Role == nil || result.Role.AssumeRolePolicyDocument == nil {
		return "", fmt.Errorf("no trust policy found for role %s", roleName)
	}

	return urlDecode(*result.Role.AssumeRolePolicyDocument), nil
}

// ListAttachedPolicies returns a list of attached managed policy ARNs.
func (r *IAMRole) ListAttachedPolicies(ctx context.Context, roleName string) ([]string, error) {
	f := r.getFactory()
	if f == nil {
		return nil, fmt.Errorf("factory not initialized")
	}

	iamClient := f.Client().IAM()
	if iamClient == nil {
		return nil, fmt.Errorf("failed to get IAM client")
	}

	result, err := iamClient.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
		RoleName: &roleName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list attached policies for role %s: %w", roleName, err)
	}

	policies := make([]string, 0, len(result.AttachedPolicies))
	for _, policy := range result.AttachedPolicies {
		if policy.PolicyArn != nil {
			policies = append(policies, *policy.PolicyArn)
		}
	}

	return policies, nil
}

// ListInlinePolicies returns a list of inline policy names.
func (r *IAMRole) ListInlinePolicies(ctx context.Context, roleName string) ([]string, error) {
	f := r.getFactory()
	if f == nil {
		return nil, fmt.Errorf("factory not initialized")
	}

	iamClient := f.Client().IAM()
	if iamClient == nil {
		return nil, fmt.Errorf("failed to get IAM client")
	}

	result, err := iamClient.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{
		RoleName: &roleName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list inline policies for role %s: %w", roleName, err)
	}

	return result.PolicyNames, nil
}

// roleToAWSObject converts an IAM Role to an AWSObject.
func roleToAWSObject(role types.Role) AWSObject {
	tags := make(map[string]string)
	for _, tag := range role.Tags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}

	roleName := ""
	if role.RoleName != nil {
		roleName = *role.RoleName
	}

	roleID := ""
	if role.RoleId != nil {
		roleID = *role.RoleId
	}

	arn := ""
	if role.Arn != nil {
		arn = *role.Arn
	}

	return &BaseAWSObject{
		ARN:       arn,
		ID:        roleID,
		Name:      roleName,
		Region:    "global", // IAM is global
		Tags:      tags,
		CreatedAt: role.CreateDate,
		Raw:       role,
	}
}

// parseRolePath parses a role path (just the role name).
func parseRolePath(path string) string {
	// Path is just the role name
	return strings.TrimSpace(path)
}

// urlDecode decodes a URL-encoded string (for trust policy documents).
func urlDecode(s string) string {
	decoded, err := url.QueryUnescape(s)
	if err != nil {
		return s // Return original if decoding fails
	}
	return decoded
}
