// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// StartInstance starts an EC2 instance.
func StartInstance(ctx context.Context, client *ec2.Client, instanceID string) error {
	_, err := client.StartInstances(ctx, &ec2.StartInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("failed to start instance %s: %w", instanceID, err)
	}
	return nil
}

// StopInstance stops an EC2 instance.
func StopInstance(ctx context.Context, client *ec2.Client, instanceID string) error {
	_, err := client.StopInstances(ctx, &ec2.StopInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("failed to stop instance %s: %w", instanceID, err)
	}
	return nil
}

// RebootInstance reboots an EC2 instance.
func RebootInstance(ctx context.Context, client *ec2.Client, instanceID string) error {
	_, err := client.RebootInstances(ctx, &ec2.RebootInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("failed to reboot instance %s: %w", instanceID, err)
	}
	return nil
}

// TerminateInstance terminates an EC2 instance.
func TerminateInstance(ctx context.Context, client *ec2.Client, instanceID string) error {
	_, err := client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("failed to terminate instance %s: %w", instanceID, err)
	}
	return nil
}
