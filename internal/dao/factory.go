// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package dao

import (
	"github.com/a1s/a1s/internal/aws"
)

// AWSFactory implements the Factory interface using an APIClient.
type AWSFactory struct {
	client  aws.Connection
	profile string
	region  string
}

// NewFactory creates a new AWSFactory with the given client.
func NewFactory(client aws.Connection) *AWSFactory {
	profile := ""
	region := ""
	if client != nil {
		profile = client.ActiveProfile()
		region = client.ActiveRegion()
	}
	return &AWSFactory{
		client:  client,
		profile: profile,
		region:  region,
	}
}

// Client returns the AWS connection.
func (f *AWSFactory) Client() aws.Connection {
	return f.client
}

// Profile returns the current AWS profile.
func (f *AWSFactory) Profile() string {
	if f.client != nil {
		return f.client.ActiveProfile()
	}
	return f.profile
}

// Region returns the current AWS region.
func (f *AWSFactory) Region() string {
	if f.client != nil {
		return f.client.ActiveRegion()
	}
	return f.region
}

// SetProfile switches to a different AWS profile.
func (f *AWSFactory) SetProfile(profile string) error {
	if f.client == nil {
		return aws.ErrNoConnection
	}
	err := f.client.SwitchProfile(profile)
	if err == nil {
		f.profile = profile
	}
	return err
}

// SetRegion switches to a different AWS region.
func (f *AWSFactory) SetRegion(region string) error {
	if f.client == nil {
		return aws.ErrNoConnection
	}
	err := f.client.SwitchRegion(region)
	if err == nil {
		f.region = region
	}
	return err
}
