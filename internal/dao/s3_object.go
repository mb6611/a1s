package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/a1s/a1s/internal/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func init() {
	RegisterAccessor(&S3ObjectRID, &S3Object{})
}

// S3Object is the DAO for S3 objects with hierarchical navigation.
type S3Object struct {
	AWSResource
}

// ListResult represents hierarchical S3 listing results.
type ListResult struct {
	Objects        []AWSObject
	CommonPrefixes []string
}

// List returns objects in a bucket with hierarchical navigation.
// Path format: "bucket" or "bucket/prefix/"
// Uses Delimiter="/" for folder-like navigation.
func (s *S3Object) List(ctx context.Context, path string) ([]AWSObject, error) {
	bucket, prefix := parseListPath(path)
	if bucket == "" {
		return nil, fmt.Errorf("invalid path format, expected 'bucket' or 'bucket/prefix/', got: %s", path)
	}

	client := s.Client().S3()
	if client == nil {
		return nil, fmt.Errorf("failed to get S3 client")
	}

	// Get bucket region
	region, err := s.getBucketRegion(ctx, client, bucket)
	if err != nil {
		return nil, err
	}

	// Use regional client for bucket operations
	regionalClient := s.Client().S3Regional(region)
	if regionalClient == nil {
		return nil, fmt.Errorf("failed to get regional S3 client for %s", region)
	}

	input := &s3.ListObjectsV2Input{
		Bucket:    &bucket,
		Delimiter: stringPtr("/"),
	}

	if prefix != "" {
		input.Prefix = &prefix
	}

	paginator := s3.NewListObjectsV2Paginator(regionalClient, input)

	var objects []AWSObject
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, aws.WrapAWSError(err, "list objects")
		}

		for _, obj := range output.Contents {
			objects = append(objects, objectToAWSObject(obj, bucket, region))
		}

		// Include common prefixes as folder objects
		for _, prefix := range output.CommonPrefixes {
			if prefix.Prefix != nil {
				objects = append(objects, folderToAWSObject(*prefix.Prefix, bucket, region))
			}
		}
	}

	return objects, nil
}

// Get retrieves a single S3 object metadata by path.
// Path format: "bucket/key"
func (s *S3Object) Get(ctx context.Context, path string) (AWSObject, error) {
	bucket, key, err := parseObjectPath(path)
	if err != nil {
		return nil, err
	}

	client := s.Client().S3()
	if client == nil {
		return nil, fmt.Errorf("failed to get S3 client")
	}

	input := &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	output, err := client.HeadObject(ctx, input)
	if err != nil {
		return nil, aws.WrapAWSError(err, "head object")
	}

	// Get bucket region
	region, err := s.getBucketRegion(ctx, client, bucket)
	if err != nil {
		return nil, err
	}

	// Convert HeadObject output to AWSObject
	return headObjectToAWSObject(output, bucket, key, region), nil
}

// Describe returns a formatted description of the S3 object.
func (s *S3Object) Describe(path string) (string, error) {
	obj, err := s.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Bucket: %s\n", extractBucketFromPath(path)))
	sb.WriteString(fmt.Sprintf("Key: %s\n", obj.GetID()))
	sb.WriteString(fmt.Sprintf("Name: %s\n", obj.GetName()))
	sb.WriteString(fmt.Sprintf("Region: %s\n", obj.GetRegion()))

	// Extract additional metadata from raw object
	if raw := obj.GetRaw(); raw != nil {
		if headOutput, ok := raw.(*s3.HeadObjectOutput); ok {
			if headOutput.ContentLength != nil {
				sb.WriteString(fmt.Sprintf("Size: %s\n", formatSize(*headOutput.ContentLength)))
			}
			if headOutput.ContentType != nil {
				sb.WriteString(fmt.Sprintf("Content-Type: %s\n", *headOutput.ContentType))
			}
			if headOutput.ETag != nil {
				sb.WriteString(fmt.Sprintf("ETag: %s\n", *headOutput.ETag))
			}
			if headOutput.StorageClass != "" {
				sb.WriteString(fmt.Sprintf("Storage Class: %s\n", headOutput.StorageClass))
			}
			if headOutput.ServerSideEncryption != "" {
				sb.WriteString(fmt.Sprintf("Encryption: %s\n", headOutput.ServerSideEncryption))
			}
		}
	}

	if obj.GetCreatedAt() != nil {
		sb.WriteString(fmt.Sprintf("Last Modified: %s\n", obj.GetCreatedAt().Format("2006-01-02 15:04:05")))
	}

	if len(obj.GetTags()) > 0 {
		sb.WriteString("Tags:\n")
		for k, v := range obj.GetTags() {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	return sb.String(), nil
}

// ToJSON returns a JSON representation of the S3 object.
func (s *S3Object) ToJSON(path string) (string, error) {
	obj, err := s.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(obj.GetRaw(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal object to JSON: %w", err)
	}

	return string(data), nil
}

// Delete deletes an S3 object or objects with a prefix.
func (s *S3Object) Delete(ctx context.Context, path string, force bool) error {
	bucket, key, err := parseObjectPath(path)
	if err != nil {
		return err
	}

	client := s.Client().S3()
	if client == nil {
		return fmt.Errorf("failed to get S3 client")
	}

	// Get bucket region for regional access
	region, err := s.getBucketRegion(ctx, client, bucket)
	if err != nil {
		return err
	}

	regionalClient := s.Client().S3Regional(region)
	if regionalClient == nil {
		return fmt.Errorf("failed to get regional S3 client for %s", region)
	}

	// If key ends with '/', delete all objects with this prefix
	if strings.HasSuffix(key, "/") {
		return s.deletePrefix(ctx, regionalClient, bucket, key, force)
	}

	// Delete single object
	input := &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	_, err = regionalClient.DeleteObject(ctx, input)
	if err != nil {
		return aws.WrapAWSError(err, "delete object")
	}

	return nil
}

// deletePrefix deletes all objects with the specified prefix.
func (s *S3Object) deletePrefix(ctx context.Context, client *s3.Client, bucket, prefix string, force bool) error {
	if !force {
		return fmt.Errorf("deleting prefix requires force=true")
	}

	// List all objects with prefix
	input := &s3.ListObjectsV2Input{
		Bucket: &bucket,
		Prefix: &prefix,
	}

	paginator := s3.NewListObjectsV2Paginator(client, input)

	var objectsToDelete []types.ObjectIdentifier
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return aws.WrapAWSError(err, "list objects for deletion")
		}

		for _, obj := range output.Contents {
			objectsToDelete = append(objectsToDelete, types.ObjectIdentifier{
				Key: obj.Key,
			})
		}
	}

	if len(objectsToDelete) == 0 {
		return nil
	}

	// Delete objects in batches of 1000 (S3 limit)
	const batchSize = 1000
	for i := 0; i < len(objectsToDelete); i += batchSize {
		end := i + batchSize
		if end > len(objectsToDelete) {
			end = len(objectsToDelete)
		}

		deleteInput := &s3.DeleteObjectsInput{
			Bucket: &bucket,
			Delete: &types.Delete{
				Objects: objectsToDelete[i:end],
				Quiet:   boolPtr(true),
			},
		}

		_, err := client.DeleteObjects(ctx, deleteInput)
		if err != nil {
			return aws.WrapAWSError(err, "delete objects")
		}
	}

	return nil
}

// Download downloads an S3 object to the provided writer.
func (s *S3Object) Download(ctx context.Context, bucket, key string, writer io.Writer) error {
	client := s.Client().S3()
	if client == nil {
		return fmt.Errorf("failed to get S3 client")
	}

	// Get bucket region for regional access
	region, err := s.getBucketRegion(ctx, client, bucket)
	if err != nil {
		return err
	}

	regionalClient := s.Client().S3Regional(region)
	if regionalClient == nil {
		return fmt.Errorf("failed to get regional S3 client for %s", region)
	}

	input := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	output, err := regionalClient.GetObject(ctx, input)
	if err != nil {
		return aws.WrapAWSError(err, "get object")
	}
	defer output.Body.Close()

	_, err = io.Copy(writer, output.Body)
	if err != nil {
		return fmt.Errorf("failed to write object data: %w", err)
	}

	return nil
}

// Upload uploads data from the reader to an S3 object.
func (s *S3Object) Upload(ctx context.Context, bucket, key string, reader io.Reader) error {
	client := s.Client().S3()
	if client == nil {
		return fmt.Errorf("failed to get S3 client")
	}

	input := &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   reader,
	}

	_, err := client.PutObject(ctx, input)
	if err != nil {
		return aws.WrapAWSError(err, "put object")
	}

	return nil
}

// GetPresignedURL generates a presigned URL for downloading an object.
func (s *S3Object) GetPresignedURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	client := s.Client().S3()
	if client == nil {
		return "", fmt.Errorf("failed to get S3 client")
	}

	presignClient := s3.NewPresignClient(client)

	input := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	presignedURL, err := presignClient.PresignGetObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", aws.WrapAWSError(err, "presign URL")
	}

	return presignedURL.URL, nil
}

// objectToAWSObject converts an S3 object to an AWSObject.
func objectToAWSObject(obj types.Object, bucket, region string) AWSObject {
	var key string
	if obj.Key != nil {
		key = *obj.Key
	}

	// Extract name from key (last path component)
	name := key
	if idx := strings.LastIndex(key, "/"); idx >= 0 {
		name = key[idx+1:]
	}

	arn := fmt.Sprintf("arn:aws:s3:::%s/%s", bucket, key)

	tags := make(map[string]string)

	return &BaseAWSObject{
		ARN:       arn,
		ID:        key,
		Name:      name,
		Region:    region,
		Tags:      tags,
		CreatedAt: obj.LastModified,
		Raw:       obj,
	}
}

// folderToAWSObject converts a common prefix (folder) to an AWSObject.
func folderToAWSObject(prefix, bucket, region string) AWSObject {
	// Extract name from prefix
	name := strings.TrimSuffix(prefix, "/")
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	arn := fmt.Sprintf("arn:aws:s3:::%s/%s", bucket, prefix)

	return &BaseAWSObject{
		ARN:    arn,
		ID:     prefix,
		Name:   name + "/",
		Region: region,
		Tags:   make(map[string]string),
		Raw:    prefix,
	}
}

// headObjectToAWSObject converts HeadObject output to AWSObject.
func headObjectToAWSObject(output *s3.HeadObjectOutput, bucket, key, region string) AWSObject {
	// Extract name from key
	name := key
	if idx := strings.LastIndex(key, "/"); idx >= 0 {
		name = key[idx+1:]
	}

	arn := fmt.Sprintf("arn:aws:s3:::%s/%s", bucket, key)

	tags := make(map[string]string)
	// HeadObject doesn't return tags directly, would need separate GetObjectTagging call

	return &BaseAWSObject{
		ARN:       arn,
		ID:        key,
		Name:      name,
		Region:    region,
		Tags:      tags,
		CreatedAt: output.LastModified,
		Raw:       output,
	}
}

// parseObjectPath parses a path in the format "bucket/key".
func parseObjectPath(path string) (bucket, key string, err error) {
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid path format, expected 'bucket/key', got: %s", path)
	}

	bucket = strings.TrimSpace(parts[0])
	key = strings.TrimSpace(parts[1])

	if bucket == "" || key == "" {
		return "", "", fmt.Errorf("bucket and key cannot be empty")
	}

	return bucket, key, nil
}

// parseListPath parses a list path in the format "bucket" or "bucket/prefix/".
func parseListPath(path string) (bucket, prefix string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", ""
	}

	parts := strings.SplitN(path, "/", 2)
	bucket = parts[0]
	if len(parts) == 2 {
		prefix = parts[1]
	}

	return bucket, prefix
}

// extractBucketFromPath extracts the bucket name from a path.
func extractBucketFromPath(path string) string {
	bucket, _ := parseListPath(path)
	return bucket
}

// getBucketRegion retrieves the region of a bucket.
func (s *S3Object) getBucketRegion(ctx context.Context, client *s3.Client, bucket string) (string, error) {
	input := &s3.GetBucketLocationInput{
		Bucket: &bucket,
	}

	output, err := client.GetBucketLocation(ctx, input)
	if err != nil {
		return "", aws.WrapAWSError(err, "get bucket location")
	}

	// If LocationConstraint is empty, bucket is in us-east-1
	if output.LocationConstraint == "" {
		return "us-east-1", nil
	}

	return string(output.LocationConstraint), nil
}

// formatSize formats a byte size into a human-readable string.
func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	return fmt.Sprintf("%.1f %s", float64(size)/float64(div), units[exp])
}

// Helper functions for pointer conversion
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
