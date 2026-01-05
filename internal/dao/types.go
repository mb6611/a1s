package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/a1s/a1s/internal/aws"
)

// ResourceID identifies an AWS resource type (replaces Kubernetes GVR).
type ResourceID struct {
	Service  string // e.g., "ec2", "s3", "iam", "eks", "vpc"
	Resource string // e.g., "instance", "bucket", "user", "cluster"
}

// String returns a string representation in the form "service/resource".
func (r ResourceID) String() string {
	return fmt.Sprintf("%s/%s", r.Service, r.Resource)
}

// Parse parses a string in the form "service/resource" into a ResourceID.
func (r *ResourceID) Parse(s string) error {
	var service, resource string
	n, err := fmt.Sscanf(s, "%s/%s", &service, &resource)
	if err != nil || n != 2 {
		return fmt.Errorf("invalid resource ID format: %s (expected service/resource)", s)
	}
	r.Service = service
	r.Resource = resource
	return nil
}

// Predefined ResourceID variables for common AWS resources.
var (
	EC2InstanceRID       = ResourceID{Service: "ec2", Resource: "instance"}
	EC2VolumeRID         = ResourceID{Service: "ec2", Resource: "volume"}
	EC2SecurityGroupRID  = ResourceID{Service: "ec2", Resource: "securitygroup"}
	VPCResourceRID       = ResourceID{Service: "vpc", Resource: "vpc"}
	SubnetRID            = ResourceID{Service: "vpc", Resource: "subnet"}
	S3BucketRID          = ResourceID{Service: "s3", Resource: "bucket"}
	S3ObjectRID          = ResourceID{Service: "s3", Resource: "object"}
	IAMUserRID           = ResourceID{Service: "iam", Resource: "user"}
	IAMRoleRID           = ResourceID{Service: "iam", Resource: "role"}
	IAMPolicyRID         = ResourceID{Service: "iam", Resource: "policy"}
	EKSClusterRID        = ResourceID{Service: "eks", Resource: "cluster"}
	EKSNodeGroupRID      = ResourceID{Service: "eks", Resource: "nodegroup"}
)

// AWSObject represents a generic AWS resource with common metadata.
type AWSObject interface {
	GetARN() string
	GetID() string
	GetName() string
	GetRegion() string
	GetTags() map[string]string
	GetCreatedAt() *time.Time
	GetRaw() interface{}
}

// Factory provides AWS client configuration and management.
type Factory interface {
	Client() aws.Connection
	Profile() string
	Region() string
	SetProfile(profile string) error
	SetRegion(region string) error
}

// Getter retrieves a single AWS resource by path.
type Getter interface {
	Get(ctx context.Context, path string) (AWSObject, error)
}

// Lister retrieves multiple AWS resources from a region.
type Lister interface {
	List(ctx context.Context, region string) ([]AWSObject, error)
}

// Accessor combines getting and listing capabilities with initialization.
type Accessor interface {
	Getter
	Lister
	Init(Factory, *ResourceID)
	ResourceID() *ResourceID
}

// Describer provides formatted descriptions of AWS resources.
type Describer interface {
	Describe(path string) (string, error)
	ToJSON(path string) (string, error)
}

// Nuker provides deletion capabilities for AWS resources.
type Nuker interface {
	Delete(ctx context.Context, path string, force bool) error
}

// CloudFormationType maps ResourceID strings to CloudFormation type names for Cloud Control API.
var CloudFormationType = map[string]string{
	"ec2/instance":      "AWS::EC2::Instance",
	"ec2/volume":        "AWS::EC2::Volume",
	"vpc/securitygroup": "AWS::EC2::SecurityGroup",
	"vpc/vpc":           "AWS::EC2::VPC",
	"vpc/subnet":        "AWS::EC2::Subnet",
	"s3/bucket":         "AWS::S3::Bucket",
	"iam/user":          "AWS::IAM::User",
	"iam/role":          "AWS::IAM::Role",
	"iam/policy":        "AWS::IAM::ManagedPolicy",
	"eks/cluster":       "AWS::EKS::Cluster",
	"eks/nodegroup":     "AWS::EKS::Nodegroup",
}

// GetCloudFormationType returns the CloudFormation type name for a ResourceID.
// Returns the type name and true if found, or empty string and false if not supported.
func GetCloudFormationType(rid *ResourceID) (string, bool) {
	if rid == nil {
		return "", false
	}
	cfType, ok := CloudFormationType[rid.String()]
	return cfType, ok
}
