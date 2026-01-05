package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
)

func init() {
	RegisterAccessor(&EKSNodeGroupRID, &EKSNodeGroup{})
}

// EKSNodeGroup implements the DAO for AWS EKS Node Groups.
type EKSNodeGroup struct {
	AWSResource
}

// Scalable interface for scaling node groups.
type Scalable interface {
	Scale(ctx context.Context, path string, desiredSize int32) error
}

// List retrieves all EKS node groups across all clusters in the specified region.
func (n *EKSNodeGroup) List(ctx context.Context, region string) ([]AWSObject, error) {
	f := n.getFactory()
	if f == nil {
		return nil, fmt.Errorf("factory not initialized")
	}

	eksClient := f.Client().EKS(region)
	if eksClient == nil {
		return nil, fmt.Errorf("failed to get EKS client for region %s", region)
	}

	// List all clusters first
	clustersInput := &eks.ListClustersInput{}
	clustersResult, err := eksClient.ListClusters(ctx, clustersInput)
	if err != nil {
		return nil, fmt.Errorf("failed to list EKS clusters: %w", err)
	}

	var objects []AWSObject

	// For each cluster, list its node groups
	for _, clusterName := range clustersResult.Clusters {
		ngInput := &eks.ListNodegroupsInput{
			ClusterName: aws.String(clusterName),
		}

		ngResult, err := eksClient.ListNodegroups(ctx, ngInput)
		if err != nil {
			return nil, fmt.Errorf("failed to list node groups for cluster %s: %w", clusterName, err)
		}

		// Describe each node group
		for _, ngName := range ngResult.Nodegroups {
			describeInput := &eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(ngName),
			}

			describeResult, err := eksClient.DescribeNodegroup(ctx, describeInput)
			if err != nil {
				return nil, fmt.Errorf("failed to describe node group %s in cluster %s: %w", ngName, clusterName, err)
			}

			if describeResult.Nodegroup != nil {
				obj := nodegroupToAWSObject(describeResult.Nodegroup, region, clusterName)
				objects = append(objects, obj)
			}
		}
	}

	return objects, nil
}

// Get retrieves a single EKS node group by path (region/cluster-name/nodegroup-name).
func (n *EKSNodeGroup) Get(ctx context.Context, path string) (AWSObject, error) {
	region, clusterName, nodegroupName, err := parseNodegroupPath(path)
	if err != nil {
		return nil, err
	}

	f := n.getFactory()
	if f == nil {
		return nil, fmt.Errorf("factory not initialized")
	}

	eksClient := f.Client().EKS(region)
	if eksClient == nil {
		return nil, fmt.Errorf("failed to get EKS client for region %s", region)
	}

	input := &eks.DescribeNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodegroupName),
	}

	result, err := eksClient.DescribeNodegroup(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe node group %s in cluster %s: %w", nodegroupName, clusterName, err)
	}

	if result.Nodegroup == nil {
		return nil, fmt.Errorf("node group %s not found in cluster %s", nodegroupName, clusterName)
	}

	return nodegroupToAWSObject(result.Nodegroup, region, clusterName), nil
}

// Describe returns a formatted description of an EKS node group.
func (n *EKSNodeGroup) Describe(path string) (string, error) {
	obj, err := n.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	ng := obj.GetRaw().(types.Nodegroup)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Node Group Name: %s\n", obj.GetName()))
	b.WriteString(fmt.Sprintf("ARN:             %s\n", obj.GetARN()))
	b.WriteString(fmt.Sprintf("Region:          %s\n", obj.GetRegion()))
	b.WriteString(fmt.Sprintf("Status:          %s\n", ng.Status))

	if ng.ScalingConfig != nil {
		b.WriteString("\nScaling Config:\n")
		if ng.ScalingConfig.MinSize != nil {
			b.WriteString(fmt.Sprintf("  Min Size:     %d\n", *ng.ScalingConfig.MinSize))
		}
		if ng.ScalingConfig.MaxSize != nil {
			b.WriteString(fmt.Sprintf("  Max Size:     %d\n", *ng.ScalingConfig.MaxSize))
		}
		if ng.ScalingConfig.DesiredSize != nil {
			b.WriteString(fmt.Sprintf("  Desired Size: %d\n", *ng.ScalingConfig.DesiredSize))
		}
	}

	if ng.InstanceTypes != nil && len(ng.InstanceTypes) > 0 {
		b.WriteString(fmt.Sprintf("\nInstance Types:  %s\n", strings.Join(ng.InstanceTypes, ", ")))
	}

	if ng.AmiType != "" {
		b.WriteString(fmt.Sprintf("AMI Type:        %s\n", ng.AmiType))
	}

	if ng.NodeRole != nil {
		b.WriteString(fmt.Sprintf("Node Role:       %s\n", *ng.NodeRole))
	}

	if ng.CreatedAt != nil {
		b.WriteString(fmt.Sprintf("\nCreated At:      %s\n", ng.CreatedAt.Format("2006-01-02 15:04:05")))
	}

	if len(obj.GetTags()) > 0 {
		b.WriteString("\nTags:\n")
		for k, v := range obj.GetTags() {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	return b.String(), nil
}

// ToJSON returns a JSON representation of an EKS node group.
func (n *EKSNodeGroup) ToJSON(path string) (string, error) {
	obj, err := n.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(obj.GetRaw(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal node group to JSON: %w", err)
	}

	return string(data), nil
}

// Delete deletes an EKS node group.
func (n *EKSNodeGroup) Delete(ctx context.Context, path string, force bool) error {
	region, clusterName, nodegroupName, err := parseNodegroupPath(path)
	if err != nil {
		return err
	}

	f := n.getFactory()
	if f == nil {
		return fmt.Errorf("factory not initialized")
	}

	eksClient := f.Client().EKS(region)
	if eksClient == nil {
		return fmt.Errorf("failed to get EKS client for region %s", region)
	}

	input := &eks.DeleteNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodegroupName),
	}

	_, err = eksClient.DeleteNodegroup(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete node group %s in cluster %s: %w", nodegroupName, clusterName, err)
	}

	return nil
}

// Scale updates the desired size of an EKS node group.
func (n *EKSNodeGroup) Scale(ctx context.Context, path string, desiredSize int32) error {
	region, clusterName, nodegroupName, err := parseNodegroupPath(path)
	if err != nil {
		return err
	}

	f := n.getFactory()
	if f == nil {
		return fmt.Errorf("factory not initialized")
	}

	eksClient := f.Client().EKS(region)
	if eksClient == nil {
		return fmt.Errorf("failed to get EKS client for region %s", region)
	}

	// Get current scaling config to preserve min/max
	min, max, _, err := n.GetScalingConfig(ctx, clusterName, nodegroupName)
	if err != nil {
		return fmt.Errorf("failed to get current scaling config: %w", err)
	}

	// Validate desired size is within bounds
	if desiredSize < min {
		return fmt.Errorf("desired size %d is less than minimum size %d", desiredSize, min)
	}
	if desiredSize > max {
		return fmt.Errorf("desired size %d is greater than maximum size %d", desiredSize, max)
	}

	input := &eks.UpdateNodegroupConfigInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodegroupName),
		ScalingConfig: &types.NodegroupScalingConfig{
			MinSize:     aws.Int32(min),
			MaxSize:     aws.Int32(max),
			DesiredSize: aws.Int32(desiredSize),
		},
	}

	_, err = eksClient.UpdateNodegroupConfig(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to scale node group %s in cluster %s: %w", nodegroupName, clusterName, err)
	}

	return nil
}

// GetScalingConfig retrieves the current scaling configuration for a node group.
func (n *EKSNodeGroup) GetScalingConfig(ctx context.Context, clusterName, nodegroupName string) (min, max, desired int32, err error) {
	f := n.getFactory()
	if f == nil {
		return 0, 0, 0, fmt.Errorf("factory not initialized")
	}

	region := f.Region()
	eksClient := f.Client().EKS(region)
	if eksClient == nil {
		return 0, 0, 0, fmt.Errorf("failed to get EKS client for region %s", region)
	}

	input := &eks.DescribeNodegroupInput{
		ClusterName:   aws.String(clusterName),
		NodegroupName: aws.String(nodegroupName),
	}

	result, err := eksClient.DescribeNodegroup(ctx, input)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to describe node group %s: %w", nodegroupName, err)
	}

	if result.Nodegroup == nil || result.Nodegroup.ScalingConfig == nil {
		return 0, 0, 0, fmt.Errorf("no scaling config found for node group %s", nodegroupName)
	}

	sc := result.Nodegroup.ScalingConfig
	if sc.MinSize != nil {
		min = *sc.MinSize
	}
	if sc.MaxSize != nil {
		max = *sc.MaxSize
	}
	if sc.DesiredSize != nil {
		desired = *sc.DesiredSize
	}

	return min, max, desired, nil
}

// nodegroupToAWSObject converts an EKS Nodegroup to an AWSObject.
func nodegroupToAWSObject(ng *types.Nodegroup, region, clusterName string) AWSObject {
	tags := make(map[string]string)
	if ng.Tags != nil {
		for k, v := range ng.Tags {
			tags[k] = v
		}
	}

	name := ""
	if ng.NodegroupName != nil {
		name = *ng.NodegroupName
	}

	arn := ""
	if ng.NodegroupArn != nil {
		arn = *ng.NodegroupArn
	}

	return &BaseAWSObject{
		ARN:       arn,
		ID:        name,
		Name:      name,
		Region:    region,
		Tags:      tags,
		CreatedAt: ng.CreatedAt,
		Raw:       *ng,
	}
}

// parseNodegroupPath parses a node group path in the format "region/cluster-name/nodegroup-name".
func parseNodegroupPath(path string) (region, clusterName, nodegroupName string, err error) {
	parts := strings.Split(path, "/")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid node group path format: expected 'region/cluster-name/nodegroup-name', got '%s'", path)
	}

	region = parts[0]
	clusterName = parts[1]
	nodegroupName = parts[2]

	if region == "" {
		return "", "", "", fmt.Errorf("region cannot be empty in path: %s", path)
	}

	if clusterName == "" {
		return "", "", "", fmt.Errorf("cluster name cannot be empty in path: %s", path)
	}

	if nodegroupName == "" {
		return "", "", "", fmt.Errorf("node group name cannot be empty in path: %s", path)
	}

	return region, clusterName, nodegroupName, nil
}
