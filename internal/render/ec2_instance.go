package render

import (
	"fmt"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gdamore/tcell/v2"
)

// EC2Instance renders EC2 instances
type EC2Instance struct {
	Base
}

// Header returns the EC2 instance header
func (e *EC2Instance) Header(region string) model1.Header {
	return model1.Header{
		{Name: "REGION"},
		{Name: "INSTANCE-ID"},
		{Name: "NAME"},
		{Name: "TYPE"},
		{Name: "STATE"},
		{Name: "AZ", Attrs: model1.Attrs{Wide: true}},
		{Name: "PRIVATE-IP"},
		{Name: "PUBLIC-IP", Attrs: model1.Attrs{Wide: true}},
		{Name: "VPC-ID", Attrs: model1.Attrs{Wide: true}},
		{Name: "VALID", Attrs: model1.Attrs{Wide: true}},
		{Name: "AGE", Attrs: model1.Attrs{Time: true}},
	}
}

// Render renders an EC2 instance to a row
func (e *EC2Instance) Render(o any, region string, row *model1.Row) error {
	obj, ok := o.(dao.AWSObject)
	if !ok {
		return fmt.Errorf("expected AWSObject, got %T", o)
	}

	instance, ok := obj.GetRaw().(types.Instance)
	if !ok {
		return fmt.Errorf("expected types.Instance, got %T", obj.GetRaw())
	}

	row.ID = fmt.Sprintf("%s/%s", obj.GetRegion(), obj.GetID())
	row.Fields = model1.Fields{
		obj.GetRegion(),
		obj.GetID(),
		NA(obj.GetName()),
		string(instance.InstanceType),
		string(instance.State.Name),
		getAZ(instance),
		StrPtrToStr(instance.PrivateIpAddress),
		StrPtrToStr(instance.PublicIpAddress),
		StrPtrToStr(instance.VpcId),
		e.validate(instance),
		ToAge(obj.GetCreatedAt()),
	}
	return nil
}

// ColorerFunc returns the instance colorer
func (e *EC2Instance) ColorerFunc() model1.ColorerFunc {
	return func(region string, h model1.Header, re *model1.RowEvent) tcell.Color {
		stateIdx, ok := h.IndexOf("STATE", true)
		if !ok || stateIdx >= len(re.Row.Fields) {
			return model1.DefaultColorer(region, h, re)
		}

		state := re.Row.Fields[stateIdx]
		switch state {
		case "running":
			return model1.StdColor
		case "stopped":
			return model1.PendingColor
		case "pending", "shutting-down", "stopping":
			return model1.AddColor
		case "terminated":
			return model1.KillColor
		default:
			return model1.StdColor
		}
	}
}

// validate checks instance for security issues
func (e *EC2Instance) validate(instance types.Instance) string {
	var issues []string

	// Check for IMDSv1 (security concern)
	if instance.MetadataOptions != nil {
		if instance.MetadataOptions.HttpTokens == types.HttpTokensStateOptional {
			issues = append(issues, "imdsv1-enabled")
		}
	}

	// Check for public IP (informational)
	if instance.PublicIpAddress != nil && *instance.PublicIpAddress != "" {
		// Could flag this if needed
	}

	if len(issues) > 0 {
		return JoinStrings(",", issues...)
	}
	return ""
}

func getAZ(instance types.Instance) string {
	if instance.Placement != nil && instance.Placement.AvailabilityZone != nil {
		return *instance.Placement.AvailabilityZone
	}
	return NAValue
}
