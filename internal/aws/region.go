package aws

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	RegionAll     = "all"
	DefaultRegion = "us-east-1"
	GlobalRegion  = "global"
)

type RegionInfo struct {
	Name        string
	Description string
	OptInStatus string
	Enabled     bool
}

type RegionManager struct {
	regions     map[string]*RegionInfo
	regionList  []string
	lastRefresh time.Time
	mx          sync.RWMutex
}

var GlobalServices = map[string]bool{
	"iam":     true,
	"s3":      true,
	"route53": true,
}

// NewRegionManager creates a new RegionManager with hardcoded fallback regions.
func NewRegionManager() *RegionManager {
	m := &RegionManager{
		regions:    make(map[string]*RegionInfo),
		regionList: make([]string, 0),
	}

	// Initialize with hardcoded list of common AWS regions as fallback
	fallbackRegions := []string{
		"us-east-1",
		"us-east-2",
		"us-west-1",
		"us-west-2",
		"eu-west-1",
		"eu-west-2",
		"eu-central-1",
		"ap-northeast-1",
		"ap-northeast-2",
		"ap-southeast-1",
		"ap-southeast-2",
		"ap-south-1",
		"ca-central-1",
		"sa-east-1",
		"af-south-1",
		"eu-north-1",
		"me-south-1",
		"ap-east-1",
	}

	for _, region := range fallbackRegions {
		m.regions[region] = &RegionInfo{
			Name:    region,
			Enabled: true,
		}
		m.regionList = append(m.regionList, region)
	}

	return m
}

// DiscoverRegions calls the EC2 DescribeRegions API and updates the internal region list.
func (m *RegionManager) DiscoverRegions(ctx context.Context, ec2Client *ec2.Client) error {
	if ec2Client == nil {
		return errors.New("ec2Client cannot be nil")
	}

	result, err := ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{
		AllRegions: nil,
	})
	if err != nil {
		return fmt.Errorf("failed to describe regions: %w", err)
	}

	m.mx.Lock()
	defer m.mx.Unlock()

	// Clear existing regions and rebuild from API response
	m.regions = make(map[string]*RegionInfo)
	m.regionList = make([]string, 0)

	for _, region := range result.Regions {
		if region.RegionName == nil {
			return errors.New("region name must not be nil in DescribeRegions response")
		}

		regionName := *region.RegionName

		optInStatus := ""
		if region.OptInStatus != nil {
			optInStatus = *region.OptInStatus
		}

		enabled := region.OptInStatus == nil || *region.OptInStatus != "not-opted-in"

		m.regions[regionName] = &RegionInfo{
			Name:        regionName,
			Description: regionName,
			OptInStatus: optInStatus,
			Enabled:     enabled,
		}
		m.regionList = append(m.regionList, regionName)
	}

	m.lastRefresh = time.Now()

	return nil
}

// ListRegions returns a list of all known regions.
func (m *RegionManager) ListRegions() []string {
	m.mx.RLock()
	defer m.mx.RUnlock()

	result := make([]string, len(m.regionList))
	copy(result, m.regionList)
	return result
}

// GetRegion retrieves detailed information about a specific region.
func (m *RegionManager) GetRegion(name string) (*RegionInfo, error) {
	m.mx.RLock()
	defer m.mx.RUnlock()

	if region, exists := m.regions[name]; exists {
		// Return a copy to prevent external mutation
		return &RegionInfo{
			Name:        region.Name,
			Description: region.Description,
			OptInStatus: region.OptInStatus,
			Enabled:     region.Enabled,
		}, nil
	}

	return nil, fmt.Errorf("region %q not found", name)
}

// ValidateRegion checks if a region is valid. Special regions "all" and "global" are always valid.
func (m *RegionManager) ValidateRegion(region string) error {
	if region == RegionAll || region == GlobalRegion {
		return nil
	}

	m.mx.RLock()
	defer m.mx.RUnlock()

	if _, exists := m.regions[region]; !exists {
		return fmt.Errorf("invalid region: %q", region)
	}

	return nil
}

// IsGlobalService checks if a service is a global service.
func IsGlobalService(service string) bool {
	return GlobalServices[service]
}

// ResolveRegion determines the appropriate region to use based on service and requested region.
// Logic:
// - If service is global (IAM, S3, Route53), return "us-east-1"
// - If requestedRegion is "all", return requestedRegion (handled elsewhere)
// - If requestedRegion is empty, return defaultRegion
// - Otherwise return requestedRegion
func ResolveRegion(service, requestedRegion, defaultRegion string) string {
	if IsGlobalService(service) {
		return DefaultRegion
	}

	if requestedRegion == RegionAll {
		return requestedRegion
	}

	if requestedRegion == "" {
		return defaultRegion
	}

	return requestedRegion
}

