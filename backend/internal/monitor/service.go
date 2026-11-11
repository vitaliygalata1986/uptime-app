package monitor

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"uptime-app/backend/internal/store"
)

var (
	ErrInvalidURL      = errors.New("invalid monitor URL")
	ErrInvalidInterval = errors.New("invalid monitor interval")
)

const MaxIntervalSeconds = 2147483647 // PostgreSQL INTEGER capacity.

// Monitor is the public representation; ownership comes only from authentication.
type Monitor struct {
	ID              uint64    `json:"id"`
	URL             string    `json:"url"`
	IntervalSeconds int       `json:"interval_seconds"`
	CreatedAt       time.Time `json:"created_at"`
}

type Service struct {
	Store *store.Store
}

func ValidateURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || len(value) > 2048 || strings.ContainsFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) {
		return "", ErrInvalidURL
	}
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return "", ErrInvalidURL
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", ErrInvalidURL
		}
	}
	return value, nil
}

func (s *Service) Create(ctx context.Context, userID uint64, rawURL string, interval int) (Monitor, error) {
	value, err := ValidateURL(rawURL)
	if err != nil {
		return Monitor{}, err
	}
	if interval < 1 || interval > MaxIntervalSeconds {
		return Monitor{}, ErrInvalidInterval
	}
	row := store.Monitor{UserID: userID, URL: value, IntervalSeconds: interval, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := s.Store.CreateMonitor(ctx, &row); err != nil {
		return Monitor{}, err
	}
	return publicMonitor(row), nil
}

func (s *Service) List(ctx context.Context, userID uint64) ([]Monitor, error) {
	rows, err := s.Store.ListMonitors(ctx, userID)
	if err != nil {
		return nil, err
	}
	monitors := make([]Monitor, 0, len(rows))
	for _, row := range rows {
		monitors = append(monitors, publicMonitor(row))
	}
	return monitors, nil
}

func publicMonitor(row store.Monitor) Monitor {
	return Monitor{ID: row.ID, URL: row.URL, IntervalSeconds: row.IntervalSeconds, CreatedAt: row.CreatedAt}
}
