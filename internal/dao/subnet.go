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
	RegisterAccessor(&SubnetRID, &Subnet{})
}

// Subnet implements the DAO for AWS Subnets.
type Subnet struct {
	AWSResource
}

// List retrieves all subnets in the specified region.
func (s *Subnet) List(ctx context.Context, region string) ([]AWSObject, error) {
	client := s.Client().EC2(region)
	if client == nil {
		return nil, fmt.Errorf("failed to get EC2 client for region %s", region)
	}

	input := &ec2.DescribeSubnetsInput{}
	result, err := client.DescribeSubnets(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe subnets: %w", err)
	}

	objects := make([]AWSObject, 0, len(result.Subnets))
	for _, subnet := range result.Subnets {
		obj := subnetToAWSObject(subnet, region)
		objects = append(objects, obj)
	}

	return objects, nil
}

// Get retrieves a single subnet by path (region/subnet-id).
func (s *Subnet) Get(ctx context.Context, path string) (AWSObject, error) {
	region, subnetID, err := parseSubnetPath(path)
	if err != nil {
		return nil, err
	}

	client := s.Client().EC2(region)
	if client == nil {
		return nil, fmt.Errorf("failed to get EC2 client for region %s", region)
	}

	input := &ec2.DescribeSubnetsInput{
		SubnetIds: []string{subnetID},
	}

	result, err := client.DescribeSubnets(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe subnet %s: %w", subnetID, err)
	}

	if len(result.Subnets) == 0 {
		return nil, fmt.Errorf("subnet %s not found in region %s", subnetID, region)
	}

	return subnetToAWSObject(result.Subnets[0], region), nil
}

// Describe returns a formatted description of a subnet.
func (s *Subnet) Describe(path string) (string, error) {
	obj, err := s.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	subnet := obj.GetRaw().(types.Subnet)
	az := getAvailabilityZone(subnet)
	availableIPs := getAvailableIPs(subnet)

	vpcID := ""
	if subnet.VpcId != nil {
		vpcID = *subnet.VpcId
	}

	cidr := ""
	if subnet.CidrBlock != nil {
		cidr = *subnet.CidrBlock
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Subnet ID:     %s\n", obj.GetID()))
	b.WriteString(fmt.Sprintf("Name:          %s\n", obj.GetName()))
	b.WriteString(fmt.Sprintf("Region:        %s\n", obj.GetRegion()))
	b.WriteString(fmt.Sprintf("VPC ID:        %s\n", vpcID))
	b.WriteString(fmt.Sprintf("CIDR Block:    %s\n", cidr))
	b.WriteString(fmt.Sprintf("AZ:            %s\n", az))
	b.WriteString(fmt.Sprintf("State:         %s\n", subnet.State))
	b.WriteString(fmt.Sprintf("Available IPs: %d\n", availableIPs))

	if len(obj.GetTags()) > 0 {
		b.WriteString("\nTags:\n")
		for k, v := range obj.GetTags() {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	return b.String(), nil
}

// ToJSON returns a JSON representation of a subnet.
func (s *Subnet) ToJSON(path string) (string, error) {
	obj, err := s.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(obj.GetRaw(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal subnet to JSON: %w", err)
	}

	return string(data), nil
}

// Delete deletes a subnet by path.
func (s *Subnet) Delete(ctx context.Context, path string, force bool) error {
	region, subnetID, err := parseSubnetPath(path)
	if err != nil {
		return err
	}

	client := s.Client().EC2(region)
	if client == nil {
		return fmt.Errorf("failed to get EC2 client for region %s", region)
	}

	input := &ec2.DeleteSubnetInput{
		SubnetId: &subnetID,
	}

	_, err = client.DeleteSubnet(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete subnet %s: %w", subnetID, err)
	}

	return nil
}

// subnetToAWSObject converts an EC2 Subnet to an AWSObject.
func subnetToAWSObject(subnet types.Subnet, region string) AWSObject {
	tags := make(map[string]string)
	name := ""

	for _, tag := range subnet.Tags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
			if *tag.Key == "Name" {
				name = *tag.Value
			}
		}
	}

	subnetID := ""
	if subnet.SubnetId != nil {
		subnetID = *subnet.SubnetId
	}

	arn := fmt.Sprintf("arn:aws:ec2:%s::subnet/%s", region, subnetID)

	return &BaseAWSObject{
		ARN:    arn,
		ID:     subnetID,
		Name:   name,
		Region: region,
		Tags:   tags,
		Raw:    subnet,
	}
}

// parseSubnetPath parses a subnet path in the format "region/subnet-id".
func parseSubnetPath(path string) (region, subnetID string, err error) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid subnet path format: expected 'region/subnet-id', got '%s'", path)
	}

	region = parts[0]
	subnetID = parts[1]

	if region == "" {
		return "", "", fmt.Errorf("region cannot be empty in path: %s", path)
	}

	if subnetID == "" {
		return "", "", fmt.Errorf("subnet ID cannot be empty in path: %s", path)
	}

	if !strings.HasPrefix(subnetID, "subnet-") {
		return "", "", fmt.Errorf("invalid subnet ID format: %s (expected subnet-*)", subnetID)
	}

	return region, subnetID, nil
}

// getAvailabilityZone returns the availability zone of a subnet.
func getAvailabilityZone(subnet types.Subnet) string {
	if subnet.AvailabilityZone != nil {
		return *subnet.AvailabilityZone
	}
	return ""
}

// getAvailableIPs returns the number of available IP addresses in a subnet.
func getAvailableIPs(subnet types.Subnet) int32 {
	if subnet.AvailableIpAddressCount != nil {
		return *subnet.AvailableIpAddressCount
	}
	return 0
}
