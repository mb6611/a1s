// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package aws

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

const (
	// SSMRoleName is the default role name for SSM access
	SSMRoleName = "a1s-ssm-role"
	// SSMInstanceProfileName is the default instance profile name
	SSMInstanceProfileName = "a1s-ssm-instance-profile"
	// SSMManagedPolicyARN is the AWS managed policy for SSM
	SSMManagedPolicyARN = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
)

// SSMSetupResult contains the result of SSM setup
type SSMSetupResult struct {
	RoleCreated     bool
	ProfileCreated  bool
	PolicyAttached  bool
	ProfileAttached bool
	Message         string
}

// SetupSSMAccess enables SSM access on an EC2 instance by:
// 1. Checking if instance already has an instance profile
// 2. If yes: adds SSM policy to the existing role
// 3. If no: creates a new role + instance profile and attaches it
func SetupSSMAccess(ctx context.Context, ec2Client *ec2.Client, iamClient *iam.Client, instanceID string) (*SSMSetupResult, error) {
	result := &SSMSetupResult{}

	// Get instance details
	descOutput, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance: %w", err)
	}
	if len(descOutput.Reservations) == 0 || len(descOutput.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}

	instance := descOutput.Reservations[0].Instances[0]

	// Check if instance already has an instance profile
	if instance.IamInstanceProfile != nil && instance.IamInstanceProfile.Arn != nil {
		// Instance has a profile - add SSM policy to existing role
		profileArn := *instance.IamInstanceProfile.Arn
		return addSSMPolicyToExistingProfile(ctx, iamClient, profileArn, result)
	}

	// No instance profile - create new role and profile
	return createAndAttachSSMProfile(ctx, ec2Client, iamClient, instanceID, result)
}

// addSSMPolicyToExistingProfile adds the SSM managed policy to an existing instance profile's role
func addSSMPolicyToExistingProfile(ctx context.Context, iamClient *iam.Client, profileArn string, result *SSMSetupResult) (*SSMSetupResult, error) {
	// Extract profile name from ARN (format: arn:aws:iam::123456789012:instance-profile/profile-name)
	profileName := extractProfileName(profileArn)
	if profileName == "" {
		return nil, fmt.Errorf("failed to extract profile name from ARN: %s", profileArn)
	}

	// Get the instance profile to find the role
	profileOutput, err := iamClient.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{
		InstanceProfileName: aws.String(profileName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get instance profile: %w", err)
	}

	if len(profileOutput.InstanceProfile.Roles) == 0 {
		return nil, errors.New("instance profile has no associated role")
	}

	roleName := *profileOutput.InstanceProfile.Roles[0].RoleName

	// Check if policy is already attached
	policies, err := iamClient.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list role policies: %w", err)
	}

	for _, policy := range policies.AttachedPolicies {
		if policy.PolicyArn != nil && *policy.PolicyArn == SSMManagedPolicyARN {
			result.Message = "SSM policy already attached to role " + roleName
			return result, nil
		}
	}

	// Attach SSM policy to the role
	_, err = iamClient.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
		RoleName:  aws.String(roleName),
		PolicyArn: aws.String(SSMManagedPolicyARN),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach SSM policy: %w", err)
	}

	result.PolicyAttached = true
	result.Message = fmt.Sprintf("Added SSM policy to existing role %s. SSM will connect within ~1 minute.", roleName)
	return result, nil
}

// createAndAttachSSMProfile creates a new IAM role, instance profile, and attaches to instance
func createAndAttachSSMProfile(ctx context.Context, ec2Client *ec2.Client, iamClient *iam.Client, instanceID string, result *SSMSetupResult) (*SSMSetupResult, error) {
	// Trust policy for EC2
	trustPolicy := `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Service": "ec2.amazonaws.com"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}`

	// Create role (or get existing)
	_, err := iamClient.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 aws.String(SSMRoleName),
		AssumeRolePolicyDocument: aws.String(trustPolicy),
		Description:              aws.String("Role for SSM access created by a1s"),
	})
	if err != nil {
		// Check if role already exists
		var entityExists *iamtypes.EntityAlreadyExistsException
		if !errors.As(err, &entityExists) {
			return nil, fmt.Errorf("failed to create role: %w", err)
		}
	} else {
		result.RoleCreated = true
	}

	// Attach SSM policy to role
	_, err = iamClient.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
		RoleName:  aws.String(SSMRoleName),
		PolicyArn: aws.String(SSMManagedPolicyARN),
	})
	if err != nil {
		// Ignore if already attached
		var alreadyAttached *iamtypes.PolicyNotAttachableException
		if !errors.As(err, &alreadyAttached) {
			// Policy might already be attached, continue
		}
	}
	result.PolicyAttached = true

	// Create instance profile (or get existing)
	_, err = iamClient.CreateInstanceProfile(ctx, &iam.CreateInstanceProfileInput{
		InstanceProfileName: aws.String(SSMInstanceProfileName),
	})
	if err != nil {
		var entityExists *iamtypes.EntityAlreadyExistsException
		if !errors.As(err, &entityExists) {
			return nil, fmt.Errorf("failed to create instance profile: %w", err)
		}
	} else {
		result.ProfileCreated = true
	}

	// Add role to instance profile
	_, err = iamClient.AddRoleToInstanceProfile(ctx, &iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: aws.String(SSMInstanceProfileName),
		RoleName:            aws.String(SSMRoleName),
	})
	if err != nil {
		// Ignore if already added
		var limitExceeded *iamtypes.LimitExceededException
		if !errors.As(err, &limitExceeded) {
			// Role might already be in profile, continue
		}
	}

	// Wait a moment for IAM to propagate
	time.Sleep(2 * time.Second)

	// Associate instance profile with EC2 instance
	_, err = ec2Client.AssociateIamInstanceProfile(ctx, &ec2.AssociateIamInstanceProfileInput{
		InstanceId: aws.String(instanceID),
		IamInstanceProfile: &ec2types.IamInstanceProfileSpecification{
			Name: aws.String(SSMInstanceProfileName),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to associate instance profile: %w", err)
	}
	result.ProfileAttached = true

	result.Message = "Created and attached SSM role. SSM will connect within ~1 minute."
	return result, nil
}

// extractProfileName extracts the profile name from an instance profile ARN
func extractProfileName(arn string) string {
	// ARN format: arn:aws:iam::123456789012:instance-profile/profile-name
	// or: arn:aws:iam::123456789012:instance-profile/path/profile-name
	idx := len(arn) - 1
	for idx >= 0 && arn[idx] != '/' {
		idx--
	}
	if idx >= 0 {
		return arn[idx+1:]
	}
	return ""
}
