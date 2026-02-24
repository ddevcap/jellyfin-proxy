package handler

import (
	"encoding/json"
	"sync"
	"time"
)

// viewCache is a simple TTL cache for merged library views per user.
// The mergedViews fan-out is expensive (hits all backends), but library
// structure rarely changes. A short TTL (30s) avoids redundant work when
// clients make multiple views requests in rapid succession (e.g. during
// page loads).
type viewCache struct {
	mu      sync.RWMutex
	entries map[string]*viewCacheEntry
	ttl     time.Duration
}

type viewCacheEntry struct {
	items     []json.RawMessage
	createdAt time.Time
}

const defaultViewCacheTTL = 30 * time.Second

func newViewCache() *viewCache {
	return &viewCache{
		entries: make(map[string]*viewCacheEntry),
		ttl:     defaultViewCacheTTL,
	}
}

// get returns the cached views for the given user ID, or nil if not cached
// or expired.
func (vc *viewCache) get(userID string) []json.RawMessage {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	entry, ok := vc.entries[userID]
	if !ok || time.Since(entry.createdAt) > vc.ttl {
		return nil
	}
	// Return a copy of the slice so callers can't mutate cache state.
	result := make([]json.RawMessage, len(entry.items))
	copy(result, entry.items)
	return result
}

// set stores the views for the given user ID.
func (vc *viewCache) set(userID string, items []json.RawMessage) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	vc.entries[userID] = &viewCacheEntry{
		items:     items,
		createdAt: time.Now(),
	}
}
