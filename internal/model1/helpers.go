package model1

import (
	"sort"
	"strings"

	"github.com/fvbommel/sortorder"
)

// IsValid returns true if resource is valid, false otherwise
func IsValid(_ string, h Header, r Row) bool {
	if len(r.Fields) == 0 {
		return true
	}
	idx, ok := h.IndexOf("VALID", true)
	if !ok || idx >= len(r.Fields) {
		return true
	}
	val := strings.TrimSpace(r.Fields[idx])
	return val == "" || strings.ToLower(val) == "true"
}

// Less returns true if v1 <= v2
func Less(isNumber, isDuration, isCapacity bool, id1, id2, v1, v2 string) bool {
	var less bool
	switch {
	case isNumber:
		less = lessNumber(v1, v2)
	case isDuration:
		less = lessDuration(v1, v2)
	case isCapacity:
		less = lessCapacity(v1, v2)
	default:
		less = sortorder.NaturalLess(v1, v2)
	}
	if v1 == v2 {
		return sortorder.NaturalLess(id1, id2)
	}
	return less
}

func lessDuration(s1, s2 string) bool {
	d1, d2 := durationToSeconds(s1), durationToSeconds(s2)
	return d1 <= d2
}

func lessCapacity(s1, s2 string) bool {
	// Simple numeric comparison for capacity
	return sortorder.NaturalLess(s1, s2)
}

func lessNumber(s1, s2 string) bool {
	v1, v2 := strings.ReplaceAll(s1, ",", ""), strings.ReplaceAll(s2, ",", "")
	return sortorder.NaturalLess(v1, v2)
}

func durationToSeconds(duration string) int64 {
	if duration == "" || duration == NAValue {
		return 0
	}
	num := make([]rune, 0, 5)
	var n, m int64
	for _, r := range duration {
		switch r {
		case 'y':
			m = 365 * 24 * 60 * 60
		case 'd':
			m = 24 * 60 * 60
		case 'h':
			m = 60 * 60
		case 'm':
			m = 60
		case 's':
			m = 1
		default:
			num = append(num, r)
			continue
		}
		n, num = n+runesToNum(num)*m, num[:0]
	}
	return n
}

func runesToNum(rr []rune) int64 {
	var r int64
	var m int64 = 1
	for i := len(rr) - 1; i >= 0; i-- {
		v := int64(rr[i] - '0')
		r += v * m
		m *= 10
	}
	return r
}

func sortLabels(m map[string]string) (keys, vals []string) {
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		vals = append(vals, m[k])
	}
	return
}
