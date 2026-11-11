package store

import (
	"context"
	"time"
)

type Monitor struct {
	ID              uint64
	UserID          uint64
	URL             string
	IntervalSeconds int
	CreatedAt       time.Time
}

func (s *Store) CreateMonitor(ctx context.Context, monitor *Monitor) error {
	return s.db.WithContext(ctx).Create(monitor).Error
}

func (s *Store) ListMonitors(ctx context.Context, userID uint64) ([]Monitor, error) {
	monitors := make([]Monitor, 0)
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("created_at DESC, id DESC").Find(&monitors).Error
	return monitors, err
}
