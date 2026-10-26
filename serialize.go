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

// Load replaces the contents of the map with entries, inserting them in
// slice order so that ties keep that order. Entries that cannot be stored
// are skipped: keys that are not comparable at run time (an interface key
// holding a slice, map, or func), non-reflexive keys such as NaN, and
// repeated keys (the first occurrence wins). It returns the number of
// entries skipped. The only error is ErrUninitialized, in which case the
// map is left unchanged.
func (t *Map[K, V]) Load(entries []Entry[K, V]) (skipped int, err error) {
	if t.compare == nil {
		return 0, ErrUninitialized
	}
	t.Clear()
	for _, e := range entries {
		if !validKey(e.Key) || t.Has(e.Key) {
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
// invalid or repeated keys are skipped as described for Load. If the JSON
// itself is malformed, an error is returned and the map is left unchanged.
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
	_, err := t.Load(entries)
	return err
}
