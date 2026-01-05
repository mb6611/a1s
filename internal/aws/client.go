package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
)

type Error string

const (
	ErrNoCredentials      = Error("no AWS credentials found")
	ErrExpiredCredentials = Error("AWS credentials have expired")
	ErrNoConnection       = Error("no connection to AWS")
	ErrInvalidProfile     = Error("invalid AWS profile")
	ErrInvalidRegion      = Error("invalid AWS region")
)

func (e Error) Error() string {
	return string(e)
}

type Connection interface {
	Config() *ClientConfig
	ConnectionOK() bool
	CheckConnectivity() bool
	SwitchProfile(profile string) error
	SwitchRegion(region string) error
	ActiveProfile() string
	ActiveRegion() string
	AccountID() string
	ProfileNames() []string
	ProfileRegion(profile string) string
	EC2(region string) *ec2.Client
	S3() *s3.Client
	S3Regional(region string) *s3.Client
	IAM() *iam.Client
	EKS(region string) *eks.Client
	STS(region string) *sts.Client
	CloudControl(region string) *cloudcontrol.Client
	CloudFormation(region string) *cloudformation.Client
}

type ClientConfig struct {
	Profile string
	Region  string
	Timeout time.Duration
}

type ServiceClients struct {
	ec2Client            *ec2.Client
	s3Client             *s3.Client
	iamClient            *iam.Client
	eksClient            *eks.Client
	stsClient            *sts.Client
	cloudcontrolClient   *cloudcontrol.Client
	cloudformationClient *cloudformation.Client
	awsConfig            aws.Config
	createdAt            time.Time
}

type APIClient struct {
	config    *ClientConfig
	settings  ProfileSettings
	clients   map[string]*ServiceClients
	accountID string
	connOK    bool
	mx        sync.RWMutex
}

// NewAPIClient creates a new APIClient instance with the provided settings and configuration.
func NewAPIClient(settings ProfileSettings, cfg *ClientConfig) (*APIClient, error) {
	if settings == nil {
		return nil, errors.New("settings cannot be nil")
	}
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}

	client := &APIClient{
		config:   cfg,
		settings: settings,
		clients:  make(map[string]*ServiceClients),
	}

	return client, nil
}

// InitConnection is a convenience function that creates a ProfileManager and then an APIClient.
// It uses the current profile and region from the ProfileManager.
func InitConnection(settings ProfileSettings, cfg *ClientConfig) (*APIClient, error) {
	client, err := NewAPIClient(settings, cfg)
	if err != nil {
		return nil, err
	}

	// Verify connectivity on initialization
	if !client.CheckConnectivity() {
		return nil, ErrNoConnection
	}

	return client, nil
}

// Config returns the client configuration.
func (c *APIClient) Config() *ClientConfig {
	c.mx.RLock()
	defer c.mx.RUnlock()
	// Return a copy
	return &ClientConfig{
		Profile: c.config.Profile,
		Region:  c.config.Region,
		Timeout: c.config.Timeout,
	}
}

// ConnectionOK returns whether the connection to AWS is established and working.
func (c *APIClient) ConnectionOK() bool {
	c.mx.RLock()
	defer c.mx.RUnlock()
	return c.connOK
}

// CheckConnectivity verifies connectivity to AWS by calling STS GetCallerIdentity.
// It caches the account ID on success.
func (c *APIClient) CheckConnectivity() bool {
	ctx := context.Background()
	if c.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.Timeout)
		defer cancel()
	}

	stsClient := c.STS(c.config.Region)
	if stsClient == nil {
		c.mx.Lock()
		c.connOK = false
		c.mx.Unlock()
		return false
	}

	result, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		c.mx.Lock()
		c.connOK = false
		c.mx.Unlock()
		return false
	}

	c.mx.Lock()
	c.connOK = true
	if result.Account != nil {
		c.accountID = *result.Account
	}
	c.mx.Unlock()

	return true
}

// SwitchProfile switches to a new AWS profile and invalidates cached clients for the old profile.
func (c *APIClient) SwitchProfile(profile string) error {
	// Verify profile exists
	_, err := c.settings.GetProfile(profile)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidProfile, profile)
	}

	c.mx.Lock()
	defer c.mx.Unlock()

	oldProfile := c.config.Profile

	// Invalidate all clients for the old profile
	for key := range c.clients {
		if strings.HasPrefix(key, oldProfile+":") {
			delete(c.clients, key)
		}
	}

	// Update configuration
	c.config.Profile = profile
	c.connOK = false
	c.accountID = ""

	return nil
}

// SwitchRegion switches to a new AWS region.
// Clients are lazily created per region, so this just updates the active region.
func (c *APIClient) SwitchRegion(region string) error {
	if region == "" {
		return fmt.Errorf("%w: region cannot be empty", ErrInvalidRegion)
	}

	c.mx.Lock()
	defer c.mx.Unlock()

	c.config.Region = region
	return nil
}

// ActiveProfile returns the currently active AWS profile.
func (c *APIClient) ActiveProfile() string {
	c.mx.RLock()
	defer c.mx.RUnlock()
	return c.config.Profile
}

// ActiveRegion returns the currently active AWS region.
func (c *APIClient) ActiveRegion() string {
	c.mx.RLock()
	defer c.mx.RUnlock()
	return c.config.Region
}

// AccountID returns the cached AWS account ID.
func (c *APIClient) AccountID() string {
	c.mx.RLock()
	defer c.mx.RUnlock()
	return c.accountID
}

// ProfileNames returns all available profile names.
func (c *APIClient) ProfileNames() []string {
	if c.settings == nil {
		return nil
	}
	names, err := c.settings.ProfileNames()
	if err != nil {
		return nil
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	return result
}

// ProfileRegion returns the default region for a profile.
func (c *APIClient) ProfileRegion(profile string) string {
	if c.settings == nil {
		return ""
	}
	p, err := c.settings.GetProfile(profile)
	if err != nil || p == nil {
		return ""
	}
	return p.DefaultRegion
}

// EC2 returns an EC2 client for the specified region.
func (c *APIClient) EC2(region string) *ec2.Client {
	clients, err := c.getClients(region)
	if err != nil {
		return nil
	}
	return clients.ec2Client
}

// S3 returns an S3 client (uses us-east-1 for bucket listing).
func (c *APIClient) S3() *s3.Client {
	clients, err := c.getClients(DefaultRegion)
	if err != nil {
		return nil
	}
	return clients.s3Client
}

// S3Regional returns an S3 client for a specific region.
// Use this for bucket operations when the bucket is in a different region.
func (c *APIClient) S3Regional(region string) *s3.Client {
	if region == "" {
		region = DefaultRegion
	}
	clients, err := c.getClients(region)
	if err != nil {
		return nil
	}
	return clients.s3Client
}

// IAM returns an IAM client (uses us-east-1 as IAM is a global service).
func (c *APIClient) IAM() *iam.Client {
	clients, err := c.getClients(DefaultRegion)
	if err != nil {
		return nil
	}
	return clients.iamClient
}

// EKS returns an EKS client for the specified region.
func (c *APIClient) EKS(region string) *eks.Client {
	clients, err := c.getClients(region)
	if err != nil {
		return nil
	}
	return clients.eksClient
}

// STS returns an STS client for the specified region.
func (c *APIClient) STS(region string) *sts.Client {
	clients, err := c.getClients(region)
	if err != nil {
		return nil
	}
	return clients.stsClient
}

// CloudControl returns a CloudControl client for the specified region.
func (c *APIClient) CloudControl(region string) *cloudcontrol.Client {
	clients, err := c.getClients(region)
	if err != nil {
		return nil
	}
	return clients.cloudcontrolClient
}

// CloudFormation returns a CloudFormation client for the specified region.
func (c *APIClient) CloudFormation(region string) *cloudformation.Client {
	clients, err := c.getClients(region)
	if err != nil {
		return nil
	}
	return clients.cloudformationClient
}

// Reset clears all cached clients and resets connection state.
func (c *APIClient) Reset() {
	c.mx.Lock()
	defer c.mx.Unlock()

	c.clients = make(map[string]*ServiceClients)
	c.connOK = false
	c.accountID = ""
}

// getClients retrieves or creates service clients for the specified region.
// Uses the Read-Lock-Upgrade pattern for thread safety.
func (c *APIClient) getClients(region string) (*ServiceClients, error) {
	// Fast path: read lock
	c.mx.RLock()
	key := c.config.Profile + ":" + region
	if clients, ok := c.clients[key]; ok {
		c.mx.RUnlock()
		return clients, nil
	}
	c.mx.RUnlock()

	// Slow path: write lock
	c.mx.Lock()
	defer c.mx.Unlock()

	// Re-read key after acquiring write lock (profile may have changed)
	key = c.config.Profile + ":" + region

	// Double-check after acquiring write lock
	if clients, ok := c.clients[key]; ok {
		return clients, nil
	}

	clients, err := c.createClients(c.config.Profile, region)
	if err != nil {
		return nil, err
	}
	c.clients[key] = clients
	return clients, nil
}

// createClients creates a new set of service clients for the specified profile and region.
func (c *APIClient) createClients(profile, region string) (*ServiceClients, error) {
	ctx := context.Background()
	if c.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.Timeout)
		defer cancel()
	}

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithSharedConfigProfile(profile),
	)
	if err != nil {
		return nil, WrapAWSError(err, "load AWS config")
	}

	clients := &ServiceClients{
		awsConfig: cfg,
		createdAt: time.Now(),
	}

	// Create service clients
	clients.ec2Client = ec2.NewFromConfig(cfg)
	clients.s3Client = s3.NewFromConfig(cfg)
	clients.iamClient = iam.NewFromConfig(cfg)
	clients.eksClient = eks.NewFromConfig(cfg)
	clients.stsClient = sts.NewFromConfig(cfg)
	clients.cloudcontrolClient = cloudcontrol.NewFromConfig(cfg)
	clients.cloudformationClient = cloudformation.NewFromConfig(cfg)

	return clients, nil
}

// WrapAWSError wraps AWS SDK errors with additional context.
func WrapAWSError(err error, operation string) error {
	if err == nil {
		return nil
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "AccessDenied", "AccessDeniedException":
			return fmt.Errorf("access denied for %s: %w", operation, err)
		case "ExpiredToken", "ExpiredTokenException":
			return fmt.Errorf("%w: %s", ErrExpiredCredentials, operation)
		case "ThrottlingException":
			return fmt.Errorf("rate limited during %s: %w", operation, err)
		case "InvalidClientTokenId":
			return fmt.Errorf("%w: %s", ErrNoCredentials, operation)
		default:
			return fmt.Errorf("%s failed: %s (%s)", operation, apiErr.ErrorMessage(), apiErr.ErrorCode())
		}
	}

	return fmt.Errorf("%s failed: %w", operation, err)
}
