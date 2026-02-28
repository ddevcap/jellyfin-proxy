package handler

import (
	"encoding/json"
	"time"

	"github.com/jellydator/ttlcache/v3"
)

const defaultViewCacheTTL = 30 * time.Second

// newViewCache creates a TTL cache for merged library views per user.
// The mergedViews fan-out is expensive (hits all backends), but library
// structure rarely changes. A short TTL (30s) avoids redundant work when
// clients make multiple views requests in rapid succession (e.g. during
// page loads). Expired entries are evicted automatically.
func newViewCache() *ttlcache.Cache[string, []json.RawMessage] {
	cache := ttlcache.New[string, []json.RawMessage](
		ttlcache.WithTTL[string, []json.RawMessage](defaultViewCacheTTL),
		ttlcache.WithDisableTouchOnHit[string, []json.RawMessage](),
	)
	go cache.Start() // starts the automatic expired-item eviction loop
	return cache
}
