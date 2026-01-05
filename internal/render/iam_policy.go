package render

import (
	"fmt"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// IAMPolicy renders IAM policies
type IAMPolicy struct {
	Base
}

// Header returns the IAM policy header
func (p *IAMPolicy) Header(region string) model1.Header {
	return model1.Header{
		{Name: "POLICY-NAME"},
		{Name: "POLICY-ID", Attrs: model1.Attrs{Wide: true}},
		{Name: "PATH", Attrs: model1.Attrs{Wide: true}},
		{Name: "ATTACHABLE"},
		{Name: "ATTACHMENTS", Attrs: model1.Attrs{Capacity: true}},
		{Name: "VERSIONS", Attrs: model1.Attrs{Capacity: true, Wide: true}},
		{Name: "AGE", Attrs: model1.Attrs{Time: true}},
	}
}

// Render renders an IAM policy to a row
func (p *IAMPolicy) Render(o any, region string, row *model1.Row) error {
	obj, ok := o.(dao.AWSObject)
	if !ok {
		return fmt.Errorf("expected AWSObject, got %T", o)
	}

	policy, ok := obj.GetRaw().(types.Policy)
	if !ok {
		return fmt.Errorf("expected types.Policy, got %T", obj.GetRaw())
	}

	row.ID = obj.GetARN() // Policy ARN as ID
	row.Fields = model1.Fields{
		StrPtrToStr(policy.PolicyName),
		StrPtrToStr(policy.PolicyId),
		StrPtrToStr(policy.Path),
		BoolToYesNo(policy.IsAttachable),
		Int32PtrToStr(policy.AttachmentCount),
		NA(StrPtrToStr(policy.DefaultVersionId)),
		ToAge(obj.GetCreatedAt()),
	}
	return nil
}

// ColorerFunc returns the policy colorer
func (p *IAMPolicy) ColorerFunc() model1.ColorerFunc {
	return model1.DefaultColorer
}
