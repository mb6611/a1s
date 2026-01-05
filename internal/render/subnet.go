package render

import (
	"fmt"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gdamore/tcell/v2"
)

// Subnet renders subnets
type Subnet struct {
	Base
}

// Header returns the subnet header
func (s *Subnet) Header(region string) model1.Header {
	return model1.Header{
		{Name: "REGION"},
		{Name: "SUBNET-ID"},
		{Name: "NAME"},
		{Name: "VPC-ID"},
		{Name: "CIDR"},
		{Name: "AZ"},
		{Name: "STATE"},
		{Name: "AVAILABLE-IPS", Attrs: model1.Attrs{Capacity: true}},
		{Name: "PUBLIC", Attrs: model1.Attrs{Wide: true}},
		{Name: "AGE", Attrs: model1.Attrs{Time: true}},
	}
}

// Render renders a subnet to a row
func (s *Subnet) Render(o any, region string, row *model1.Row) error {
	obj, ok := o.(dao.AWSObject)
	if !ok {
		return fmt.Errorf("expected AWSObject, got %T", o)
	}

	subnet, ok := obj.GetRaw().(types.Subnet)
	if !ok {
		return fmt.Errorf("expected types.Subnet, got %T", obj.GetRaw())
	}

	row.ID = fmt.Sprintf("%s/%s", obj.GetRegion(), obj.GetID())
	row.Fields = model1.Fields{
		obj.GetRegion(),
		obj.GetID(),
		NA(obj.GetName()),
		StrPtrToStr(subnet.VpcId),
		StrPtrToStr(subnet.CidrBlock),
		StrPtrToStr(subnet.AvailabilityZone),
		string(subnet.State),
		Int32PtrToStr(subnet.AvailableIpAddressCount),
		BoolPtrToYesNo(subnet.MapPublicIpOnLaunch),
		ToAge(obj.GetCreatedAt()),
	}
	return nil
}

// ColorerFunc returns the subnet colorer
func (s *Subnet) ColorerFunc() model1.ColorerFunc {
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
