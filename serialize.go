package valorderedmap

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
)

// Entry is a key/value pair. A slice of entries in ascending value order
// (as produced by Entries) is the portable, wire-friendly form of a Map.
type Entry[K comparable, V any] struct {
	Key   K `json:"key"`
	Value V `json:"value"`
}

// ErrUninitialized is returned when decoding into a Map that was not
// created with New or NewFunc; the comparator cannot be serialized, so the
// destination must already have one.
var ErrUninitialized = errors.New("valorderedmap: map not initialized with New or NewFunc")

// Entries returns all entries in ascending value order (ties in their
// current order). Passing the result to Load on a map with the same
// comparator reproduces the same iteration order.
func (t *Map[K, V]) Entries() []Entry[K, V] {
	out := make([]Entry[K, V], 0, t.Len())
	for k, v := range t.All() {
		out = append(out, Entry[K, V]{k, v})
	}
	return out
}

// Load merges entries into the map. It does not clear the map first:
// existing keys not in entries are kept. Each entry is applied with Set, in
// slice order. So an entry whose key is already present overwrites that
// value, and if a key repeats within entries, the last occurrence wins.
// New keys with equal values keep their slice order. To replace the
// contents instead, call Clear first.
//
// Entries whose keys can't be stored are skipped: keys that are not
// comparable at run time (an interface key holding a slice, map, or func)
// and non-reflexive keys such as NaN. Load returns the number skipped. The
// only error is ErrUninitialized, in which case the map is left unchanged.
func (t *Map[K, V]) Load(entries []Entry[K, V]) (skipped int, err error) {
	if t.compare == nil {
		return 0, ErrUninitialized
	}
	for _, e := range entries {
		if !validKey(e.Key) {
			skipped++
			continue
		}
		t.Set(e.Key, e.Value)
	}
	return skipped, nil
}

// validKey reports whether key can be safely used as a map key: it must be
// comparable at run time (checked first, since == itself panics otherwise)
// and equal to itself.
func validKey[K comparable](key K) bool {
	if !reflect.ValueOf(&key).Elem().Comparable() {
		return false
	}
	return key == key
}

// MarshalJSON encodes the map as a JSON array of {"key","value"} objects
// in ascending value order.
func (t *Map[K, V]) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Entries())
}

// UnmarshalJSON replaces the map's contents from the format produced by
// MarshalJSON. The receiver must have been created with New or NewFunc so
// that it has a comparator. JSON null yields an empty map. Entries with
// invalid keys are skipped as described for Load, and repeated keys are
// skipped too (the first occurrence wins). Unlike Load, decoding does not
// merge. If the JSON itself is malformed, an error is returned and the map
// is left unchanged.
func (t *Map[K, V]) UnmarshalJSON(data []byte) error {
	if t.compare == nil {
		return ErrUninitialized
	}
	var entries []Entry[K, V]
	if !bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		if err := json.Unmarshal(data, &entries); err != nil {
			return err
		}
	}
	t.Clear()
	for _, e := range entries {
		if validKey(e.Key) && !t.Has(e.Key) {
			t.Set(e.Key, e.Value)
		}
	}
	return nil
}
