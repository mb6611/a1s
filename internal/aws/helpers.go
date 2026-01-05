package aws

import "time"

// SafeString safely dereferences a string pointer, returning empty string if nil.
func SafeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// StringValue safely dereferences a string pointer, returning empty string if nil.
func StringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// BoolValue safely dereferences a bool pointer, returning false if nil.
func BoolValue(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// DefaultWaiterTimeout is the default timeout for AWS waiters (15 minutes).
const DefaultWaiterTimeout = 15 * time.Minute
