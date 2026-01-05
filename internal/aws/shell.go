// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package aws

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// SSHConfig holds SSH connection configuration.
type SSHConfig struct {
	User    string // SSH user (ec2-user, ubuntu, etc.)
	KeyPath string // Path to SSH private key
	Port    int    // SSH port (default 22)
}

// DefaultSSHConfig returns default SSH configuration.
func DefaultSSHConfig() *SSHConfig {
	return &SSHConfig{
		Port: 22,
	}
}

// GetInstancePublicIP retrieves the public IP of an EC2 instance.
func GetInstancePublicIP(ctx context.Context, client *ec2.Client, instanceID string) (string, error) {
	output, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe instance: %w", err)
	}

	if len(output.Reservations) == 0 || len(output.Reservations[0].Instances) == 0 {
		return "", fmt.Errorf("instance %s not found", instanceID)
	}

	instance := output.Reservations[0].Instances[0]
	if instance.PublicIpAddress == nil {
		return "", fmt.Errorf("instance %s has no public IP", instanceID)
	}

	return *instance.PublicIpAddress, nil
}

// GetInstancePrivateIP retrieves the private IP of an EC2 instance.
func GetInstancePrivateIP(ctx context.Context, client *ec2.Client, instanceID string) (string, error) {
	output, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe instance: %w", err)
	}

	if len(output.Reservations) == 0 || len(output.Reservations[0].Instances) == 0 {
		return "", fmt.Errorf("instance %s not found", instanceID)
	}

	instance := output.Reservations[0].Instances[0]
	if instance.PrivateIpAddress == nil {
		return "", fmt.Errorf("instance %s has no private IP", instanceID)
	}

	return *instance.PrivateIpAddress, nil
}

// DetectSSHUser attempts to detect the SSH username based on the instance's AMI.
// Common defaults:
// - Amazon Linux: ec2-user
// - Ubuntu: ubuntu
// - CentOS: centos
// - RHEL: ec2-user
// - Debian: admin
// - SUSE: ec2-user
func DetectSSHUser(ctx context.Context, client *ec2.Client, instanceID string) string {
	output, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil || len(output.Reservations) == 0 || len(output.Reservations[0].Instances) == 0 {
		return "ec2-user" // default
	}

	instance := output.Reservations[0].Instances[0]
	if instance.ImageId == nil {
		return "ec2-user"
	}

	// Try to get AMI details
	amiOutput, err := client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		ImageIds: []string{*instance.ImageId},
	})
	if err != nil || len(amiOutput.Images) == 0 {
		return "ec2-user"
	}

	image := amiOutput.Images[0]
	name := ""
	if image.Name != nil {
		name = strings.ToLower(*image.Name)
	}
	desc := ""
	if image.Description != nil {
		desc = strings.ToLower(*image.Description)
	}

	// Check for known patterns
	combined := name + " " + desc
	switch {
	case strings.Contains(combined, "ubuntu"):
		return "ubuntu"
	case strings.Contains(combined, "debian"):
		return "admin"
	case strings.Contains(combined, "centos"):
		return "centos"
	case strings.Contains(combined, "fedora"):
		return "fedora"
	case strings.Contains(combined, "suse"):
		return "ec2-user"
	default:
		return "ec2-user"
	}
}

// BuildSSHCommand builds the SSH command arguments.
func BuildSSHCommand(ip string, cfg *SSHConfig) []string {
	args := []string{}

	if cfg.KeyPath != "" {
		args = append(args, "-i", cfg.KeyPath)
	}

	if cfg.Port != 22 {
		args = append(args, "-p", fmt.Sprintf("%d", cfg.Port))
	}

	// Add common options
	args = append(args, "-o", "StrictHostKeyChecking=accept-new")

	// Build user@host
	target := ip
	if cfg.User != "" {
		target = cfg.User + "@" + ip
	}
	args = append(args, target)

	return args
}

// BuildSSMCommand builds the AWS SSM start-session command arguments.
func BuildSSMCommand(instanceID, region string) []string {
	args := []string{"ssm", "start-session", "--target", instanceID}
	if region != "" {
		args = append(args, "--region", region)
	}
	return args
}

// ExecSSH spawns an SSH session. This should be called with TUI suspended.
func ExecSSH(ip string, cfg *SSHConfig) error {
	args := BuildSSHCommand(ip, cfg)
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ExecSSM spawns an SSM session. This should be called with TUI suspended.
func ExecSSM(instanceID, region string) error {
	args := BuildSSMCommand(instanceID, region)
	cmd := exec.Command("aws", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// BuildEC2ICCommand builds the EC2 Instance Connect SSH command arguments.
// This works through AWS infrastructure without needing direct network access.
func BuildEC2ICCommand(instanceID, region string) []string {
	args := []string{"ec2-instance-connect", "ssh", "--instance-id", instanceID}
	if region != "" {
		args = append(args, "--region", region)
	}
	return args
}

// ExecEC2IC spawns an EC2 Instance Connect SSH session.
// This tunnels through AWS and works for private instances without public IPs.
// Requires: EC2 Instance Connect Endpoint in the VPC, or instance with public IP.
func ExecEC2IC(instanceID, region string) error {
	args := BuildEC2ICCommand(instanceID, region)
	cmd := exec.Command("aws", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
