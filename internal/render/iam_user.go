package render

import (
	"fmt"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// IAMUser renders IAM users
type IAMUser struct {
	Base
}

// Header returns the IAM user header
func (u *IAMUser) Header(region string) model1.Header {
	return model1.Header{
		{Name: "USER-NAME"},
		{Name: "USER-ID", Attrs: model1.Attrs{Wide: true}},
		{Name: "PATH", Attrs: model1.Attrs{Wide: true}},
		{Name: "ARN", Attrs: model1.Attrs{Wide: true}},
		{Name: "AGE", Attrs: model1.Attrs{Time: true}},
	}
}

// Render renders an IAM user to a row
func (u *IAMUser) Render(o any, region string, row *model1.Row) error {
	obj, ok := o.(dao.AWSObject)
	if !ok {
		return fmt.Errorf("expected AWSObject, got %T", o)
	}

	user, ok := obj.GetRaw().(types.User)
	if !ok {
		return fmt.Errorf("expected types.User, got %T", obj.GetRaw())
	}

	row.ID = obj.GetName() // User name
	row.Fields = model1.Fields{
		StrPtrToStr(user.UserName),
		StrPtrToStr(user.UserId),
		StrPtrToStr(user.Path),
		StrPtrToStr(user.Arn),
		ToAge(obj.GetCreatedAt()),
	}
	return nil
}

// ColorerFunc returns the user colorer
func (u *IAMUser) ColorerFunc() model1.ColorerFunc {
	return model1.DefaultColorer
}
