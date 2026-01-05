package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsinternal "github.com/a1s/a1s/internal/aws"
)

func init() {
	RegisterAccessor(&EC2VolumeRID, &EC2Volume{})
}

// EC2Volume implements the DAO for EC2 EBS Volumes.
type EC2Volume struct {
	AWSResource
}

// List retrieves all EBS volumes in the specified region using pagination.
func (v *EC2Volume) List(ctx context.Context, region string) ([]AWSObject, error) {
	f := v.getFactory()
	if f == nil {
		return nil, fmt.Errorf("factory not initialized")
	}

	client := f.Client().EC2(region)
	if client == nil {
		return nil, fmt.Errorf("failed to get EC2 client for region: %s", region)
	}

	var objects []AWSObject
	paginator := ec2.NewDescribeVolumesPaginator(client, &ec2.DescribeVolumesInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, awsinternal.WrapAWSError(err, "DescribeVolumes")
		}

		for _, volume := range page.Volumes {
			objects = append(objects, volumeToAWSObject(volume, region))
		}
	}

	return objects, nil
}

// Get retrieves a single EBS volume by path (format: "region/volume-id").
func (v *EC2Volume) Get(ctx context.Context, path string) (AWSObject, error) {
	region, volumeID, err := parseVolumePath(path)
	if err != nil {
		return nil, err
	}

	f := v.getFactory()
	if f == nil {
		return nil, fmt.Errorf("factory not initialized")
	}

	client := f.Client().EC2(region)
	if client == nil {
		return nil, fmt.Errorf("failed to get EC2 client for region: %s", region)
	}

	result, err := client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
		VolumeIds: []string{volumeID},
	})
	if err != nil {
		return nil, awsinternal.WrapAWSError(err, "DescribeVolumes")
	}

	if len(result.Volumes) == 0 {
		return nil, fmt.Errorf("volume not found: %s", volumeID)
	}

	return volumeToAWSObject(result.Volumes[0], region), nil
}

// Describe returns a human-readable description of the volume.
func (v *EC2Volume) Describe(path string) (string, error) {
	obj, err := v.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	volume, ok := obj.GetRaw().(types.Volume)
	if !ok {
		return "", fmt.Errorf("invalid volume object")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Volume ID: %s\n", aws.ToString(volume.VolumeId)))
	if volume.Size != nil {
		sb.WriteString(fmt.Sprintf("Size: %d GiB\n", *volume.Size))
	}
	sb.WriteString(fmt.Sprintf("Type: %s\n", volume.VolumeType))
	sb.WriteString(fmt.Sprintf("State: %s\n", volume.State))
	if volume.AvailabilityZone != nil {
		sb.WriteString(fmt.Sprintf("Availability Zone: %s\n", *volume.AvailabilityZone))
	}
	if volume.Iops != nil {
		sb.WriteString(fmt.Sprintf("IOPS: %d\n", *volume.Iops))
	}
	if volume.Throughput != nil {
		sb.WriteString(fmt.Sprintf("Throughput: %d MiB/s\n", *volume.Throughput))
	}
	if volume.Encrypted != nil {
		sb.WriteString(fmt.Sprintf("Encrypted: %t\n", *volume.Encrypted))
	}
	if volume.SnapshotId != nil && *volume.SnapshotId != "" {
		sb.WriteString(fmt.Sprintf("Snapshot ID: %s\n", *volume.SnapshotId))
	}

	attachmentInfo := getVolumeAttachmentInfo(volume.Attachments)
	if attachmentInfo != "" {
		sb.WriteString(fmt.Sprintf("Attachments:\n%s", attachmentInfo))
	}

	if len(volume.Tags) > 0 {
		sb.WriteString("Tags:\n")
		for _, tag := range volume.Tags {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", aws.ToString(tag.Key), aws.ToString(tag.Value)))
		}
	}

	return sb.String(), nil
}

// ToJSON returns a JSON representation of the volume.
func (v *EC2Volume) ToJSON(path string) (string, error) {
	obj, err := v.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(obj.GetRaw(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal volume to JSON: %w", err)
	}

	return string(data), nil
}

// Delete deletes an EBS volume.
func (v *EC2Volume) Delete(ctx context.Context, path string, force bool) error {
	region, volumeID, err := parseVolumePath(path)
	if err != nil {
		return err
	}

	f := v.getFactory()
	if f == nil {
		return fmt.Errorf("factory not initialized")
	}

	client := f.Client().EC2(region)
	if client == nil {
		return fmt.Errorf("failed to get EC2 client for region: %s", region)
	}

	// If force is true, attempt to detach the volume first
	if force {
		// Get volume to check attachments
		result, err := client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
			VolumeIds: []string{volumeID},
		})
		if err != nil {
			return awsinternal.WrapAWSError(err, "DescribeVolumes")
		}

		if len(result.Volumes) > 0 && len(result.Volumes[0].Attachments) > 0 {
			// Detach volume forcefully
			if err := v.Detach(ctx, region, volumeID, true); err != nil {
				return fmt.Errorf("failed to detach volume before deletion: %w", err)
			}
		}
	}

	_, err = client.DeleteVolume(ctx, &ec2.DeleteVolumeInput{
		VolumeId: aws.String(volumeID),
	})
	if err != nil {
		return awsinternal.WrapAWSError(err, "DeleteVolume")
	}

	return nil
}

// Attach attaches a volume to an EC2 instance.
func (v *EC2Volume) Attach(ctx context.Context, region, volumeID, instanceID, device string) error {
	f := v.getFactory()
	if f == nil {
		return fmt.Errorf("factory not initialized")
	}

	client := f.Client().EC2(region)
	if client == nil {
		return fmt.Errorf("failed to get EC2 client for region: %s", region)
	}

	_, err := client.AttachVolume(ctx, &ec2.AttachVolumeInput{
		VolumeId:   aws.String(volumeID),
		InstanceId: aws.String(instanceID),
		Device:     aws.String(device),
	})
	if err != nil {
		return awsinternal.WrapAWSError(err, "AttachVolume")
	}

	return nil
}

// Detach detaches a volume from its instance.
func (v *EC2Volume) Detach(ctx context.Context, region, volumeID string, force bool) error {
	f := v.getFactory()
	if f == nil {
		return fmt.Errorf("factory not initialized")
	}

	client := f.Client().EC2(region)
	if client == nil {
		return fmt.Errorf("failed to get EC2 client for region: %s", region)
	}

	_, err := client.DetachVolume(ctx, &ec2.DetachVolumeInput{
		VolumeId: aws.String(volumeID),
		Force:    aws.Bool(force),
	})
	if err != nil {
		return awsinternal.WrapAWSError(err, "DetachVolume")
	}

	return nil
}

// CreateSnapshot creates a snapshot of the specified volume.
func (v *EC2Volume) CreateSnapshot(ctx context.Context, region, volumeID, description string) (string, error) {
	f := v.getFactory()
	if f == nil {
		return "", fmt.Errorf("factory not initialized")
	}

	client := f.Client().EC2(region)
	if client == nil {
		return "", fmt.Errorf("failed to get EC2 client for region: %s", region)
	}

	input := &ec2.CreateSnapshotInput{
		VolumeId: aws.String(volumeID),
	}
	if description != "" {
		input.Description = aws.String(description)
	}

	result, err := client.CreateSnapshot(ctx, input)
	if err != nil {
		return "", awsinternal.WrapAWSError(err, "CreateSnapshot")
	}

	if result.SnapshotId == nil {
		return "", fmt.Errorf("snapshot creation returned no snapshot ID")
	}

	return *result.SnapshotId, nil
}

// volumeToAWSObject converts an EC2 Volume to an AWSObject.
func volumeToAWSObject(volume types.Volume, region string) AWSObject {
	tags := make(map[string]string)
	var name string

	for _, tag := range volume.Tags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)
		tags[key] = value
		if key == "Name" {
			name = value
		}
	}

	// Build ARN: arn:aws:ec2:region:account-id:volume/volume-id
	// Note: We don't have account ID in the volume object, so we construct a partial ARN
	arn := fmt.Sprintf("arn:aws:ec2:%s::volume/%s", region, aws.ToString(volume.VolumeId))

	return &BaseAWSObject{
		ARN:       arn,
		ID:        aws.ToString(volume.VolumeId),
		Name:      name,
		Region:    region,
		Tags:      tags,
		CreatedAt: volume.CreateTime,
		Raw:       volume,
	}
}

// parseVolumePath parses a path in the format "region/volume-id".
func parseVolumePath(path string) (region, volumeID string, err error) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid volume path format, expected 'region/volume-id', got: %s", path)
	}

	region = parts[0]
	volumeID = parts[1]

	if region == "" || volumeID == "" {
		return "", "", fmt.Errorf("invalid volume path, region and volume-id cannot be empty: %s", path)
	}

	return region, volumeID, nil
}

// getVolumeAttachmentInfo returns formatted attachment information.
func getVolumeAttachmentInfo(attachments []types.VolumeAttachment) string {
	if len(attachments) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, attachment := range attachments {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("  Instance ID: %s\n", aws.ToString(attachment.InstanceId)))
		sb.WriteString(fmt.Sprintf("  Device: %s\n", aws.ToString(attachment.Device)))
		sb.WriteString(fmt.Sprintf("  State: %s\n", attachment.State))
		if attachment.DeleteOnTermination != nil {
			sb.WriteString(fmt.Sprintf("  Delete on Termination: %t\n", *attachment.DeleteOnTermination))
		}
	}

	return sb.String()
}
