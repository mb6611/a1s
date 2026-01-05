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
	RegisterAccessor(&VPCResourceRID, &VPC{})
}

// VPC implements the DAO for AWS VPCs.
type VPC struct {
	AWSResource
}

// List retrieves all VPCs in the specified region.
func (v *VPC) List(ctx context.Context, region string) ([]AWSObject, error) {
	client := v.Client().EC2(region)
	if client == nil {
		return nil, fmt.Errorf("failed to get EC2 client for region %s", region)
	}

	input := &ec2.DescribeVpcsInput{}
	result, err := client.DescribeVpcs(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe VPCs: %w", err)
	}

	objects := make([]AWSObject, 0, len(result.Vpcs))
	for _, vpc := range result.Vpcs {
		obj := vpcToAWSObject(vpc, region)
		objects = append(objects, obj)
	}

	return objects, nil
}

// Get retrieves a single VPC by path (region/vpc-id).
func (v *VPC) Get(ctx context.Context, path string) (AWSObject, error) {
	region, vpcID, err := parseVPCPath(path)
	if err != nil {
		return nil, err
	}

	client := v.Client().EC2(region)
	if client == nil {
		return nil, fmt.Errorf("failed to get EC2 client for region %s", region)
	}

	input := &ec2.DescribeVpcsInput{
		VpcIds: []string{vpcID},
	}

	result, err := client.DescribeVpcs(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe VPC %s: %w", vpcID, err)
	}

	if len(result.Vpcs) == 0 {
		return nil, fmt.Errorf("VPC %s not found in region %s", vpcID, region)
	}

	return vpcToAWSObject(result.Vpcs[0], region), nil
}

// Describe returns a formatted description of a VPC.
func (v *VPC) Describe(path string) (string, error) {
	obj, err := v.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	vpc := obj.GetRaw().(types.Vpc)
	isDefault := "false"
	if isDefaultVPC(vpc) {
		isDefault = "true"
	}

	cidr := getVPCCIDR(vpc)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("VPC ID:    %s\n", obj.GetID()))
	b.WriteString(fmt.Sprintf("Name:      %s\n", obj.GetName()))
	b.WriteString(fmt.Sprintf("Region:    %s\n", obj.GetRegion()))
	b.WriteString(fmt.Sprintf("CIDR:      %s\n", cidr))
	b.WriteString(fmt.Sprintf("State:     %s\n", vpc.State))
	b.WriteString(fmt.Sprintf("Default:   %s\n", isDefault))

	if len(obj.GetTags()) > 0 {
		b.WriteString("\nTags:\n")
		for k, v := range obj.GetTags() {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	return b.String(), nil
}

// ToJSON returns a JSON representation of a VPC.
func (v *VPC) ToJSON(path string) (string, error) {
	obj, err := v.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(obj.GetRaw(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal VPC to JSON: %w", err)
	}

	return string(data), nil
}

// Delete deletes a VPC by path. Only works if the VPC is empty.
func (v *VPC) Delete(ctx context.Context, path string, force bool) error {
	region, vpcID, err := parseVPCPath(path)
	if err != nil {
		return err
	}

	// Get VPC details first to check if it's default
	obj, err := v.Get(ctx, path)
	if err != nil {
		return err
	}

	vpc := obj.GetRaw().(types.Vpc)
	if isDefaultVPC(vpc) && !force {
		return fmt.Errorf("refusing to delete default VPC %s (use --force to override)", vpcID)
	}

	client := v.Client().EC2(region)
	if client == nil {
		return fmt.Errorf("failed to get EC2 client for region %s", region)
	}

	input := &ec2.DeleteVpcInput{
		VpcId: &vpcID,
	}

	_, err = client.DeleteVpc(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete VPC %s: %w", vpcID, err)
	}

	return nil
}

// vpcToAWSObject converts an EC2 VPC to an AWSObject.
func vpcToAWSObject(vpc types.Vpc, region string) AWSObject {
	tags := make(map[string]string)
	name := ""

	for _, tag := range vpc.Tags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
			if *tag.Key == "Name" {
				name = *tag.Value
			}
		}
	}

	vpcID := ""
	if vpc.VpcId != nil {
		vpcID = *vpc.VpcId
	}

	arn := fmt.Sprintf("arn:aws:ec2:%s::vpc/%s", region, vpcID)

	return &BaseAWSObject{
		ARN:    arn,
		ID:     vpcID,
		Name:   name,
		Region: region,
		Tags:   tags,
		Raw:    vpc,
	}
}

// parseVPCPath parses a VPC path in the format "region/vpc-id".
func parseVPCPath(path string) (region, vpcID string, err error) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid VPC path format: expected 'region/vpc-id', got '%s'", path)
	}

	region = parts[0]
	vpcID = parts[1]

	if region == "" {
		return "", "", fmt.Errorf("region cannot be empty in path: %s", path)
	}

	if vpcID == "" {
		return "", "", fmt.Errorf("VPC ID cannot be empty in path: %s", path)
	}

	if !strings.HasPrefix(vpcID, "vpc-") {
		return "", "", fmt.Errorf("invalid VPC ID format: %s (expected vpc-*)", vpcID)
	}

	return region, vpcID, nil
}

// getVPCCIDR returns the primary CIDR block of a VPC.
func getVPCCIDR(vpc types.Vpc) string {
	if vpc.CidrBlock != nil {
		return *vpc.CidrBlock
	}
	return ""
}

// isDefaultVPC checks if a VPC is the default VPC.
func isDefaultVPC(vpc types.Vpc) bool {
	return vpc.IsDefault != nil && *vpc.IsDefault
}
