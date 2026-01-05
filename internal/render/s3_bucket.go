package render

import (
	"fmt"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
)

// S3Bucket renders S3 buckets
type S3Bucket struct {
	Base
}

// Header returns the S3 bucket header
func (b *S3Bucket) Header(region string) model1.Header {
	return model1.Header{
		{Name: "NAME"},
		{Name: "REGION"},
		{Name: "AGE", Attrs: model1.Attrs{Time: true}},
	}
}

// Render renders an S3 bucket to a row
func (b *S3Bucket) Render(o any, region string, row *model1.Row) error {
	obj, ok := o.(dao.AWSObject)
	if !ok {
		return fmt.Errorf("expected AWSObject, got %T", o)
	}

	row.ID = obj.GetName() // Bucket name is globally unique
	row.Fields = model1.Fields{
		obj.GetName(),
		NA(obj.GetRegion()),
		ToAge(obj.GetCreatedAt()),
	}
	return nil
}

// ColorerFunc returns the bucket colorer
func (b *S3Bucket) ColorerFunc() model1.ColorerFunc {
	return model1.DefaultColorer
}
