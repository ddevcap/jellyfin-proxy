package handler

import (
	"encoding/json"
	"sync"
)

// displayPrefsStore stores display preferences per user+client combination
// in memory. Preferences are small JSON blobs that clients send/retrieve
// frequently; keeping them in memory avoids DB round-trips while the proxy
// is running. Preferences survive per-session because clients re-send them
// on each login.
//
// Key format: "<userId>:<prefsId>:<client>" where client comes from the
// query parameter.
type displayPrefsStore struct {
	mu    sync.RWMutex
	prefs map[string]json.RawMessage
}

func newDisplayPrefsStore() *displayPrefsStore {
	return &displayPrefsStore{
		prefs: make(map[string]json.RawMessage),
	}
}

func (s *displayPrefsStore) get(key string) (json.RawMessage, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.prefs[key]
	return v, ok
}

func (s *displayPrefsStore) set(key string, value json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prefs[key] = value
}
