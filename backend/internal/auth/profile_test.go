package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	for _, tc := range []struct {
		name  string
		want  string
		valid bool
	}{
		{" Анна Марія ", "Анна Марія", true},
		{"O'Connor-Smith", "O'Connor-Smith", true},
		{strings.Repeat("🙂", 100), strings.Repeat("🙂", 100), true},
		{"", "", false},
		{"   ", "", false},
		{strings.Repeat("я", 101), "", false},
		{"a\nb", "", false},
		{"a\x00b", "", false},
		{"a\x7fb", "", false},
		{"a\xffb", "", false},
	} {
		got, err := ValidateName(tc.name)
		if tc.valid {
			if err != nil || got != tc.want {
				t.Fatalf("%q: got %q, %v", tc.name, got, err)
			}
		} else if !errors.Is(err, ErrInvalidName) {
			t.Fatalf("%q: want invalid name, got %v", tc.name, err)
		}
	}
}
