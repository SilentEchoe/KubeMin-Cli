package utils

import (
	"strings"
)

// GetBoxDrawingString get line drawing string, see https://en.wikipedia.org/wiki/Box-drawing_character
// nolint:gocyclo
func GetBoxDrawingString(up bool, down bool, left bool, right bool, padLeft int, padRight int) string {
	var c rune
	switch {
	case up && down && left && right:
		c = '┼'
	case up && down && left && !right:
		c = '┤'
	case up && down && !left && right:
		c = '├'
	case up && down && !left && !right:
		c = '│'
	case up && !down && left && right:
		c = '┴'
	case up && !down && left && !right:
		c = '┘'
	case up && !down && !left && right:
		c = '└'
	case up && !down && !left && !right:
		c = '╵'
	case !up && down && left && right:
		c = '┬'
	case !up && down && left && !right:
		c = '┐'
	case !up && down && !left && right:
		c = '┌'
	case !up && down && !left && !right:
		c = '╷'
	case !up && !down && left && right:
		c = '─'
	case !up && !down && left && !right:
		c = '╴'
	case !up && !down && !left && right:
		c = '╶'
	case !up && !down && !left && !right:
		c = ' '
	}
	sb := strings.Builder{}
	writePadding := func(connect bool, width int) {
		for i := 0; i < width; i++ {
			if connect {
				sb.WriteRune('─')
			} else {
				sb.WriteRune(' ')
			}
		}
	}
	writePadding(left, padLeft)
	sb.WriteRune(c)
	writePadding(right, padRight)
	return sb.String()
}

// Sanitize the inputs by removing line endings
func Sanitize(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func Int32Ptr(i int32) *int32 { return &i }

func ParseInt64(i int64) *int64 {
	return &i
}
