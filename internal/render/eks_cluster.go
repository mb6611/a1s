package render

import (
	"fmt"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/gdamore/tcell/v2"
)

// EKSCluster renders EKS clusters
type EKSCluster struct {
	Base
}

// Header returns the EKS cluster header
func (c *EKSCluster) Header(region string) model1.Header {
	return model1.Header{
		{Name: "REGION"},
		{Name: "CLUSTER"},
		{Name: "VERSION"},
		{Name: "STATUS"},
		{Name: "ENDPOINT-ACCESS", Attrs: model1.Attrs{Wide: true}},
		{Name: "VALID", Attrs: model1.Attrs{Wide: true}},
		{Name: "AGE", Attrs: model1.Attrs{Time: true}},
	}
}

// Render renders an EKS cluster to a row
func (c *EKSCluster) Render(o any, region string, row *model1.Row) error {
	obj, ok := o.(dao.AWSObject)
	if !ok {
		return fmt.Errorf("expected AWSObject, got %T", o)
	}

	cluster, ok := obj.GetRaw().(*types.Cluster)
	if !ok {
		return fmt.Errorf("expected *types.Cluster, got %T", obj.GetRaw())
	}

	row.ID = fmt.Sprintf("%s/%s", obj.GetRegion(), obj.GetName())
	row.Fields = model1.Fields{
		obj.GetRegion(),
		StrPtrToStr(cluster.Name),
		StrPtrToStr(cluster.Version),
		string(cluster.Status),
		getEndpointAccess(cluster.ResourcesVpcConfig),
		c.validate(cluster),
		ToAge(obj.GetCreatedAt()),
	}
	return nil
}

// ColorerFunc returns the cluster colorer
func (c *EKSCluster) ColorerFunc() model1.ColorerFunc {
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
		case "DELETING":
			return model1.KillColor
		case "FAILED":
			return model1.ErrColor
		default:
			return model1.StdColor
		}
	}
}

// validate checks cluster for security issues
func (c *EKSCluster) validate(cluster *types.Cluster) string {
	var issues []string

	// Check for public endpoint with unrestricted access
	if cluster.ResourcesVpcConfig != nil {
		if cluster.ResourcesVpcConfig.EndpointPublicAccess {
			if len(cluster.ResourcesVpcConfig.PublicAccessCidrs) == 0 ||
				(len(cluster.ResourcesVpcConfig.PublicAccessCidrs) == 1 &&
					cluster.ResourcesVpcConfig.PublicAccessCidrs[0] == "0.0.0.0/0") {
				issues = append(issues, "public-unrestricted")
			}
		}
	}

	if len(issues) > 0 {
		return JoinStrings(",", issues...)
	}
	return ""
}

func getEndpointAccess(config *types.VpcConfigResponse) string {
	if config == nil {
		return NAValue
	}

	var access []string
	if config.EndpointPublicAccess {
		access = append(access, "Public")
	}
	if config.EndpointPrivateAccess {
		access = append(access, "Private")
	}

	if len(access) == 0 {
		return NAValue
	}
	return JoinStrings("/", access...)
}
