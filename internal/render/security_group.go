package render

import (
	"fmt"
	"strings"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gdamore/tcell/v2"
)

// SecurityGroup renders security groups
type SecurityGroup struct {
	Base
}

// Header returns the security group header
func (sg *SecurityGroup) Header(region string) model1.Header {
	return model1.Header{
		{Name: "REGION"},
		{Name: "SG-ID"},
		{Name: "NAME"},
		{Name: "VPC-ID"},
		{Name: "DESCRIPTION", Attrs: model1.Attrs{Wide: true}},
		{Name: "INGRESS-RULES", Attrs: model1.Attrs{Capacity: true}},
		{Name: "EGRESS-RULES", Attrs: model1.Attrs{Capacity: true}},
		{Name: "VALID", Attrs: model1.Attrs{Wide: true}},
	}
}

// Render renders a security group to a row
func (sg *SecurityGroup) Render(o any, region string, row *model1.Row) error {
	obj, ok := o.(dao.AWSObject)
	if !ok {
		return fmt.Errorf("expected AWSObject, got %T", o)
	}

	group, ok := obj.GetRaw().(types.SecurityGroup)
	if !ok {
		return fmt.Errorf("expected types.SecurityGroup, got %T", obj.GetRaw())
	}

	row.ID = fmt.Sprintf("%s/%s", obj.GetRegion(), obj.GetID())
	row.Fields = model1.Fields{
		obj.GetRegion(),
		obj.GetID(),
		NA(StrPtrToStr(group.GroupName)),
		StrPtrToStr(group.VpcId),
		Truncate(StrPtrToStr(group.Description), 40),
		AsCount(len(group.IpPermissions)),
		AsCount(len(group.IpPermissionsEgress)),
		sg.validate(group),
	}
	return nil
}

// ColorerFunc returns the security group colorer
func (sg *SecurityGroup) ColorerFunc() model1.ColorerFunc {
	return func(region string, h model1.Header, re *model1.RowEvent) tcell.Color {
		validIdx, ok := h.IndexOf("VALID", true)
		if ok && validIdx < len(re.Row.Fields) && re.Row.Fields[validIdx] != "" {
			return model1.ErrColor // Has security issues
		}
		return model1.StdColor
	}
}

// validate checks security group for security issues
func (sg *SecurityGroup) validate(group types.SecurityGroup) string {
	var issues []string

	// Check for dangerous open ports from 0.0.0.0/0
	dangerousPorts := map[int32]string{
		22:    "ssh-open-world",
		3389:  "rdp-open-world",
		3306:  "mysql-open-world",
		5432:  "postgres-open-world",
		27017: "mongodb-open-world",
		6379:  "redis-open-world",
	}

	for _, perm := range group.IpPermissions {
		for _, ipRange := range perm.IpRanges {
			if ipRange.CidrIp != nil && *ipRange.CidrIp == "0.0.0.0/0" {
				if perm.FromPort != nil {
					if issue, ok := dangerousPorts[*perm.FromPort]; ok {
						issues = append(issues, issue)
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		return strings.Join(issues, ",")
	}
	return ""
}
