package utils

import "strings"

func Int32Ptr(i int32) *int32 { return &i }

func ParseInt64(i int64) *int64 {
	return &i
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
