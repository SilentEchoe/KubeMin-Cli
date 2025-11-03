package utils

import "strings"

func Int32Ptr(i int32) *int32 { return &i }

func ParseInt64(i int64) *int64 {
	return &i
}

func StringPtr(val string) *string {
	if val == "" {
		return nil
	}
	v := val
	return &v
}

func CopyStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

var trimLowerReplacer = strings.NewReplacer(
	" ", "",
	"\n", "",
	"\r", "",
	"\t", "",
)

// NormalizeLowerStrip removes whitespace runes we do not want and returns a lowercase copy.
func NormalizeLowerStrip(val string) string {
	if val == "" {
		return ""
	}
	return trimLowerReplacer.Replace(strings.ToLower(val))
}
