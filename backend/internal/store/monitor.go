package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrMonitorLimit = errors.New("monitor quota reached")

type Monitor struct {
	ID              uint64
	UserID          uint64
	URL             string
	IntervalSeconds int
	CreatedAt       time.Time
}

type MonitorCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uint64    `json:"id"`
}

func (s *Store) CreateMonitor(ctx context.Context, monitor *Monitor, quota int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the owner so concurrent creates cannot both claim the last slot.
		var user User
		if err := tx.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, monitor.UserID).Error; err != nil {
			return unauthorizedIfMissing(err)
		}
		var count int64
		if err := tx.Model(&Monitor{}).Where("user_id = ?", monitor.UserID).Count(&count).Error; err != nil {
			return err
		}
		if count >= int64(quota) {
			return ErrMonitorLimit
		}
		return tx.Create(monitor).Error
	})
}

func (s *Store) ListMonitors(ctx context.Context, userID uint64, before *MonitorCursor, limit int) ([]Monitor, error) {
	monitors := make([]Monitor, 0)
	query := s.db.WithContext(ctx).Where("user_id = ?", userID)
	if before != nil {
		query = query.Where("(created_at, id) < (?, ?)", before.CreatedAt, before.ID)
	}
	err := query.Order("created_at DESC, id DESC").Limit(limit).Find(&monitors).Error
	return monitors, err
}
