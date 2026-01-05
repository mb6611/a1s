// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package ui

import (
	"context"
	"errors"

	"github.com/a1s/a1s/internal/aws"
	"github.com/derailed/tcell/v2"
)

func init() {
	RegisterActions("ec2/instance", []ResourceAction{
		{
			Key:         KeyS,
			Name:        "Stop",
			Description: "Stop instance",
			Dangerous:   true,
			Handler: func(ctx context.Context, client aws.Connection, region, identifier string) error {
				ec2Client := client.EC2(region)
				if ec2Client == nil {
					return errors.New("failed to get EC2 client")
				}
				return aws.StopInstance(ctx, ec2Client, identifier)
			},
		},
		{
			Key:         tcell.KeyCtrlS,
			Name:        "Start",
			Description: "Start instance",
			Dangerous:   false,
			Handler: func(ctx context.Context, client aws.Connection, region, identifier string) error {
				ec2Client := client.EC2(region)
				if ec2Client == nil {
					return errors.New("failed to get EC2 client")
				}
				return aws.StartInstance(ctx, ec2Client, identifier)
			},
		},
		{
			Key:         tcell.KeyCtrlR,
			Name:        "Reboot",
			Description: "Reboot instance",
			Dangerous:   true,
			Handler: func(ctx context.Context, client aws.Connection, region, identifier string) error {
				ec2Client := client.EC2(region)
				if ec2Client == nil {
					return errors.New("failed to get EC2 client")
				}
				return aws.RebootInstance(ctx, ec2Client, identifier)
			},
		},
		{
			Key:         tcell.KeyCtrlD,
			Name:        "Terminate",
			Description: "Terminate instance",
			Dangerous:   true,
			Handler: func(ctx context.Context, client aws.Connection, region, identifier string) error {
				ec2Client := client.EC2(region)
				if ec2Client == nil {
					return errors.New("failed to get EC2 client")
				}
				return aws.TerminateInstance(ctx, ec2Client, identifier)
			},
		},
	})
}
