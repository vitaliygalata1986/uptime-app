package monitor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestValidateURL(t *testing.T) {
	for _, value := range []string{
		"https://example.com", "http://example.com/path?x=1&y=2",
		"https://example.com:8443/health", "http://localhost:8081/healthz",
		"https://[::1]/", "https://приклад.укр/сторінка",
	} {
		t.Run(value, func(t *testing.T) {
			got, err := ValidateURL("  " + value + "  ")
			if err != nil || got != value {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
	for _, value := range []string{
		"", "example.com", "//example.com", "ftp://example.com", "javascript:alert(1)",
		"https://", "https:///path", "https://user:password@example.com",
		"https://example.com/#fragment", "https://example.com/a b", "https://example.com/\x00",
		"https://example.com:\n80", "https://example.com:no", "https://example.com:65536", "https://example.com:0",
		"https://example.com/%zz",
	} {
		t.Run(fmt.Sprintf("invalid %q", value[:min(len(value), 80)]), func(t *testing.T) {
			if _, err := ValidateURL(value); !errors.Is(err, ErrInvalidURL) {
				t.Fatalf("accepted URL %q: %v", value, err)
			}
		})
	}
}

func TestValidateURLLength(t *testing.T) {
	const prefix = "https://example.com/"
	atLimit := prefix + strings.Repeat("a", 4096-len(prefix))
	if got, err := ValidateURL("  " + atLimit + "  "); err != nil || got != atLimit {
		t.Fatalf("expected a trimmed URL of 4096 bytes, got %d bytes: %v", len(got), err)
	}
	for _, value := range []string{atLimit + "a", atLimit[:len(atLimit)-1] + "я"} {
		if _, err := ValidateURL(value); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("accepted a URL of %d bytes: %v", len(value), err)
		}
	}
}

func TestCreateRejectsInvalidIntervalBeforeStorage(t *testing.T) {
	service := &Service{}
	for _, interval := range []int{-1, 0, MaxIntervalSeconds + 1} {
		if _, err := service.Create(context.Background(), 1, "https://example.com", interval); !errors.Is(err, ErrInvalidInterval) {
			t.Fatalf("interval %d: %v", interval, err)
		}
	}
}
