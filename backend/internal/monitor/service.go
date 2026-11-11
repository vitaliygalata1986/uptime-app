package monitor

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
	ErrInvalidCursor   = errors.New("invalid monitor cursor")
)

const MaxIntervalSeconds = 2147483647 // PostgreSQL INTEGER capacity.
const MaxMonitorsPerUser = 100
const PageSize = 20

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

type Page struct {
	Monitors   []Monitor `json:"monitors"`
	NextCursor string    `json:"next_cursor"`
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
	if err := s.Store.CreateMonitor(ctx, &row, MaxMonitorsPerUser); err != nil {
		return Monitor{}, err
	}
	return publicMonitor(row), nil
}

func (s *Service) List(ctx context.Context, userID uint64, cursor string) (Page, error) {
	var before *store.MonitorCursor
	if cursor != "" {
		if len(cursor) > 256 {
			return Page{}, ErrInvalidCursor
		}
		decoded, err := base64.RawURLEncoding.DecodeString(cursor)
		var value store.MonitorCursor
		if err != nil || json.Unmarshal(decoded, &value) != nil || value.ID == 0 || value.ID > 9223372036854775807 || value.CreatedAt.IsZero() {
			return Page{}, ErrInvalidCursor
		}
		before = &value
	}
	rows, err := s.Store.ListMonitors(ctx, userID, before, PageSize+1)
	if err != nil {
		return Page{}, err
	}
	page := Page{}
	if len(rows) > PageSize {
		rows = rows[:PageSize]
		last := rows[len(rows)-1]
		encoded, err := json.Marshal(store.MonitorCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		if err != nil {
			return Page{}, err
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
	}
	monitors := make([]Monitor, 0, len(rows))
	for _, row := range rows {
		monitors = append(monitors, publicMonitor(row))
	}
	page.Monitors = monitors
	return page, nil
}

func publicMonitor(row store.Monitor) Monitor {
	return Monitor{ID: row.ID, URL: row.URL, IntervalSeconds: row.IntervalSeconds, CreatedAt: row.CreatedAt}
}
