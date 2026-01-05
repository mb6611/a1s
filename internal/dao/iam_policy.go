package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/a1s/a1s/internal/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

func init() {
	RegisterAccessor(&IAMPolicyRID, &IAMPolicy{})
}

// IAMPolicy is the DAO for IAM policies.
type IAMPolicy struct {
	AWSResource
}

// PolicyVersion represents a policy version.
type PolicyVersion struct {
	VersionID        string
	IsDefaultVersion bool
	CreateDate       string
}

// List returns all customer-managed IAM policies (excludes AWS-managed policies).
func (p *IAMPolicy) List(ctx context.Context, region string) ([]AWSObject, error) {
	client := p.Client().IAM()
	if client == nil {
		return nil, fmt.Errorf("failed to get IAM client")
	}

	input := &iam.ListPoliciesInput{
		Scope: types.PolicyScopeTypeLocal, // Customer managed only
	}
	paginator := iam.NewListPoliciesPaginator(client, input)

	var policies []AWSObject
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, aws.WrapAWSError(err, "list policies")
		}

		for _, policy := range output.Policies {
			policies = append(policies, policyToAWSObject(policy))
		}
	}

	return policies, nil
}

// Get retrieves a single IAM policy by ARN.
func (p *IAMPolicy) Get(ctx context.Context, path string) (AWSObject, error) {
	policyARN := parsePolicyPath(path)

	client := p.Client().IAM()
	if client == nil {
		return nil, fmt.Errorf("failed to get IAM client")
	}

	input := &iam.GetPolicyInput{
		PolicyArn: &policyARN,
	}

	output, err := client.GetPolicy(ctx, input)
	if err != nil {
		return nil, aws.WrapAWSError(err, "get policy")
	}

	if output.Policy == nil {
		return nil, fmt.Errorf("policy not found: %s", policyARN)
	}

	return policyToAWSObject(*output.Policy), nil
}

// Describe returns a formatted description of the IAM policy.
func (p *IAMPolicy) Describe(path string) (string, error) {
	obj, err := p.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	policy, ok := obj.GetRaw().(types.Policy)
	if !ok {
		return "", fmt.Errorf("invalid policy object")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Policy Name: %s\n", obj.GetName()))
	sb.WriteString(fmt.Sprintf("Policy ARN: %s\n", obj.GetARN()))
	sb.WriteString(fmt.Sprintf("Policy ID: %s\n", obj.GetID()))

	if policy.Path != nil {
		sb.WriteString(fmt.Sprintf("Path: %s\n", *policy.Path))
	}

	if policy.Description != nil {
		sb.WriteString(fmt.Sprintf("Description: %s\n", *policy.Description))
	}

	if policy.AttachmentCount != nil {
		sb.WriteString(fmt.Sprintf("Attachment Count: %d\n", *policy.AttachmentCount))
	}

	if policy.PermissionsBoundaryUsageCount != nil {
		sb.WriteString(fmt.Sprintf("Permissions Boundary Usage Count: %d\n", *policy.PermissionsBoundaryUsageCount))
	}

	sb.WriteString(fmt.Sprintf("Is Attachable: %t\n", policy.IsAttachable))

	if policy.DefaultVersionId != nil {
		sb.WriteString(fmt.Sprintf("Default Version ID: %s\n", *policy.DefaultVersionId))
	}

	if obj.GetCreatedAt() != nil {
		sb.WriteString(fmt.Sprintf("Create Date: %s\n", obj.GetCreatedAt().Format("2006-01-02 15:04:05")))
	}

	if policy.UpdateDate != nil {
		sb.WriteString(fmt.Sprintf("Update Date: %s\n", policy.UpdateDate.Format("2006-01-02 15:04:05")))
	}

	// Get policy document
	policyARN := obj.GetARN()
	doc, err := p.GetPolicyDocument(context.Background(), policyARN)
	if err == nil && doc != "" {
		sb.WriteString("\nPolicy Document:\n")
		sb.WriteString(doc)
		sb.WriteString("\n")
	}

	// Get attachments
	users, roles, groups, err := p.ListAttachments(context.Background(), policyARN)
	if err == nil {
		if len(users) > 0 {
			sb.WriteString("\nAttached Users:\n")
			for _, user := range users {
				sb.WriteString(fmt.Sprintf("  - %s\n", user))
			}
		}
		if len(roles) > 0 {
			sb.WriteString("\nAttached Roles:\n")
			for _, role := range roles {
				sb.WriteString(fmt.Sprintf("  - %s\n", role))
			}
		}
		if len(groups) > 0 {
			sb.WriteString("\nAttached Groups:\n")
			for _, group := range groups {
				sb.WriteString(fmt.Sprintf("  - %s\n", group))
			}
		}
	}

	return sb.String(), nil
}

// ToJSON returns a JSON representation of the IAM policy.
func (p *IAMPolicy) ToJSON(path string) (string, error) {
	obj, err := p.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(obj.GetRaw(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal policy to JSON: %w", err)
	}

	return string(data), nil
}

// Delete deletes an IAM policy.
// If force is true, detaches from all users/roles/groups and deletes all non-default versions first.
func (p *IAMPolicy) Delete(ctx context.Context, path string, force bool) error {
	policyARN := parsePolicyPath(path)

	client := p.Client().IAM()
	if client == nil {
		return fmt.Errorf("failed to get IAM client")
	}

	if force {
		// Detach from all entities
		users, roles, groups, err := p.ListAttachments(ctx, policyARN)
		if err != nil {
			return fmt.Errorf("failed to list attachments: %w", err)
		}

		// Detach from users
		for _, userName := range users {
			_, err := client.DetachUserPolicy(ctx, &iam.DetachUserPolicyInput{
				UserName:  &userName,
				PolicyArn: &policyARN,
			})
			if err != nil {
				return aws.WrapAWSError(err, fmt.Sprintf("detach policy from user %s", userName))
			}
		}

		// Detach from roles
		for _, roleName := range roles {
			_, err := client.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
				RoleName:  &roleName,
				PolicyArn: &policyARN,
			})
			if err != nil {
				return aws.WrapAWSError(err, fmt.Sprintf("detach policy from role %s", roleName))
			}
		}

		// Detach from groups
		for _, groupName := range groups {
			_, err := client.DetachGroupPolicy(ctx, &iam.DetachGroupPolicyInput{
				GroupName: &groupName,
				PolicyArn: &policyARN,
			})
			if err != nil {
				return aws.WrapAWSError(err, fmt.Sprintf("detach policy from group %s", groupName))
			}
		}

		// Delete all non-default versions
		versions, err := p.ListVersions(ctx, policyARN)
		if err != nil {
			return fmt.Errorf("failed to list policy versions: %w", err)
		}

		for _, version := range versions {
			if !version.IsDefaultVersion {
				_, err := client.DeletePolicyVersion(ctx, &iam.DeletePolicyVersionInput{
					PolicyArn: &policyARN,
					VersionId: &version.VersionID,
				})
				if err != nil {
					return aws.WrapAWSError(err, fmt.Sprintf("delete policy version %s", version.VersionID))
				}
			}
		}
	}

	// Delete the policy
	_, err := client.DeletePolicy(ctx, &iam.DeletePolicyInput{
		PolicyArn: &policyARN,
	})
	if err != nil {
		return aws.WrapAWSError(err, "delete policy")
	}

	return nil
}

// GetPolicyDocument retrieves the policy document for the default version.
// Returns the URL-decoded JSON document.
func (p *IAMPolicy) GetPolicyDocument(ctx context.Context, policyARN string) (string, error) {
	client := p.Client().IAM()
	if client == nil {
		return "", fmt.Errorf("failed to get IAM client")
	}

	// First get the policy to find the default version
	policyOutput, err := client.GetPolicy(ctx, &iam.GetPolicyInput{
		PolicyArn: &policyARN,
	})
	if err != nil {
		return "", aws.WrapAWSError(err, "get policy")
	}

	if policyOutput.Policy == nil || policyOutput.Policy.DefaultVersionId == nil {
		return "", fmt.Errorf("policy or default version not found")
	}

	versionID := *policyOutput.Policy.DefaultVersionId

	// Get the policy version document
	versionOutput, err := client.GetPolicyVersion(ctx, &iam.GetPolicyVersionInput{
		PolicyArn: &policyARN,
		VersionId: &versionID,
	})
	if err != nil {
		return "", aws.WrapAWSError(err, "get policy version")
	}

	if versionOutput.PolicyVersion == nil || versionOutput.PolicyVersion.Document == nil {
		return "", fmt.Errorf("policy document not found")
	}

	// URL decode the document
	decoded := urlDecode(*versionOutput.PolicyVersion.Document)
	return decoded, nil
}

// ListVersions returns all versions of the policy.
func (p *IAMPolicy) ListVersions(ctx context.Context, policyARN string) ([]PolicyVersion, error) {
	client := p.Client().IAM()
	if client == nil {
		return nil, fmt.Errorf("failed to get IAM client")
	}

	input := &iam.ListPolicyVersionsInput{
		PolicyArn: &policyARN,
	}
	paginator := iam.NewListPolicyVersionsPaginator(client, input)

	var versions []PolicyVersion
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, aws.WrapAWSError(err, "list policy versions")
		}

		for _, version := range output.Versions {
			pv := PolicyVersion{}
			if version.VersionId != nil {
				pv.VersionID = *version.VersionId
			}
			pv.IsDefaultVersion = version.IsDefaultVersion
			if version.CreateDate != nil {
				pv.CreateDate = version.CreateDate.Format("2006-01-02 15:04:05")
			}
			versions = append(versions, pv)
		}
	}

	return versions, nil
}

// ListAttachments returns all entities (users, roles, groups) that have this policy attached.
func (p *IAMPolicy) ListAttachments(ctx context.Context, policyARN string) (users []string, roles []string, groups []string, err error) {
	client := p.Client().IAM()
	if client == nil {
		return nil, nil, nil, fmt.Errorf("failed to get IAM client")
	}

	input := &iam.ListEntitiesForPolicyInput{
		PolicyArn: &policyARN,
	}
	paginator := iam.NewListEntitiesForPolicyPaginator(client, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, nil, nil, aws.WrapAWSError(err, "list entities for policy")
		}

		for _, user := range output.PolicyUsers {
			if user.UserName != nil {
				users = append(users, *user.UserName)
			}
		}

		for _, role := range output.PolicyRoles {
			if role.RoleName != nil {
				roles = append(roles, *role.RoleName)
			}
		}

		for _, group := range output.PolicyGroups {
			if group.GroupName != nil {
				groups = append(groups, *group.GroupName)
			}
		}
	}

	return users, roles, groups, nil
}

// policyToAWSObject converts an IAM policy to an AWSObject.
func policyToAWSObject(policy types.Policy) AWSObject {
	var arn string
	if policy.Arn != nil {
		arn = *policy.Arn
	}

	var id string
	if policy.PolicyId != nil {
		id = *policy.PolicyId
	}

	var name string
	if policy.PolicyName != nil {
		name = *policy.PolicyName
	}

	return &BaseAWSObject{
		ARN:       arn,
		ID:        id,
		Name:      name,
		Region:    "global", // IAM is a global service
		Tags:      make(map[string]string),
		CreatedAt: policy.CreateDate,
		Raw:       policy,
	}
}

// parsePolicyPath extracts the policy ARN from the path.
// The path is expected to be the ARN itself.
func parsePolicyPath(path string) string {
	return strings.TrimSpace(path)
}

