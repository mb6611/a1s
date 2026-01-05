package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/a1s/a1s/internal/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func init() {
	RegisterAccessor(&EC2SecurityGroupRID, &SecurityGroup{})
}

// SecurityGroup implements DAO for EC2 Security Groups.
type SecurityGroup struct {
	AWSResource
}

// List retrieves all security groups in the specified region.
func (sg *SecurityGroup) List(ctx context.Context, region string) ([]AWSObject, error) {
	factory := sg.getFactory()
	if factory == nil {
		return nil, fmt.Errorf("factory not initialized")
	}

	client := factory.Client().EC2(region)
	if client == nil {
		return nil, fmt.Errorf("failed to get EC2 client for region: %s", region)
	}

	input := &ec2.DescribeSecurityGroupsInput{}
	result, err := client.DescribeSecurityGroups(ctx, input)
	if err != nil {
		return nil, aws.WrapAWSError(err, "DescribeSecurityGroups")
	}

	objects := make([]AWSObject, 0, len(result.SecurityGroups))
	for _, securityGroup := range result.SecurityGroups {
		objects = append(objects, sgToAWSObject(securityGroup, region))
	}

	return objects, nil
}

// Get retrieves a specific security group by path.
// Path format: "region/sg-id"
func (sg *SecurityGroup) Get(ctx context.Context, path string) (AWSObject, error) {
	region, sgID, err := parseSGPath(path)
	if err != nil {
		return nil, err
	}

	factory := sg.getFactory()
	if factory == nil {
		return nil, fmt.Errorf("factory not initialized")
	}

	client := factory.Client().EC2(region)
	if client == nil {
		return nil, fmt.Errorf("failed to get EC2 client for region: %s", region)
	}

	input := &ec2.DescribeSecurityGroupsInput{
		GroupIds: []string{sgID},
	}

	result, err := client.DescribeSecurityGroups(ctx, input)
	if err != nil {
		return nil, aws.WrapAWSError(err, "DescribeSecurityGroups")
	}

	if len(result.SecurityGroups) == 0 {
		return nil, fmt.Errorf("security group not found: %s", sgID)
	}

	return sgToAWSObject(result.SecurityGroups[0], region), nil
}

// Describe returns a formatted description of the security group.
func (sg *SecurityGroup) Describe(path string) (string, error) {
	obj, err := sg.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	base := obj.(*BaseAWSObject)
	raw := base.Raw.(types.SecurityGroup)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Security Group: %s\n", aws.SafeString(raw.GroupName)))
	sb.WriteString(fmt.Sprintf("ID: %s\n", aws.SafeString(raw.GroupId)))
	sb.WriteString(fmt.Sprintf("VPC: %s\n", aws.SafeString(raw.VpcId)))
	sb.WriteString(fmt.Sprintf("Description: %s\n", aws.SafeString(raw.Description)))
	sb.WriteString(fmt.Sprintf("Region: %s\n", base.Region))

	if len(raw.IpPermissions) > 0 {
		sb.WriteString("\nIngress Rules:\n")
		sb.WriteString(formatRules(raw.IpPermissions))
	}

	if len(raw.IpPermissionsEgress) > 0 {
		sb.WriteString("\nEgress Rules:\n")
		sb.WriteString(formatRules(raw.IpPermissionsEgress))
	}

	if len(base.Tags) > 0 {
		sb.WriteString("\nTags:\n")
		for k, v := range base.Tags {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	return sb.String(), nil
}

// ToJSON returns a JSON representation of the security group.
func (sg *SecurityGroup) ToJSON(path string) (string, error) {
	obj, err := sg.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal security group to JSON: %w", err)
	}

	return string(data), nil
}

// Delete removes a security group.
func (sg *SecurityGroup) Delete(ctx context.Context, path string, force bool) error {
	region, sgID, err := parseSGPath(path)
	if err != nil {
		return err
	}

	factory := sg.getFactory()
	if factory == nil {
		return fmt.Errorf("factory not initialized")
	}

	client := factory.Client().EC2(region)
	if client == nil {
		return fmt.Errorf("failed to get EC2 client for region: %s", region)
	}

	input := &ec2.DeleteSecurityGroupInput{
		GroupId: &sgID,
	}

	_, err = client.DeleteSecurityGroup(ctx, input)
	if err != nil {
		return aws.WrapAWSError(err, "DeleteSecurityGroup")
	}

	return nil
}

// AddIngressRule adds an ingress rule to a security group.
func (sg *SecurityGroup) AddIngressRule(ctx context.Context, sgID, protocol string, fromPort, toPort int32, cidr string) error {
	factory := sg.getFactory()
	if factory == nil {
		return fmt.Errorf("factory not initialized")
	}

	region := factory.Region()
	client := factory.Client().EC2(region)
	if client == nil {
		return fmt.Errorf("failed to get EC2 client for region: %s", region)
	}

	ipPermission := types.IpPermission{
		IpProtocol: &protocol,
		FromPort:   &fromPort,
		ToPort:     &toPort,
		IpRanges: []types.IpRange{
			{CidrIp: &cidr},
		},
	}

	input := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       &sgID,
		IpPermissions: []types.IpPermission{ipPermission},
	}

	_, err := client.AuthorizeSecurityGroupIngress(ctx, input)
	if err != nil {
		return aws.WrapAWSError(err, "AuthorizeSecurityGroupIngress")
	}

	return nil
}

// RemoveIngressRule removes an ingress rule from a security group.
func (sg *SecurityGroup) RemoveIngressRule(ctx context.Context, sgID, protocol string, fromPort, toPort int32, cidr string) error {
	factory := sg.getFactory()
	if factory == nil {
		return fmt.Errorf("factory not initialized")
	}

	region := factory.Region()
	client := factory.Client().EC2(region)
	if client == nil {
		return fmt.Errorf("failed to get EC2 client for region: %s", region)
	}

	ipPermission := types.IpPermission{
		IpProtocol: &protocol,
		FromPort:   &fromPort,
		ToPort:     &toPort,
		IpRanges: []types.IpRange{
			{CidrIp: &cidr},
		},
	}

	input := &ec2.RevokeSecurityGroupIngressInput{
		GroupId:       &sgID,
		IpPermissions: []types.IpPermission{ipPermission},
	}

	_, err := client.RevokeSecurityGroupIngress(ctx, input)
	if err != nil {
		return aws.WrapAWSError(err, "RevokeSecurityGroupIngress")
	}

	return nil
}

// AddEgressRule adds an egress rule to a security group.
func (sg *SecurityGroup) AddEgressRule(ctx context.Context, sgID, protocol string, fromPort, toPort int32, cidr string) error {
	factory := sg.getFactory()
	if factory == nil {
		return fmt.Errorf("factory not initialized")
	}

	region := factory.Region()
	client := factory.Client().EC2(region)
	if client == nil {
		return fmt.Errorf("failed to get EC2 client for region: %s", region)
	}

	ipPermission := types.IpPermission{
		IpProtocol: &protocol,
		FromPort:   &fromPort,
		ToPort:     &toPort,
		IpRanges: []types.IpRange{
			{CidrIp: &cidr},
		},
	}

	input := &ec2.AuthorizeSecurityGroupEgressInput{
		GroupId:       &sgID,
		IpPermissions: []types.IpPermission{ipPermission},
	}

	_, err := client.AuthorizeSecurityGroupEgress(ctx, input)
	if err != nil {
		return aws.WrapAWSError(err, "AuthorizeSecurityGroupEgress")
	}

	return nil
}

// RemoveEgressRule removes an egress rule from a security group.
func (sg *SecurityGroup) RemoveEgressRule(ctx context.Context, sgID, protocol string, fromPort, toPort int32, cidr string) error {
	factory := sg.getFactory()
	if factory == nil {
		return fmt.Errorf("factory not initialized")
	}

	region := factory.Region()
	client := factory.Client().EC2(region)
	if client == nil {
		return fmt.Errorf("failed to get EC2 client for region: %s", region)
	}

	ipPermission := types.IpPermission{
		IpProtocol: &protocol,
		FromPort:   &fromPort,
		ToPort:     &toPort,
		IpRanges: []types.IpRange{
			{CidrIp: &cidr},
		},
	}

	input := &ec2.RevokeSecurityGroupEgressInput{
		GroupId:       &sgID,
		IpPermissions: []types.IpPermission{ipPermission},
	}

	_, err := client.RevokeSecurityGroupEgress(ctx, input)
	if err != nil {
		return aws.WrapAWSError(err, "RevokeSecurityGroupEgress")
	}

	return nil
}

// Helper functions

// sgToAWSObject converts an EC2 SecurityGroup to an AWSObject.
func sgToAWSObject(sg types.SecurityGroup, region string) AWSObject {
	tags := make(map[string]string)
	for _, tag := range sg.Tags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}

	name := aws.SafeString(sg.GroupName)
	if tagName, ok := tags["Name"]; ok {
		name = tagName
	}

	return &BaseAWSObject{
		ARN:       buildSecurityGroupARN(region, sg),
		ID:        aws.SafeString(sg.GroupId),
		Name:      name,
		Region:    region,
		Tags:      tags,
		CreatedAt: nil, // Security groups don't have a creation timestamp in the API
		Raw:       sg,
	}
}

// parseSGPath parses a security group path in the format "region/sg-id".
func parseSGPath(path string) (region, sgID string, err error) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid security group path format: %s (expected: region/sg-id)", path)
	}
	return parts[0], parts[1], nil
}

// formatRules formats IP permissions into a readable string.
func formatRules(perms []types.IpPermission) string {
	var sb strings.Builder

	for _, perm := range perms {
		protocol := aws.SafeString(perm.IpProtocol)
		if protocol == "-1" {
			protocol = "All"
		}

		var portRange string
		if perm.FromPort != nil && perm.ToPort != nil {
			if *perm.FromPort == *perm.ToPort {
				portRange = fmt.Sprintf("%d", *perm.FromPort)
			} else {
				portRange = fmt.Sprintf("%d-%d", *perm.FromPort, *perm.ToPort)
			}
		} else {
			portRange = "All"
		}

		// IPv4 ranges
		for _, ipRange := range perm.IpRanges {
			cidr := aws.SafeString(ipRange.CidrIp)
			description := aws.SafeString(ipRange.Description)
			if description != "" {
				sb.WriteString(fmt.Sprintf("  %s %s %s (%s)\n", protocol, portRange, cidr, description))
			} else {
				sb.WriteString(fmt.Sprintf("  %s %s %s\n", protocol, portRange, cidr))
			}
		}

		// IPv6 ranges
		for _, ipv6Range := range perm.Ipv6Ranges {
			cidr := aws.SafeString(ipv6Range.CidrIpv6)
			description := aws.SafeString(ipv6Range.Description)
			if description != "" {
				sb.WriteString(fmt.Sprintf("  %s %s %s (%s)\n", protocol, portRange, cidr, description))
			} else {
				sb.WriteString(fmt.Sprintf("  %s %s %s\n", protocol, portRange, cidr))
			}
		}

		// Security group references
		for _, ugp := range perm.UserIdGroupPairs {
			sgID := aws.SafeString(ugp.GroupId)
			description := aws.SafeString(ugp.Description)
			if description != "" {
				sb.WriteString(fmt.Sprintf("  %s %s %s (%s)\n", protocol, portRange, sgID, description))
			} else {
				sb.WriteString(fmt.Sprintf("  %s %s %s\n", protocol, portRange, sgID))
			}
		}
	}

	return sb.String()
}

// buildSecurityGroupARN constructs an ARN for a security group.
func buildSecurityGroupARN(region string, sg types.SecurityGroup) string {
	// ARN format: arn:aws:ec2:region:account-id:security-group/sg-id
	// We don't have account ID readily available, so we'll use a placeholder
	// The factory should provide this, but for now we'll return a partial ARN
	return fmt.Sprintf("arn:aws:ec2:%s:*:security-group/%s",
		region,
		aws.SafeString(sg.GroupId))
}
