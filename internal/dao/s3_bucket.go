package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsinternal "github.com/a1s/a1s/internal/aws"
)

func init() {
	RegisterAccessor(&S3BucketRID, &S3Bucket{})
}

// S3Bucket is the DAO for S3 buckets.
type S3Bucket struct {
	AWSResource
}

// List returns S3 buckets, filtered by region if specified.
func (s *S3Bucket) List(ctx context.Context, region string) ([]AWSObject, error) {
	client := s.Client().S3()
	if client == nil {
		return nil, fmt.Errorf("failed to get S3 client")
	}

	input := &s3.ListBucketsInput{}
	output, err := client.ListBuckets(ctx, input)
	if err != nil {
		return nil, awsinternal.WrapAWSError(err, "list buckets")
	}

	// Normalize region filter
	filterByRegion := region != "" && region != "all" && region != "*" && region != awsinternal.RegionAll

	var buckets []AWSObject
	for _, bucket := range output.Buckets {
		// Get bucket location for each bucket
		location := ""
		if bucket.Name != nil {
			loc, err := s.GetLocation(ctx, *bucket.Name)
			if err == nil {
				location = loc
			}
		}

		// Filter by region if specified
		if filterByRegion && location != region {
			continue
		}

		buckets = append(buckets, bucketToAWSObject(bucket, location))
	}

	return buckets, nil
}

// Get retrieves a single S3 bucket by path (bucket name).
func (s *S3Bucket) Get(ctx context.Context, path string) (AWSObject, error) {
	bucketName := parseBucketPath(path)
	if bucketName == "" {
		return nil, fmt.Errorf("invalid bucket path: %s", path)
	}

	client := s.Client().S3()
	if client == nil {
		return nil, fmt.Errorf("failed to get S3 client")
	}

	// Verify bucket exists using HeadBucket
	headInput := &s3.HeadBucketInput{
		Bucket: &bucketName,
	}

	_, err := client.HeadBucket(ctx, headInput)
	if err != nil {
		return nil, awsinternal.WrapAWSError(err, "head bucket")
	}

	// Get bucket location
	location, err := s.GetLocation(ctx, bucketName)
	if err != nil {
		return nil, err
	}

	// Construct a bucket object (we don't have creation date from HeadBucket)
	bucket := types.Bucket{
		Name: &bucketName,
	}

	return bucketToAWSObject(bucket, location), nil
}

// Describe returns a formatted description of the S3 bucket.
func (s *S3Bucket) Describe(path string) (string, error) {
	obj, err := s.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	bucketName := parseBucketPath(path)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Bucket Name: %s\n", obj.GetName()))
	sb.WriteString(fmt.Sprintf("Region: %s\n", obj.GetRegion()))

	if obj.GetCreatedAt() != nil {
		sb.WriteString(fmt.Sprintf("Created: %s\n", obj.GetCreatedAt().Format("2006-01-02 15:04:05")))
	}

	// Get versioning status
	versioning, err := s.GetVersioning(context.Background(), bucketName)
	if err == nil {
		sb.WriteString(fmt.Sprintf("Versioning: %s\n", versioning))
	}

	// Get bucket policy (if any)
	policy, err := s.GetPolicy(context.Background(), bucketName)
	if err == nil && policy != "" {
		sb.WriteString(fmt.Sprintf("Policy: %s\n", policy))
	}

	if obj.GetARN() != "" {
		sb.WriteString(fmt.Sprintf("ARN: %s\n", obj.GetARN()))
	}

	return sb.String(), nil
}

// ToJSON returns a JSON representation of the S3 bucket.
func (s *S3Bucket) ToJSON(path string) (string, error) {
	obj, err := s.Get(context.Background(), path)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(obj.GetRaw(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal bucket to JSON: %w", err)
	}

	return string(data), nil
}

// Delete deletes an S3 bucket. If force is true, empties the bucket first.
func (s *S3Bucket) Delete(ctx context.Context, path string, force bool) error {
	bucketName := parseBucketPath(path)
	if bucketName == "" {
		return fmt.Errorf("invalid bucket path: %s", path)
	}

	client := s.Client().S3()
	if client == nil {
		return fmt.Errorf("failed to get S3 client")
	}

	// If force, empty the bucket first
	if force {
		if err := s.emptyBucket(ctx, client, bucketName); err != nil {
			return fmt.Errorf("failed to empty bucket: %w", err)
		}
	}

	// Delete the bucket
	deleteInput := &s3.DeleteBucketInput{
		Bucket: &bucketName,
	}

	_, err := client.DeleteBucket(ctx, deleteInput)
	if err != nil {
		return awsinternal.WrapAWSError(err, "delete bucket")
	}

	return nil
}

// emptyBucket deletes all objects in a bucket.
func (s *S3Bucket) emptyBucket(ctx context.Context, client *s3.Client, bucketName string) error {
	// List and delete all objects
	listInput := &s3.ListObjectsV2Input{
		Bucket: &bucketName,
	}

	paginator := s3.NewListObjectsV2Paginator(client, listInput)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return awsinternal.WrapAWSError(err, "list objects")
		}

		if len(output.Contents) == 0 {
			continue
		}

		// Build delete request
		var objectIdentifiers []types.ObjectIdentifier
		for _, obj := range output.Contents {
			objectIdentifiers = append(objectIdentifiers, types.ObjectIdentifier{
				Key: obj.Key,
			})
		}

		deleteInput := &s3.DeleteObjectsInput{
			Bucket: &bucketName,
			Delete: &types.Delete{
				Objects: objectIdentifiers,
				Quiet:   boolPtr(true),
			},
		}

		_, err = client.DeleteObjects(ctx, deleteInput)
		if err != nil {
			return awsinternal.WrapAWSError(err, "delete objects")
		}
	}

	return nil
}

// GetLocation returns the region where the bucket is located.
func (s *S3Bucket) GetLocation(ctx context.Context, bucket string) (string, error) {
	client := s.Client().S3()
	if client == nil {
		return "", fmt.Errorf("failed to get S3 client")
	}

	input := &s3.GetBucketLocationInput{
		Bucket: &bucket,
	}

	output, err := client.GetBucketLocation(ctx, input)
	if err != nil {
		return "", awsinternal.WrapAWSError(err, "get bucket location")
	}

	// AWS returns empty string for us-east-1
	if output.LocationConstraint == "" {
		return "us-east-1", nil
	}

	return string(output.LocationConstraint), nil
}

// GetPolicy returns the bucket policy as a JSON string.
func (s *S3Bucket) GetPolicy(ctx context.Context, bucket string) (string, error) {
	client := s.Client().S3()
	if client == nil {
		return "", fmt.Errorf("failed to get S3 client")
	}

	input := &s3.GetBucketPolicyInput{
		Bucket: &bucket,
	}

	output, err := client.GetBucketPolicy(ctx, input)
	if err != nil {
		// Check if error is NoSuchBucketPolicy (not an error, just no policy)
		return "", awsinternal.WrapAWSError(err, "get bucket policy")
	}

	if output.Policy == nil {
		return "", nil
	}

	return *output.Policy, nil
}

// GetVersioning returns the versioning status of the bucket.
func (s *S3Bucket) GetVersioning(ctx context.Context, bucket string) (string, error) {
	client := s.Client().S3()
	if client == nil {
		return "", fmt.Errorf("failed to get S3 client")
	}

	input := &s3.GetBucketVersioningInput{
		Bucket: &bucket,
	}

	output, err := client.GetBucketVersioning(ctx, input)
	if err != nil {
		return "", awsinternal.WrapAWSError(err, "get bucket versioning")
	}

	if output.Status == "" {
		return "Disabled", nil
	}

	return string(output.Status), nil
}

// SetVersioning enables or disables versioning for the bucket.
func (s *S3Bucket) SetVersioning(ctx context.Context, bucket string, enabled bool) error {
	client := s.Client().S3()
	if client == nil {
		return fmt.Errorf("failed to get S3 client")
	}

	status := types.BucketVersioningStatusSuspended
	if enabled {
		status = types.BucketVersioningStatusEnabled
	}

	input := &s3.PutBucketVersioningInput{
		Bucket: &bucket,
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: status,
		},
	}

	_, err := client.PutBucketVersioning(ctx, input)
	if err != nil {
		return awsinternal.WrapAWSError(err, "put bucket versioning")
	}

	return nil
}

// bucketToAWSObject converts an S3 bucket to an AWSObject.
func bucketToAWSObject(bucket types.Bucket, location string) AWSObject {
	var arn string
	var name string

	if bucket.Name != nil {
		name = *bucket.Name
		// ARN format: arn:aws:s3:::bucket-name
		arn = fmt.Sprintf("arn:aws:s3:::%s", name)
	}

	return &BaseAWSObject{
		ARN:       arn,
		ID:        name,
		Name:      name,
		Region:    location,
		Tags:      make(map[string]string),
		CreatedAt: bucket.CreationDate,
		Raw:       bucket,
	}
}

// parseBucketPath extracts the bucket name from a path (just returns the bucket name).
func parseBucketPath(path string) string {
	return strings.TrimSpace(path)
}
