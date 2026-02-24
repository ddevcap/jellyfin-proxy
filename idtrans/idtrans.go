// Package idtrans handles translation between backend Jellyfin item IDs and
// the proxy-scoped IDs exposed to clients.
//
// Proxy IDs have the format "{serverPrefix}_{backendID}", where serverPrefix
// is the short unique string configured on each Server record (e.g. "s1").
// This lets the proxy determine which backend to route a request to from the
// ID alone, without a database lookup on every hop.
package idtrans

import (
	"fmt"
	"strings"
)

const sep = "_"

// Encode creates a proxy-scoped ID: "{prefix}_{backendID}".
// Returns an empty string if backendID is empty.
func Encode(prefix, backendID string) string {
	if backendID == "" {
		return ""
	}
	return prefix + sep + backendID
}

// Decode splits a proxy ID into its server prefix and the original backend ID.
//
//	"s1_abc123" → ("s1", "abc123", nil)
//
// Returns an error if the ID has no prefix (i.e. it was not produced by Encode).
// In that case backendID is set to proxyID so callers can pass it through as-is.
func Decode(proxyID string) (prefix, backendID string, err error) {
	idx := strings.Index(proxyID, sep)
	if idx <= 0 {
		return "", proxyID, fmt.Errorf("idtrans: %q has no server prefix", proxyID)
	}
	return proxyID[:idx], proxyID[idx+len(sep):], nil
}

// DecodePrefix returns only the server prefix from a proxy ID,
// which is enough to look up the target backend.
func DecodePrefix(proxyID string) (string, error) {
	prefix, _, err := Decode(proxyID)
	return prefix, err
}

const mergedPrefix = "merged"

// EncodeMerged returns a virtual proxy ID for a merged library view keyed by
// Jellyfin CollectionType (e.g. "movies", "tvshows"). These IDs are never sent
// to any backend; they are resolved by the proxy itself to fan out across all
// backends that expose a library of that type.
//
// Format: "merged_movies", "merged_tvshows", etc.
func EncodeMerged(collectionType string) string {
	return mergedPrefix + sep + collectionType
}

// DecodeMerged returns the CollectionType from a merged virtual ID, and whether
// the ID is a merged ID at all.
func DecodeMerged(proxyID string) (collectionType string, ok bool) {
	if !strings.HasPrefix(proxyID, mergedPrefix+sep) {
		return "", false
	}
	return proxyID[len(mergedPrefix)+len(sep):], true
}
