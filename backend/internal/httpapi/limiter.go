package httpapi

import (
	"sync"
	"time"
)

type window struct {
	count int
	end   time.Time
}

type limiter struct {
	mu        sync.Mutex
	entries   map[string]window
	nextSweep time.Time
}

func newLimiter() *limiter { return &limiter{entries: make(map[string]window)} }

func (l *limiter) allow(key string, limit int, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	// Очистка при запросах не требует фоновой горутины и её остановки.
	if !now.Before(l.nextSweep) {
		for key, entry := range l.entries {
			if !now.Before(entry.end) {
				delete(l.entries, key)
			}
		}
		l.nextSweep = now.Add(time.Minute)
	}
	entry := l.entries[key]
	if !now.Before(entry.end) {
		entry = window{end: now.Add(time.Minute)}
	}
	if entry.count >= limit {
		return false, entry.end.Sub(now)
	}
	entry.count++
	l.entries[key] = entry
	return true, 0
}
