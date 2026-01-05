package render

import (
	"fmt"
	"strings"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gdamore/tcell/v2"
)

// S3Object renders S3 objects
type S3Object struct {
	Base
}

// Header returns the S3 object header
func (o *S3Object) Header(region string) model1.Header {
	return model1.Header{
		{Name: "KEY"},
		{Name: "SIZE", Attrs: model1.Attrs{Capacity: true}},
		{Name: "STORAGE-CLASS"},
		{Name: "LAST-MODIFIED", Attrs: model1.Attrs{Time: true}},
	}
}

// Render renders an S3 object to a row
func (obj *S3Object) Render(o any, region string, row *model1.Row) error {
	awsObj, ok := o.(dao.AWSObject)
	if !ok {
		return fmt.Errorf("expected AWSObject, got %T", o)
	}

	s3Obj, ok := awsObj.GetRaw().(types.Object)
	if !ok {
		// Could be a prefix (folder)
		row.ID = awsObj.GetID()
		row.Fields = model1.Fields{
			awsObj.GetName(),
			NAValue,
			NAValue,
			NAValue,
		}
		return nil
	}

	row.ID = awsObj.GetID()
	row.Fields = model1.Fields{
		getObjectDisplayName(StrPtrToStr(s3Obj.Key)),
		formatS3Size(s3Obj.Size),
		string(s3Obj.StorageClass),
		ToAge(s3Obj.LastModified),
	}
	return nil
}

// ColorerFunc returns the object colorer
func (o *S3Object) ColorerFunc() model1.ColorerFunc {
	return func(region string, h model1.Header, re *model1.RowEvent) tcell.Color {
		storageIdx, ok := h.IndexOf("STORAGE-CLASS", true)
		if !ok || storageIdx >= len(re.Row.Fields) {
			return model1.StdColor
		}

		storage := re.Row.Fields[storageIdx]
		switch storage {
		case "GLACIER", "DEEP_ARCHIVE", "GLACIER_IR":
			return model1.PendingColor // Cyan for archived
		default:
			return model1.StdColor
		}
	}
}

func getObjectDisplayName(key string) string {
	// Show just the filename for display
	parts := strings.Split(key, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return key
}

func formatS3Size(size *int64) string {
	if size == nil {
		return NAValue
	}
	return FormatSize(*size)
}
