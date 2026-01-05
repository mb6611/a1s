package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
)

func init() {
	RegisterAccessor(&EKSClusterRID, &EKSCluster{})
}

// EKSCluster is the DAO for EKS clusters.
type EKSCluster struct {
	AWSResource
}

// List returns all EKS clusters in the specified region.
func (e *EKSCluster) List(ctx context.Context, region string) ([]AWSObject, error) {
	client := e.Client().EKS(region)
	if client == nil {
		return nil, fmt.Errorf("failed to get EKS client for region %s", region)
	}

	input := &eks.ListClustersInput{}
	paginator := eks.NewListClustersPaginator(client, input)

	var clusters []AWSObject
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list clusters: %w", err)
		}

		// For each cluster name, fetch detailed information
		for _, clusterName := range output.Clusters {
			describeInput := &eks.DescribeClusterInput{
				Name: &clusterName,
			}

			describeOutput, err := client.DescribeCluster(ctx, describeInput)
			if err != nil {
				return nil, fmt.Errorf("failed to describe cluster %s: %w", clusterName, err)
			}

			if describeOutput.Cluster != nil {
				clusters = append(clusters, clusterToAWSObject(describeOutput.Cluster, region))
			}
		}
	}

	return clusters, nil
}

// Get retrieves a single EKS cluster by path (format: "region/cluster-name").
func (e *EKSCluster) Get(ctx context.Context, path string) (AWSObject, error) {
	region, clusterName, err := parseClusterPath(path)
	if err != nil {
		return nil, err
	}

	client := e.Client().EKS(region)
	if client == nil {
		return nil, fmt.Errorf("failed to get EKS client for region %s", region)
	}

	input := &eks.DescribeClusterInput{
		Name: &clusterName,
	}

	output, err := client.DescribeCluster(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe cluster: %w", err)
	}

	if output.Cluster == nil {
		return nil, fmt.Errorf("cluster not found: %s", clusterName)
	}

	return clusterToAWSObject(output.Cluster, region), nil
}

// Describe returns a formatted description of the EKS cluster.
func (e *EKSCluster) Describe(path string) (string, error) {
	obj, err := e.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	cluster, ok := obj.GetRaw().(*types.Cluster)
	if !ok {
		return "", fmt.Errorf("invalid cluster object")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Cluster Name: %s\n", obj.GetName()))
	sb.WriteString(fmt.Sprintf("ARN: %s\n", obj.GetARN()))
	sb.WriteString(fmt.Sprintf("Status: %s\n", cluster.Status))
	sb.WriteString(fmt.Sprintf("Version: %s\n", safeString(cluster.Version)))
	sb.WriteString(fmt.Sprintf("Region: %s\n", obj.GetRegion()))

	if cluster.Endpoint != nil {
		sb.WriteString(fmt.Sprintf("Endpoint: %s\n", *cluster.Endpoint))
	}

	if cluster.RoleArn != nil {
		sb.WriteString(fmt.Sprintf("Role ARN: %s\n", *cluster.RoleArn))
	}

	if cluster.ResourcesVpcConfig != nil {
		sb.WriteString("VPC Configuration:\n")
		if cluster.ResourcesVpcConfig.VpcId != nil {
			sb.WriteString(fmt.Sprintf("  VPC ID: %s\n", *cluster.ResourcesVpcConfig.VpcId))
		}
		if len(cluster.ResourcesVpcConfig.SubnetIds) > 0 {
			sb.WriteString(fmt.Sprintf("  Subnet IDs: %s\n", strings.Join(cluster.ResourcesVpcConfig.SubnetIds, ", ")))
		}
		if len(cluster.ResourcesVpcConfig.SecurityGroupIds) > 0 {
			sb.WriteString(fmt.Sprintf("  Security Group IDs: %s\n", strings.Join(cluster.ResourcesVpcConfig.SecurityGroupIds, ", ")))
		}
		sb.WriteString(fmt.Sprintf("  Endpoint Public Access: %t\n", cluster.ResourcesVpcConfig.EndpointPublicAccess))
		sb.WriteString(fmt.Sprintf("  Endpoint Private Access: %t\n", cluster.ResourcesVpcConfig.EndpointPrivateAccess))
	}

	if cluster.PlatformVersion != nil {
		sb.WriteString(fmt.Sprintf("Platform Version: %s\n", *cluster.PlatformVersion))
	}

	if obj.GetCreatedAt() != nil {
		sb.WriteString(fmt.Sprintf("Created At: %s\n", obj.GetCreatedAt().Format("2006-01-02 15:04:05")))
	}

	if len(obj.GetTags()) > 0 {
		sb.WriteString("Tags:\n")
		for k, v := range obj.GetTags() {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	return sb.String(), nil
}

// ToJSON returns a JSON representation of the EKS cluster.
func (e *EKSCluster) ToJSON(path string) (string, error) {
	obj, err := e.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(obj.GetRaw(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal cluster to JSON: %w", err)
	}

	return string(data), nil
}

// Delete deletes an EKS cluster.
// If force is true, it will first delete all nodegroups and fargate profiles.
func (e *EKSCluster) Delete(ctx context.Context, path string, force bool) error {
	region, clusterName, err := parseClusterPath(path)
	if err != nil {
		return err
	}

	client := e.Client().EKS(region)
	if client == nil {
		return fmt.Errorf("failed to get EKS client for region %s", region)
	}

	if force {
		// Delete all nodegroups first
		nodegroups, err := e.listNodeGroupsForRegion(ctx, region, clusterName)
		if err != nil {
			return fmt.Errorf("failed to list nodegroups: %w", err)
		}

		for _, ng := range nodegroups {
			deleteNGInput := &eks.DeleteNodegroupInput{
				ClusterName:   &clusterName,
				NodegroupName: &ng,
			}

			_, err := client.DeleteNodegroup(ctx, deleteNGInput)
			if err != nil {
				return fmt.Errorf("failed to delete nodegroup %s: %w", ng, err)
			}
		}

		// Wait for nodegroups to be deleted
		for _, ng := range nodegroups {
			waiter := eks.NewNodegroupDeletedWaiter(client)
			err := waiter.Wait(ctx, &eks.DescribeNodegroupInput{
				ClusterName:   &clusterName,
				NodegroupName: &ng,
			}, 15*time.Minute)
			if err != nil {
				return fmt.Errorf("failed waiting for nodegroup %s deletion: %w", ng, err)
			}
		}

		// Delete all fargate profiles
		fargateProfiles, err := e.listFargateProfilesForRegion(ctx, region, clusterName)
		if err != nil {
			return fmt.Errorf("failed to list fargate profiles: %w", err)
		}

		for _, fp := range fargateProfiles {
			deleteFPInput := &eks.DeleteFargateProfileInput{
				ClusterName:        &clusterName,
				FargateProfileName: &fp,
			}

			_, err := client.DeleteFargateProfile(ctx, deleteFPInput)
			if err != nil {
				return fmt.Errorf("failed to delete fargate profile %s: %w", fp, err)
			}
		}

		// Wait for fargate profiles to be deleted
		for _, fp := range fargateProfiles {
			waiter := eks.NewFargateProfileDeletedWaiter(client)
			err := waiter.Wait(ctx, &eks.DescribeFargateProfileInput{
				ClusterName:        &clusterName,
				FargateProfileName: &fp,
			}, 15*time.Minute)
			if err != nil {
				return fmt.Errorf("failed waiting for fargate profile %s deletion: %w", fp, err)
			}
		}
	}

	// Delete the cluster
	input := &eks.DeleteClusterInput{
		Name: &clusterName,
	}

	_, err = client.DeleteCluster(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	return nil
}

// GetKubeconfig generates a kubeconfig YAML for the cluster.
func (e *EKSCluster) GetKubeconfig(ctx context.Context, clusterName string) (string, error) {
	region := e.Region()
	client := e.Client().EKS(region)
	if client == nil {
		return "", fmt.Errorf("failed to get EKS client for region %s", region)
	}

	input := &eks.DescribeClusterInput{
		Name: &clusterName,
	}

	output, err := client.DescribeCluster(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to describe cluster: %w", err)
	}

	if output.Cluster == nil {
		return "", fmt.Errorf("cluster not found: %s", clusterName)
	}

	return generateKubeconfig(output.Cluster, region), nil
}

// ListNodeGroups returns all nodegroups for a cluster.
func (e *EKSCluster) ListNodeGroups(ctx context.Context, clusterName string) ([]string, error) {
	return e.listNodeGroupsForRegion(ctx, e.Region(), clusterName)
}

// listNodeGroupsForRegion returns all nodegroups for a cluster in a specific region.
func (e *EKSCluster) listNodeGroupsForRegion(ctx context.Context, region, clusterName string) ([]string, error) {
	client := e.Client().EKS(region)
	if client == nil {
		return nil, fmt.Errorf("failed to get EKS client for region %s", region)
	}

	input := &eks.ListNodegroupsInput{
		ClusterName: &clusterName,
	}

	paginator := eks.NewListNodegroupsPaginator(client, input)

	var nodegroups []string
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list nodegroups: %w", err)
		}

		nodegroups = append(nodegroups, output.Nodegroups...)
	}

	return nodegroups, nil
}

// ListFargateProfiles returns all fargate profiles for a cluster.
func (e *EKSCluster) ListFargateProfiles(ctx context.Context, clusterName string) ([]string, error) {
	return e.listFargateProfilesForRegion(ctx, e.Region(), clusterName)
}

// listFargateProfilesForRegion returns all fargate profiles for a cluster in a specific region.
func (e *EKSCluster) listFargateProfilesForRegion(ctx context.Context, region, clusterName string) ([]string, error) {
	client := e.Client().EKS(region)
	if client == nil {
		return nil, fmt.Errorf("failed to get EKS client for region %s", region)
	}

	input := &eks.ListFargateProfilesInput{
		ClusterName: &clusterName,
	}

	paginator := eks.NewListFargateProfilesPaginator(client, input)

	var profiles []string
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list fargate profiles: %w", err)
		}

		profiles = append(profiles, output.FargateProfileNames...)
	}

	return profiles, nil
}

// clusterToAWSObject converts an EKS cluster to an AWSObject.
func clusterToAWSObject(cluster *types.Cluster, region string) AWSObject {
	tags := make(map[string]string)
	for k, v := range cluster.Tags {
		tags[k] = v
	}

	var arn string
	if cluster.Arn != nil {
		arn = *cluster.Arn
	}

	var name string
	if cluster.Name != nil {
		name = *cluster.Name
	}

	return &BaseAWSObject{
		ARN:       arn,
		ID:        name, // For EKS, the cluster name is the ID
		Name:      name,
		Region:    region,
		Tags:      tags,
		CreatedAt: cluster.CreatedAt,
		Raw:       cluster,
	}
}

// parseClusterPath parses a path in the format "region/cluster-name".
func parseClusterPath(path string) (region, clusterName string, err error) {
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid path format, expected 'region/cluster-name', got: %s", path)
	}

	region = strings.TrimSpace(parts[0])
	clusterName = strings.TrimSpace(parts[1])

	if region == "" || clusterName == "" {
		return "", "", fmt.Errorf("region and cluster-name cannot be empty")
	}

	return region, clusterName, nil
}

// generateKubeconfig generates a kubeconfig YAML for the cluster.
func generateKubeconfig(cluster *types.Cluster, region string) string {
	clusterName := safeString(cluster.Name)
	endpoint := safeString(cluster.Endpoint)
	ca := ""
	if cluster.CertificateAuthority != nil {
		ca = safeString(cluster.CertificateAuthority.Data)
	}

	kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: %s
    server: %s
  name: %s
contexts:
- context:
    cluster: %s
    user: %s
  name: %s
current-context: %s
users:
- name: %s
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args:
      - eks
      - get-token
      - --cluster-name
      - %s
      - --region
      - %s
`,
		ca,
		endpoint,
		clusterName,
		clusterName,
		clusterName,
		clusterName,
		clusterName,
		clusterName,
		clusterName,
		region,
	)

	return kubeconfig
}

// safeString safely dereferences a string pointer, returning empty string if nil.
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// safeBool safely dereferences a bool pointer, returning false if nil.
func safeBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
