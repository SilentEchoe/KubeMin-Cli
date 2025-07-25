package utils

import (
	"regexp"
	"testing"
)

// TestRandStringBytesRandomString tests the RandStringBytesRandomString function.
func TestRandStringBytesRandomString(t *testing.T) {
	for i := 0; i < 10; i++ {
		s := RandStringBytesRandomString(i)
		if len(s) != i {
			t.Errorf("RandStringBytesRandomString(%d) length = %d; want %d", i, len(s), i)
		}
	}
}

// TestRandomStringWithNumber tests the RandomStringWithNumber function.
func TestRandomStringWithNumber(t *testing.T) {
	for i := 0; i < 10; i++ {
		s := RandomStringWithNumber(i)
		if len(s) != i {
			t.Errorf("RandomStringWithNumber(%d) length = %d; want %d", i, len(s), i)
		}
		// Check if the string contains only allowed characters (alphanumeric)
		if !regexp.MustCompile(`^[a-zA-Z0-9]*$`).MatchString(s) {
			t.Errorf("RandomStringWithNumber(%d) = %q, contains invalid characters", i, s)
		}
	}
}

// TestRandStringByNumLowercase tests the RandStringByNumLowercase function.
func TestRandStringByNumLowercase(t *testing.T) {
	for i := 0; i < 10; i++ {
		s := RandStringByNumLowercase(i)
		if len(s) != i {
			t.Errorf("RandStringByNumLowercase(%d) length = %d; want %d", i, len(s), i)
		}
		// Check if the string contains only allowed characters (lowercase alphanumeric)
		if !regexp.MustCompile(`^[a-z0-9]*$`).MatchString(s) {
			t.Errorf("RandStringByNumLowercase(%d) = %q, contains invalid characters", i, s)
		}
	}
}

// TestToRFC1123Name tests the ToRFC1123Name function.
func TestToRFC1123Name(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", "port"},
		{"simple string", "hello", "hello"},
		{"uppercase", "World", "world"},
		{"with spaces", "hello world", "hello-world"},
		{"with special chars", "a!b@c#d$e%f^g&h*i(j)k_l+m=", "a-b-c-d-e-f-g-h-i-j-k-l-m"},
		{"leading/trailing hyphens", "-a-b-", "a-b"},
		{"consecutive hyphens", "a--b", "a-b"},
		{"long string", "this-is-a-very-long-string-that-should-be-handled-correctly", "this-is-a-very-long-string-that-should-be-handled-correctly"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ToRFC1123Name(tc.input)
			if result != tc.expected {
				t.Errorf("ToRFC1123Name(%q) = %q; want %q", tc.input, result, tc.expected)
			}
		})
	}
}
