package render

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ToAge converts time to human-readable duration
func ToAge(t *time.Time) string {
	if t == nil || t.IsZero() {
		return UnknownValue
	}
	return HumanDuration(time.Since(*t))
}

// HumanDuration converts duration to human readable format (e.g., "5d", "3h", "2m")
func HumanDuration(d time.Duration) string {
	if d < time.Second {
		return "0s"
	}

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 365 {
		years := days / 365
		return fmt.Sprintf("%dy", years)
	}
	if days > 0 {
		return fmt.Sprintf("%dd", days)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", seconds)
}

// Missing returns MissingValue if string is empty
func Missing(s string) string {
	if s == "" {
		return MissingValue
	}
	return s
}

// NA returns NAValue if string is empty
func NA(s string) string {
	if s == "" {
		return NAValue
	}
	return s
}

// BoolToYesNo converts bool to Yes/No string
func BoolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

// BoolPtrToYesNo converts *bool to Yes/No/NA string
func BoolPtrToYesNo(b *bool) string {
	if b == nil {
		return NAValue
	}
	return BoolToYesNo(*b)
}

// StrPtrToStr converts *string to string
func StrPtrToStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Int32PtrToStr converts *int32 to string
func Int32PtrToStr(i *int32) string {
	if i == nil {
		return NAValue
	}
	return strconv.Itoa(int(*i))
}

// Int64PtrToStr converts *int64 to string
func Int64PtrToStr(i *int64) string {
	if i == nil {
		return NAValue
	}
	return strconv.FormatInt(*i, 10)
}

// FormatSize formats bytes to human readable format
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatSizeGB formats size in GiB
func FormatSizeGB(sizeGB int32) string {
	return fmt.Sprintf("%d GiB", sizeGB)
}

// Truncate truncates a string to max length
func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// ExtractNameTag extracts the "Name" tag from a tag map
func ExtractNameTag(tags map[string]string) string {
	if name, ok := tags["Name"]; ok {
		return name
	}
	return ""
}

// GetTag gets a specific tag value
func GetTag(tags map[string]string, key string) string {
	if val, ok := tags[key]; ok {
		return val
	}
	return ""
}

// JoinStrings joins strings with separator, skipping empty ones
func JoinStrings(sep string, ss ...string) string {
	var parts []string
	for _, s := range ss {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, sep)
}

// MapToStr converts a map to comma-separated key=value string
func MapToStr(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ",")
}

// IntToStr converts int to string
func IntToStr(i int) string {
	return strconv.Itoa(i)
}

// AsCount formats a count (0 shows as "0")
func AsCount(n int) string {
	return strconv.Itoa(n)
}
