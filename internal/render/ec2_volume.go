package render

import (
	"fmt"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gdamore/tcell/v2"
)

// EC2Volume renders EBS volumes
type EC2Volume struct {
	Base
}

// Header returns the EBS volume header
func (v *EC2Volume) Header(region string) model1.Header {
	return model1.Header{
		{Name: "REGION"},
		{Name: "VOLUME-ID"},
		{Name: "NAME"},
		{Name: "SIZE", Attrs: model1.Attrs{Capacity: true}},
		{Name: "TYPE"},
		{Name: "STATE"},
		{Name: "ENCRYPTED"},
		{Name: "ATTACHED-TO", Attrs: model1.Attrs{Wide: true}},
		{Name: "AZ", Attrs: model1.Attrs{Wide: true}},
		{Name: "VALID", Attrs: model1.Attrs{Wide: true}},
		{Name: "AGE", Attrs: model1.Attrs{Time: true}},
	}
}

// Render renders an EBS volume to a row
func (v *EC2Volume) Render(o any, region string, row *model1.Row) error {
	obj, ok := o.(dao.AWSObject)
	if !ok {
		return fmt.Errorf("expected AWSObject, got %T", o)
	}

	volume, ok := obj.GetRaw().(types.Volume)
	if !ok {
		return fmt.Errorf("expected types.Volume, got %T", obj.GetRaw())
	}

	row.ID = fmt.Sprintf("%s/%s", obj.GetRegion(), obj.GetID())
	row.Fields = model1.Fields{
		obj.GetRegion(),
		obj.GetID(),
		NA(obj.GetName()),
		formatVolumeSize(volume.Size),
		string(volume.VolumeType),
		string(volume.State),
		BoolPtrToYesNo(volume.Encrypted),
		getAttachedInstance(volume),
		StrPtrToStr(volume.AvailabilityZone),
		v.validate(volume),
		ToAge(obj.GetCreatedAt()),
	}
	return nil
}

// ColorerFunc returns the volume colorer
func (v *EC2Volume) ColorerFunc() model1.ColorerFunc {
	return func(region string, h model1.Header, re *model1.RowEvent) tcell.Color {
		stateIdx, ok := h.IndexOf("STATE", true)
		if !ok || stateIdx >= len(re.Row.Fields) {
			return model1.DefaultColorer(region, h, re)
		}

		state := re.Row.Fields[stateIdx]
		switch state {
		case "available", "in-use":
			return model1.StdColor
		case "creating", "deleting":
			return model1.AddColor
		case "error":
			return model1.ErrColor
		default:
			return model1.StdColor
		}
	}
}

// validate checks volume for security issues
func (v *EC2Volume) validate(volume types.Volume) string {
	var issues []string

	// Check for unencrypted volume
	if volume.Encrypted != nil && !*volume.Encrypted {
		issues = append(issues, "unencrypted")
	}

	if len(issues) > 0 {
		return JoinStrings(",", issues...)
	}
	return ""
}

func formatVolumeSize(size *int32) string {
	if size == nil {
		return NAValue
	}
	return fmt.Sprintf("%d GiB", *size)
}

func getAttachedInstance(volume types.Volume) string {
	if len(volume.Attachments) == 0 {
		return NAValue
	}
	if volume.Attachments[0].InstanceId != nil {
		return *volume.Attachments[0].InstanceId
	}
	return NAValue
}
