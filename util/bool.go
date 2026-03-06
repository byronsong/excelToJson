package util

import "strings"

// ParseBool 解析布尔值
func ParseBool(value string) bool {
	lower := strings.ToLower(value)
	if lower == "true" || value == "1" || lower == "是" || lower == "yes" {
		return true
	}
	return false
}
