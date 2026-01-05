package render

import (
	"fmt"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// IAMRole renders IAM roles
type IAMRole struct {
	Base
}

// Header returns the IAM role header
func (r *IAMRole) Header(region string) model1.Header {
	return model1.Header{
		{Name: "ROLE-NAME"},
		{Name: "ROLE-ID", Attrs: model1.Attrs{Wide: true}},
		{Name: "PATH", Attrs: model1.Attrs{Wide: true}},
		{Name: "DESCRIPTION", Attrs: model1.Attrs{Wide: true}},
		{Name: "MAX-SESSION", Attrs: model1.Attrs{Capacity: true, Wide: true}},
		{Name: "AGE", Attrs: model1.Attrs{Time: true}},
	}
}

// Render renders an IAM role to a row
func (r *IAMRole) Render(o any, region string, row *model1.Row) error {
	obj, ok := o.(dao.AWSObject)
	if !ok {
		return fmt.Errorf("expected AWSObject, got %T", o)
	}

	role, ok := obj.GetRaw().(types.Role)
	if !ok {
		return fmt.Errorf("expected types.Role, got %T", obj.GetRaw())
	}

	row.ID = obj.GetName() // Role name
	row.Fields = model1.Fields{
		StrPtrToStr(role.RoleName),
		StrPtrToStr(role.RoleId),
		StrPtrToStr(role.Path),
		Truncate(StrPtrToStr(role.Description), 40),
		formatMaxSession(role.MaxSessionDuration),
		ToAge(obj.GetCreatedAt()),
	}
	return nil
}

// ColorerFunc returns the role colorer
func (r *IAMRole) ColorerFunc() model1.ColorerFunc {
	return model1.DefaultColorer
}

func formatMaxSession(duration *int32) string {
	if duration == nil {
		return NAValue
	}
	hours := *duration / 3600
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%ds", *duration)
}
