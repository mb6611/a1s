package render

import (
	"fmt"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gdamore/tcell/v2"
)

// VPC renders VPCs
type VPC struct {
	Base
}

// Header returns the VPC header
func (v *VPC) Header(region string) model1.Header {
	return model1.Header{
		{Name: "REGION"},
		{Name: "VPC-ID"},
		{Name: "NAME"},
		{Name: "CIDR"},
		{Name: "STATE"},
		{Name: "DEFAULT"},
		{Name: "TENANCY", Attrs: model1.Attrs{Wide: true}},
		{Name: "AGE", Attrs: model1.Attrs{Time: true}},
	}
}

// Render renders a VPC to a row
func (v *VPC) Render(o any, region string, row *model1.Row) error {
	obj, ok := o.(dao.AWSObject)
	if !ok {
		return fmt.Errorf("expected AWSObject, got %T", o)
	}

	vpc, ok := obj.GetRaw().(types.Vpc)
	if !ok {
		return fmt.Errorf("expected types.Vpc, got %T", obj.GetRaw())
	}

	row.ID = fmt.Sprintf("%s/%s", obj.GetRegion(), obj.GetID())
	row.Fields = model1.Fields{
		obj.GetRegion(),
		obj.GetID(),
		NA(obj.GetName()),
		StrPtrToStr(vpc.CidrBlock),
		string(vpc.State),
		BoolPtrToYesNo(vpc.IsDefault),
		string(vpc.InstanceTenancy),
		ToAge(obj.GetCreatedAt()),
	}
	return nil
}

// ColorerFunc returns the VPC colorer
func (v *VPC) ColorerFunc() model1.ColorerFunc {
	return func(region string, h model1.Header, re *model1.RowEvent) tcell.Color {
		stateIdx, ok := h.IndexOf("STATE", true)
		if !ok || stateIdx >= len(re.Row.Fields) {
			return model1.StdColor
		}

		state := re.Row.Fields[stateIdx]
		switch state {
		case "available":
			return model1.StdColor
		case "pending":
			return model1.AddColor
		default:
			return model1.StdColor
		}
	}
}
