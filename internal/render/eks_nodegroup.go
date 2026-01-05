package render

import (
	"fmt"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/gdamore/tcell/v2"
)

// EKSNodeGroup renders EKS node groups
type EKSNodeGroup struct {
	Base
}

// Header returns the EKS node group header
func (n *EKSNodeGroup) Header(region string) model1.Header {
	return model1.Header{
		{Name: "REGION"},
		{Name: "CLUSTER"},
		{Name: "NODEGROUP"},
		{Name: "STATUS"},
		{Name: "INSTANCE-TYPES", Attrs: model1.Attrs{Wide: true}},
		{Name: "DESIRED", Attrs: model1.Attrs{Capacity: true}},
		{Name: "MIN", Attrs: model1.Attrs{Capacity: true}},
		{Name: "MAX", Attrs: model1.Attrs{Capacity: true}},
		{Name: "AMI-TYPE", Attrs: model1.Attrs{Wide: true}},
		{Name: "AGE", Attrs: model1.Attrs{Time: true}},
	}
}

// Render renders an EKS node group to a row
func (n *EKSNodeGroup) Render(o any, region string, row *model1.Row) error {
	obj, ok := o.(dao.AWSObject)
	if !ok {
		return fmt.Errorf("expected AWSObject, got %T", o)
	}

	nodegroup, ok := obj.GetRaw().(types.Nodegroup)
	if !ok {
		return fmt.Errorf("expected types.Nodegroup, got %T", obj.GetRaw())
	}

	row.ID = fmt.Sprintf("%s/%s/%s", obj.GetRegion(), StrPtrToStr(nodegroup.ClusterName), obj.GetName())
	row.Fields = model1.Fields{
		obj.GetRegion(),
		StrPtrToStr(nodegroup.ClusterName),
		StrPtrToStr(nodegroup.NodegroupName),
		string(nodegroup.Status),
		JoinStrings(",", nodegroup.InstanceTypes...),
		getDesiredSize(nodegroup.ScalingConfig),
		getMinSize(nodegroup.ScalingConfig),
		getMaxSize(nodegroup.ScalingConfig),
		string(nodegroup.AmiType),
		ToAge(obj.GetCreatedAt()),
	}
	return nil
}

// ColorerFunc returns the node group colorer
func (n *EKSNodeGroup) ColorerFunc() model1.ColorerFunc {
	return func(region string, h model1.Header, re *model1.RowEvent) tcell.Color {
		statusIdx, ok := h.IndexOf("STATUS", true)
		if !ok || statusIdx >= len(re.Row.Fields) {
			return model1.StdColor
		}

		status := re.Row.Fields[statusIdx]
		switch status {
		case "ACTIVE":
			return model1.StdColor
		case "CREATING", "UPDATING":
			return model1.AddColor
		case "DELETING", "DEGRADED":
			return model1.KillColor
		case "CREATE_FAILED", "DELETE_FAILED":
			return model1.ErrColor
		default:
			return model1.StdColor
		}
	}
}

func getDesiredSize(config *types.NodegroupScalingConfig) string {
	if config == nil || config.DesiredSize == nil {
		return NAValue
	}
	return Int32PtrToStr(config.DesiredSize)
}

func getMinSize(config *types.NodegroupScalingConfig) string {
	if config == nil || config.MinSize == nil {
		return NAValue
	}
	return Int32PtrToStr(config.MinSize)
}

func getMaxSize(config *types.NodegroupScalingConfig) string {
	if config == nil || config.MaxSize == nil {
		return NAValue
	}
	return Int32PtrToStr(config.MaxSize)
}
