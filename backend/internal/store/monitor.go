package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrMonitorLimit    = errors.New("monitor quota reached")
	ErrMonitorNotFound = errors.New("monitor not found")
)

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

// MonitorChanges contains the fields supplied by a monitor update.
type MonitorChanges struct {
	URL             *string
	IntervalSeconds *int
}

// CreateMonitor checks the owner's quota and inserts a validated monitor in one transaction.
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

// ListMonitors returns only the owner's rows, newest first, strictly before the optional cursor.
func (s *Store) ListMonitors(ctx context.Context, userID uint64, before *MonitorCursor, limit int) ([]Monitor, error) {
	monitors := make([]Monitor, 0)
	query := s.db.WithContext(ctx).Where("user_id = ?", userID)
	if before != nil {
		// ID breaks timestamp ties so inserts at the front cannot shift subsequent pages.
		query = query.Where("(created_at, id) < (?, ?)", before.CreatedAt, before.ID)
	}
	err := query.Order("created_at DESC, id DESC").Limit(limit).Find(&monitors).Error
	return monitors, err
}

// UpdateMonitor applies validated changes to the owner's row and returns the persisted values.
func (s *Store) UpdateMonitor(ctx context.Context, userID, id uint64, changes MonitorChanges) (Monitor, error) {
	values := make(map[string]any, 2)
	if changes.URL != nil {
		values["url"] = *changes.URL
	}
	if changes.IntervalSeconds != nil {
		values["interval_seconds"] = *changes.IntervalSeconds
	}

	var row Monitor
	result := s.db.WithContext(ctx).Model(&row).
		Where("user_id = ? AND id = ?", userID, id).
		Clauses(clause.Returning{}).Updates(values)
	if result.Error != nil {
		return Monitor{}, result.Error
	}
	if result.RowsAffected == 0 {
		return Monitor{}, ErrMonitorNotFound
	}
	return row, nil
}

// DeleteMonitor removes the owner's row or reports ErrMonitorNotFound.
func (s *Store) DeleteMonitor(ctx context.Context, userID, id uint64) error {
	result := s.db.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).Delete(&Monitor{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrMonitorNotFound
	}
	return nil
}
