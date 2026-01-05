package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func init() {
	RegisterAccessor(&EC2InstanceRID, &EC2Instance{})
}

// EC2Instance is the DAO for EC2 instances.
type EC2Instance struct {
	AWSResource
}

// List returns all EC2 instances in the specified region.
func (e *EC2Instance) List(ctx context.Context, region string) ([]AWSObject, error) {
	f := e.getFactory()
	if f == nil {
		return nil, fmt.Errorf("factory not initialized")
	}

	client := f.Client().EC2(region)
	if client == nil {
		return nil, fmt.Errorf("failed to get EC2 client for region %s", region)
	}

	input := &ec2.DescribeInstancesInput{}
	paginator := ec2.NewDescribeInstancesPaginator(client, input)

	var instances []AWSObject
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe instances: %w", err)
		}

		for _, reservation := range output.Reservations {
			for _, instance := range reservation.Instances {
				instances = append(instances, instanceToAWSObject(instance, region))
			}
		}
	}

	return instances, nil
}

// Get retrieves a single EC2 instance by path (format: "region/instance-id").
func (e *EC2Instance) Get(ctx context.Context, path string) (AWSObject, error) {
	region, instanceID, err := parseEC2Path(path)
	if err != nil {
		return nil, err
	}

	f := e.getFactory()
	if f == nil {
		return nil, fmt.Errorf("factory not initialized")
	}

	client := f.Client().EC2(region)
	if client == nil {
		return nil, fmt.Errorf("failed to get EC2 client for region %s", region)
	}

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	output, err := client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance %s: %w", instanceID, err)
	}

	if len(output.Reservations) == 0 || len(output.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("instance not found: %s", instanceID)
	}

	instance := output.Reservations[0].Instances[0]
	return instanceToAWSObject(instance, region), nil
}

// Describe returns a formatted description of the EC2 instance.
func (e *EC2Instance) Describe(path string) (string, error) {
	obj, err := e.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	instance, ok := obj.GetRaw().(types.Instance)
	if !ok {
		return "", fmt.Errorf("invalid instance object")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Instance ID: %s\n", obj.GetID()))
	sb.WriteString(fmt.Sprintf("Name: %s\n", obj.GetName()))
	sb.WriteString(fmt.Sprintf("State: %s\n", instance.State.Name))
	sb.WriteString(fmt.Sprintf("Type: %s\n", instance.InstanceType))
	sb.WriteString(fmt.Sprintf("Region: %s\n", obj.GetRegion()))

	if instance.Placement != nil && instance.Placement.AvailabilityZone != nil {
		sb.WriteString(fmt.Sprintf("Availability Zone: %s\n", *instance.Placement.AvailabilityZone))
	}

	if instance.PublicIpAddress != nil {
		sb.WriteString(fmt.Sprintf("Public IP: %s\n", *instance.PublicIpAddress))
	}

	if instance.PrivateIpAddress != nil {
		sb.WriteString(fmt.Sprintf("Private IP: %s\n", *instance.PrivateIpAddress))
	}

	if instance.VpcId != nil {
		sb.WriteString(fmt.Sprintf("VPC ID: %s\n", *instance.VpcId))
	}

	if instance.SubnetId != nil {
		sb.WriteString(fmt.Sprintf("Subnet ID: %s\n", *instance.SubnetId))
	}

	if obj.GetCreatedAt() != nil {
		sb.WriteString(fmt.Sprintf("Launch Time: %s\n", obj.GetCreatedAt().Format("2006-01-02 15:04:05")))
	}

	if len(obj.GetTags()) > 0 {
		sb.WriteString("Tags:\n")
		for k, v := range obj.GetTags() {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	return sb.String(), nil
}

// ToJSON returns a JSON representation of the EC2 instance.
func (e *EC2Instance) ToJSON(path string) (string, error) {
	obj, err := e.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(obj.GetRaw(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal instance to JSON: %w", err)
	}

	return string(data), nil
}

// Delete terminates an EC2 instance.
func (e *EC2Instance) Delete(ctx context.Context, path string, force bool) error {
	region, instanceID, err := parseEC2Path(path)
	if err != nil {
		return err
	}

	f := e.getFactory()
	if f == nil {
		return fmt.Errorf("factory not initialized")
	}

	client := f.Client().EC2(region)
	if client == nil {
		return fmt.Errorf("failed to get EC2 client for region %s", region)
	}

	input := &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	}

	_, err = client.TerminateInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to terminate instance %s: %w", instanceID, err)
	}

	return nil
}

// Start starts a stopped EC2 instance.
func (e *EC2Instance) Start(ctx context.Context, instanceID string) error {
	f := e.getFactory()
	if f == nil {
		return fmt.Errorf("factory not initialized")
	}

	region := f.Region()
	client := f.Client().EC2(region)
	if client == nil {
		return fmt.Errorf("failed to get EC2 client for region %s", region)
	}

	input := &ec2.StartInstancesInput{
		InstanceIds: []string{instanceID},
	}

	_, err := client.StartInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to start instance %s: %w", instanceID, err)
	}

	return nil
}

// Stop stops a running EC2 instance.
func (e *EC2Instance) Stop(ctx context.Context, instanceID string, force bool) error {
	f := e.getFactory()
	if f == nil {
		return fmt.Errorf("factory not initialized")
	}

	region := f.Region()
	client := f.Client().EC2(region)
	if client == nil {
		return fmt.Errorf("failed to get EC2 client for region %s", region)
	}

	input := &ec2.StopInstancesInput{
		InstanceIds: []string{instanceID},
		Force:       &force,
	}

	_, err := client.StopInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to stop instance %s: %w", instanceID, err)
	}

	return nil
}

// Reboot reboots an EC2 instance.
func (e *EC2Instance) Reboot(ctx context.Context, instanceID string) error {
	f := e.getFactory()
	if f == nil {
		return fmt.Errorf("factory not initialized")
	}

	region := f.Region()
	client := f.Client().EC2(region)
	if client == nil {
		return fmt.Errorf("failed to get EC2 client for region %s", region)
	}

	input := &ec2.RebootInstancesInput{
		InstanceIds: []string{instanceID},
	}

	_, err := client.RebootInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to reboot instance %s: %w", instanceID, err)
	}

	return nil
}

// GetConsoleOutput retrieves the console output for an EC2 instance.
func (e *EC2Instance) GetConsoleOutput(ctx context.Context, instanceID string) (string, error) {
	f := e.getFactory()
	if f == nil {
		return "", fmt.Errorf("factory not initialized")
	}

	region := f.Region()
	client := f.Client().EC2(region)
	if client == nil {
		return "", fmt.Errorf("failed to get EC2 client for region %s", region)
	}

	input := &ec2.GetConsoleOutputInput{
		InstanceId: &instanceID,
	}

	output, err := client.GetConsoleOutput(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to get console output for instance %s: %w", instanceID, err)
	}

	if output.Output == nil {
		return "", nil
	}

	return *output.Output, nil
}

// instanceToAWSObject converts an EC2 instance to an AWSObject.
func instanceToAWSObject(instance types.Instance, region string) AWSObject {
	tags := make(map[string]string)
	for _, tag := range instance.Tags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}

	var arn string
	if instance.InstanceId != nil {
		// ARN format: arn:aws:ec2:region:account-id:instance/instance-id
		// We don't have account ID here, so we'll construct a partial ARN
		arn = fmt.Sprintf("arn:aws:ec2:%s::instance/%s", region, *instance.InstanceId)
	}

	var id string
	if instance.InstanceId != nil {
		id = *instance.InstanceId
	}

	name := extractNameTag(instance.Tags)

	return &BaseAWSObject{
		ARN:       arn,
		ID:        id,
		Name:      name,
		Region:    region,
		Tags:      tags,
		CreatedAt: instance.LaunchTime,
		Raw:       instance,
	}
}

// extractNameTag extracts the "Name" tag value from a list of tags.
func extractNameTag(tags []types.Tag) string {
	for _, tag := range tags {
		if tag.Key != nil && *tag.Key == "Name" && tag.Value != nil {
			return *tag.Value
		}
	}
	return ""
}

// parseEC2Path parses a path in the format "region/instance-id".
func parseEC2Path(path string) (region, instanceID string, err error) {
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid path format, expected 'region/instance-id', got: %s", path)
	}

	region = strings.TrimSpace(parts[0])
	instanceID = strings.TrimSpace(parts[1])

	if region == "" || instanceID == "" {
		return "", "", fmt.Errorf("region and instance-id cannot be empty")
	}

	return region, instanceID, nil
}
