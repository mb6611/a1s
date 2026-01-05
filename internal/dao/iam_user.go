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
	RegisterAccessor(&IAMUserRID, &IAMUser{})
}

// IAMUser is the DAO for IAM users.
type IAMUser struct {
	AWSResource
}

// List returns all IAM users (region is ignored as IAM is global).
func (i *IAMUser) List(ctx context.Context, region string) ([]AWSObject, error) {
	client := i.Client().IAM()
	if client == nil {
		return nil, fmt.Errorf("failed to get IAM client")
	}

	input := &iam.ListUsersInput{}
	paginator := iam.NewListUsersPaginator(client, input)

	var users []AWSObject
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, aws.WrapAWSError(err, "list users")
		}

		for _, user := range output.Users {
			users = append(users, userToAWSObject(user))
		}
	}

	return users, nil
}

// Get retrieves a single IAM user by path (path is the username).
func (i *IAMUser) Get(ctx context.Context, path string) (AWSObject, error) {
	username := parseUserPath(path)
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	client := i.Client().IAM()
	if client == nil {
		return nil, fmt.Errorf("failed to get IAM client")
	}

	input := &iam.GetUserInput{
		UserName: &username,
	}

	output, err := client.GetUser(ctx, input)
	if err != nil {
		return nil, aws.WrapAWSError(err, "get user")
	}

	if output.User == nil {
		return nil, fmt.Errorf("user not found: %s", username)
	}

	return userToAWSObject(*output.User), nil
}

// Describe returns a formatted description of the IAM user.
func (i *IAMUser) Describe(path string) (string, error) {
	obj, err := i.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	user, ok := obj.GetRaw().(types.User)
	if !ok {
		return "", fmt.Errorf("invalid user object")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("User Name: %s\n", obj.GetName()))
	sb.WriteString(fmt.Sprintf("User ID: %s\n", obj.GetID()))
	sb.WriteString(fmt.Sprintf("ARN: %s\n", obj.GetARN()))

	if user.Path != nil {
		sb.WriteString(fmt.Sprintf("Path: %s\n", *user.Path))
	}

	if obj.GetCreatedAt() != nil {
		sb.WriteString(fmt.Sprintf("Created: %s\n", obj.GetCreatedAt().Format("2006-01-02 15:04:05")))
	}

	if user.PasswordLastUsed != nil {
		sb.WriteString(fmt.Sprintf("Password Last Used: %s\n", user.PasswordLastUsed.Format("2006-01-02 15:04:05")))
	}

	if len(obj.GetTags()) > 0 {
		sb.WriteString("Tags:\n")
		for k, v := range obj.GetTags() {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	return sb.String(), nil
}

// ToJSON returns a JSON representation of the IAM user.
func (i *IAMUser) ToJSON(path string) (string, error) {
	obj, err := i.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(obj.GetRaw(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal user to JSON: %w", err)
	}

	return string(data), nil
}

// Delete deletes an IAM user. If force is true, cleans up all dependencies first.
func (i *IAMUser) Delete(ctx context.Context, path string, force bool) error {
	username := parseUserPath(path)
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	client := i.Client().IAM()
	if client == nil {
		return fmt.Errorf("failed to get IAM client")
	}

	if force {
		// Delete all access keys
		accessKeys, err := i.ListAccessKeys(ctx, username)
		if err != nil {
			return fmt.Errorf("failed to list access keys: %w", err)
		}
		for _, key := range accessKeys {
			if err := i.DeleteAccessKey(ctx, username, key.AccessKeyID); err != nil {
				return fmt.Errorf("failed to delete access key %s: %w", key.AccessKeyID, err)
			}
		}

		// Detach all attached policies
		policies, err := i.ListAttachedPolicies(ctx, username)
		if err != nil {
			return fmt.Errorf("failed to list attached policies: %w", err)
		}
		for _, policyArn := range policies {
			detachInput := &iam.DetachUserPolicyInput{
				UserName:  &username,
				PolicyArn: &policyArn,
			}
			if _, err := client.DetachUserPolicy(ctx, detachInput); err != nil {
				return aws.WrapAWSError(err, fmt.Sprintf("detach policy %s", policyArn))
			}
		}

		// Remove from all groups
		groups, err := i.ListGroups(ctx, username)
		if err != nil {
			return fmt.Errorf("failed to list groups: %w", err)
		}
		for _, groupName := range groups {
			removeInput := &iam.RemoveUserFromGroupInput{
				UserName:  &username,
				GroupName: &groupName,
			}
			if _, err := client.RemoveUserFromGroup(ctx, removeInput); err != nil {
				return aws.WrapAWSError(err, fmt.Sprintf("remove from group %s", groupName))
			}
		}

		// Delete inline policies
		listPoliciesInput := &iam.ListUserPoliciesInput{
			UserName: &username,
		}
		policiesOutput, err := client.ListUserPolicies(ctx, listPoliciesInput)
		if err != nil {
			return aws.WrapAWSError(err, "list inline policies")
		}
		for _, policyName := range policiesOutput.PolicyNames {
			deletePolicyInput := &iam.DeleteUserPolicyInput{
				UserName:   &username,
				PolicyName: &policyName,
			}
			if _, err := client.DeleteUserPolicy(ctx, deletePolicyInput); err != nil {
				return aws.WrapAWSError(err, fmt.Sprintf("delete inline policy %s", policyName))
			}
		}

		// Delete login profile if exists
		deleteLoginInput := &iam.DeleteLoginProfileInput{
			UserName: &username,
		}
		// Ignore error if login profile doesn't exist
		_, _ = client.DeleteLoginProfile(ctx, deleteLoginInput)

		// Delete MFA devices
		listMFAInput := &iam.ListMFADevicesInput{
			UserName: &username,
		}
		mfaOutput, err := client.ListMFADevices(ctx, listMFAInput)
		if err != nil {
			return aws.WrapAWSError(err, "list MFA devices")
		}
		for _, device := range mfaOutput.MFADevices {
			if device.SerialNumber != nil {
				deactivateInput := &iam.DeactivateMFADeviceInput{
					UserName:     &username,
					SerialNumber: device.SerialNumber,
				}
				if _, err := client.DeactivateMFADevice(ctx, deactivateInput); err != nil {
					return aws.WrapAWSError(err, fmt.Sprintf("deactivate MFA device %s", *device.SerialNumber))
				}
			}
		}

		// Delete SSH public keys
		listSSHInput := &iam.ListSSHPublicKeysInput{
			UserName: &username,
		}
		sshOutput, err := client.ListSSHPublicKeys(ctx, listSSHInput)
		if err != nil {
			return aws.WrapAWSError(err, "list SSH public keys")
		}
		for _, key := range sshOutput.SSHPublicKeys {
			if key.SSHPublicKeyId != nil {
				deleteSSHInput := &iam.DeleteSSHPublicKeyInput{
					UserName:       &username,
					SSHPublicKeyId: key.SSHPublicKeyId,
				}
				if _, err := client.DeleteSSHPublicKey(ctx, deleteSSHInput); err != nil {
					return aws.WrapAWSError(err, fmt.Sprintf("delete SSH key %s", *key.SSHPublicKeyId))
				}
			}
		}

		// Delete service-specific credentials
		listCredInput := &iam.ListServiceSpecificCredentialsInput{
			UserName: &username,
		}
		credOutput, err := client.ListServiceSpecificCredentials(ctx, listCredInput)
		if err != nil {
			return aws.WrapAWSError(err, "list service-specific credentials")
		}
		for _, cred := range credOutput.ServiceSpecificCredentials {
			if cred.ServiceSpecificCredentialId != nil {
				deleteCredInput := &iam.DeleteServiceSpecificCredentialInput{
					ServiceSpecificCredentialId: cred.ServiceSpecificCredentialId,
				}
				if _, err := client.DeleteServiceSpecificCredential(ctx, deleteCredInput); err != nil {
					return aws.WrapAWSError(err, fmt.Sprintf("delete credential %s", *cred.ServiceSpecificCredentialId))
				}
			}
		}

		// Delete signing certificates
		listCertInput := &iam.ListSigningCertificatesInput{
			UserName: &username,
		}
		certOutput, err := client.ListSigningCertificates(ctx, listCertInput)
		if err != nil {
			return aws.WrapAWSError(err, "list signing certificates")
		}
		for _, cert := range certOutput.Certificates {
			if cert.CertificateId != nil {
				deleteCertInput := &iam.DeleteSigningCertificateInput{
					UserName:      &username,
					CertificateId: cert.CertificateId,
				}
				if _, err := client.DeleteSigningCertificate(ctx, deleteCertInput); err != nil {
					return aws.WrapAWSError(err, fmt.Sprintf("delete certificate %s", *cert.CertificateId))
				}
			}
		}
	}

	// Finally delete the user
	deleteInput := &iam.DeleteUserInput{
		UserName: &username,
	}

	_, err := client.DeleteUser(ctx, deleteInput)
	if err != nil {
		return aws.WrapAWSError(err, "delete user")
	}

	return nil
}

// AccessKeyMetadata contains metadata about an access key.
type AccessKeyMetadata struct {
	AccessKeyID string
	Status      string
	CreateDate  string
}

// ListAccessKeys lists all access keys for a user.
func (i *IAMUser) ListAccessKeys(ctx context.Context, username string) ([]AccessKeyMetadata, error) {
	client := i.Client().IAM()
	if client == nil {
		return nil, fmt.Errorf("failed to get IAM client")
	}

	input := &iam.ListAccessKeysInput{
		UserName: &username,
	}

	output, err := client.ListAccessKeys(ctx, input)
	if err != nil {
		return nil, aws.WrapAWSError(err, "list access keys")
	}

	keys := make([]AccessKeyMetadata, 0, len(output.AccessKeyMetadata))
	for _, key := range output.AccessKeyMetadata {
		metadata := AccessKeyMetadata{}
		if key.AccessKeyId != nil {
			metadata.AccessKeyID = *key.AccessKeyId
		}
		metadata.Status = string(key.Status)
		if key.CreateDate != nil {
			metadata.CreateDate = key.CreateDate.Format("2006-01-02 15:04:05")
		}
		keys = append(keys, metadata)
	}

	return keys, nil
}

// AccessKey contains full access key information including the secret.
type AccessKey struct {
	AccessKeyID     string
	SecretAccessKey string
	Status          string
	CreateDate      string
}

// CreateAccessKey creates a new access key for a user.
func (i *IAMUser) CreateAccessKey(ctx context.Context, username string) (*AccessKey, error) {
	client := i.Client().IAM()
	if client == nil {
		return nil, fmt.Errorf("failed to get IAM client")
	}

	input := &iam.CreateAccessKeyInput{
		UserName: &username,
	}

	output, err := client.CreateAccessKey(ctx, input)
	if err != nil {
		return nil, aws.WrapAWSError(err, "create access key")
	}

	if output.AccessKey == nil {
		return nil, fmt.Errorf("no access key returned")
	}

	key := &AccessKey{}
	if output.AccessKey.AccessKeyId != nil {
		key.AccessKeyID = *output.AccessKey.AccessKeyId
	}
	if output.AccessKey.SecretAccessKey != nil {
		key.SecretAccessKey = *output.AccessKey.SecretAccessKey
	}
	key.Status = string(output.AccessKey.Status)
	if output.AccessKey.CreateDate != nil {
		key.CreateDate = output.AccessKey.CreateDate.Format("2006-01-02 15:04:05")
	}

	return key, nil
}

// DeleteAccessKey deletes an access key for a user.
func (i *IAMUser) DeleteAccessKey(ctx context.Context, username, accessKeyID string) error {
	client := i.Client().IAM()
	if client == nil {
		return fmt.Errorf("failed to get IAM client")
	}

	input := &iam.DeleteAccessKeyInput{
		UserName:    &username,
		AccessKeyId: &accessKeyID,
	}

	_, err := client.DeleteAccessKey(ctx, input)
	if err != nil {
		return aws.WrapAWSError(err, "delete access key")
	}

	return nil
}

// ListGroups lists all groups a user belongs to.
func (i *IAMUser) ListGroups(ctx context.Context, username string) ([]string, error) {
	client := i.Client().IAM()
	if client == nil {
		return nil, fmt.Errorf("failed to get IAM client")
	}

	input := &iam.ListGroupsForUserInput{
		UserName: &username,
	}

	output, err := client.ListGroupsForUser(ctx, input)
	if err != nil {
		return nil, aws.WrapAWSError(err, "list groups for user")
	}

	groups := make([]string, 0, len(output.Groups))
	for _, group := range output.Groups {
		if group.GroupName != nil {
			groups = append(groups, *group.GroupName)
		}
	}

	return groups, nil
}

// ListAttachedPolicies lists all managed policies attached to a user.
func (i *IAMUser) ListAttachedPolicies(ctx context.Context, username string) ([]string, error) {
	client := i.Client().IAM()
	if client == nil {
		return nil, fmt.Errorf("failed to get IAM client")
	}

	input := &iam.ListAttachedUserPoliciesInput{
		UserName: &username,
	}

	output, err := client.ListAttachedUserPolicies(ctx, input)
	if err != nil {
		return nil, aws.WrapAWSError(err, "list attached policies")
	}

	policies := make([]string, 0, len(output.AttachedPolicies))
	for _, policy := range output.AttachedPolicies {
		if policy.PolicyArn != nil {
			policies = append(policies, *policy.PolicyArn)
		}
	}

	return policies, nil
}

// userToAWSObject converts an IAM user to an AWSObject.
func userToAWSObject(user types.User) AWSObject {
	tags := make(map[string]string)
	for _, tag := range user.Tags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}

	var arn string
	if user.Arn != nil {
		arn = *user.Arn
	}

	var id string
	if user.UserId != nil {
		id = *user.UserId
	}

	var name string
	if user.UserName != nil {
		name = *user.UserName
	}

	return &BaseAWSObject{
		ARN:       arn,
		ID:        id,
		Name:      name,
		Region:    aws.DefaultRegion, // IAM is global
		Tags:      tags,
		CreatedAt: user.CreateDate,
		Raw:       user,
	}
}

// parseUserPath parses a path to extract the username.
// For IAM users, the path is simply the username.
func parseUserPath(path string) string {
	return strings.TrimSpace(path)
}
