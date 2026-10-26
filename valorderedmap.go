// Package valorderedmap provides a generic map whose entries are kept
// sorted by value, supporting O(1) key lookup and fast removal of all
// entries whose value is less than a threshold.
//
// Internally it combines a hash map (key -> node) with a treap ordered by
// (value, insertion sequence). Operations:
//
//	Get, Has, Len          O(1)
//	Set, Delete            O(log n) expected
//	DeleteBefore(v)        O(log n + k) expected, k = number removed
//	Min, Max              O(log n) expected
//	All                   O(n)
//
// A Map is not safe for concurrent use.
package valorderedmap

import (
	"cmp"
	"math/rand/v2"
)

type node[K comparable, V any] struct {
	key         K
	val         V
	seq         uint64 // tie-breaker for equal values
	prio        uint64
	left, right *node[K, V]
}

// Map is a key/value map kept ordered by value.
// Use New or NewFunc to initialize it. Do not copy a Map after first use.
// Values must not be mutated in ways that change their ordering while stored.
type Map[K comparable, V any] struct {
	m       map[K]*node[K, V]
	root    *node[K, V]
	compare func(a, b V) int
	seq     uint64
	rng     *rand.PCG
}

// New creates a Map for ordered value types.
func New[K comparable, V cmp.Ordered]() *Map[K, V] {
	return NewFunc[K, V](cmp.Compare[V])
}

// NewFunc creates a Map using compare to order values. compare must return
// a negative number when a < b, zero when equal, positive when a > b.
// It must define a stable strict weak ordering and must not mutate the map.
func NewFunc[K comparable, V any](compare func(a, b V) int) *Map[K, V] {
	return &Map[K, V]{
		m:       make(map[K]*node[K, V]),
		compare: compare,
		rng:     rand.NewPCG(rand.Uint64(), rand.Uint64()),
	}
}

// Len returns the number of entries.
func (t *Map[K, V]) Len() int { return len(t.m) }

// Get returns the value for key and whether it was present.
func (t *Map[K, V]) Get(key K) (V, bool) {
	if n, ok := t.m[key]; ok {
		return n.val, true
	}
	var zero V
	return zero, false
}

// Has reports whether key is present.
func (t *Map[K, V]) Has(key K) bool {
	_, ok := t.m[key]
	return ok
}

// Set inserts or updates key with val. Comparator-equal updates preserve tie order.
// It panics if key is non-reflexive (key != key), such as a NaN key.
func (t *Map[K, V]) Set(key K, val V) {
	if key != key {
		panic("valorderedmap: non-reflexive key")
	}
	n, ok := t.m[key]
	if ok {
		if t.compare(n.val, val) == 0 {
			n.val = val
			return
		}
		t.root = t.remove(t.root, n)
		n.left, n.right = nil, nil
	} else {
		n = &node[K, V]{key: key, prio: t.rng.Uint64()}
		t.m[key] = n
	}
	t.seq++
	n.val, n.seq = val, t.seq
	t.root = t.insert(t.root, n)
}

// Delete removes key, reporting whether it was present.
func (t *Map[K, V]) Delete(key K) bool {
	n, ok := t.m[key]
	if !ok {
		return false
	}
	delete(t.m, key)
	t.root = t.remove(t.root, n)
	return true
}

// DeleteBefore removes every entry whose value is strictly less than val
// and returns the number of entries removed.
func (t *Map[K, V]) DeleteBefore(val V) int {
	lt, ge := t.splitBefore(t.root, val)
	t.root = ge
	count := 0
	var walk func(n *node[K, V])
	walk = func(n *node[K, V]) {
		for n != nil {
			walk(n.left)
			delete(t.m, n.key)
			count++
			n = n.right
		}
	}
	walk(lt)
	return count
}

// Min returns the entry with the smallest value.
func (t *Map[K, V]) Min() (key K, val V, ok bool) {
	n := t.root
	if n == nil {
		return key, val, false
	}
	for n.left != nil {
		n = n.left
	}
	return n.key, n.val, true
}

// Max returns the entry with the largest value.
func (t *Map[K, V]) Max() (key K, val V, ok bool) {
	n := t.root
	if n == nil {
		return key, val, false
	}
	for n.right != nil {
		n = n.right
	}
	return n.key, n.val, true
}

// All returns an iterator over entries in ascending value order (ties in
// insertion order; comparator-equal updates preserve their position).
// Updates that change ordering count as reinsertion.
// The map must not be modified during iteration.
func (t *Map[K, V]) All() func(yield func(K, V) bool) {
	return func(yield func(K, V) bool) {
		var walk func(n *node[K, V]) bool
		walk = func(n *node[K, V]) bool {
			for n != nil {
				if !walk(n.left) || !yield(n.key, n.val) {
					return false
				}
				n = n.right
			}
			return true
		}
		walk(t.root)
	}
}

// Clear removes all entries.
func (t *Map[K, V]) Clear() {
	clear(t.m)
	t.root = nil
}

// less orders nodes by (val, seq).
func (t *Map[K, V]) less(a, b *node[K, V]) bool {
	if c := t.compare(a.val, b.val); c != 0 {
		return c < 0
	}
	return a.seq < b.seq
}

func (t *Map[K, V]) insert(root, n *node[K, V]) *node[K, V] {
	if root == nil {
		return n
	}
	if n.prio > root.prio {
		n.left, n.right = t.split(root, n)
		return n
	}
	if t.less(n, root) {
		root.left = t.insert(root.left, n)
	} else {
		root.right = t.insert(root.right, n)
	}
	return root
}

// split partitions root into nodes ordered before n and after n.
func (t *Map[K, V]) split(root, n *node[K, V]) (l, r *node[K, V]) {
	if root == nil {
		return nil, nil
	}
	if t.less(root, n) {
		root.right, r = t.split(root.right, n)
		return root, r
	}
	l, root.left = t.split(root.left, n)
	return l, root
}

// splitBefore partitions root into values < val and values >= val.
func (t *Map[K, V]) splitBefore(root *node[K, V], val V) (l, r *node[K, V]) {
	if root == nil {
		return nil, nil
	}
	if t.compare(root.val, val) < 0 {
		root.right, r = t.splitBefore(root.right, val)
		return root, r
	}
	l, root.left = t.splitBefore(root.left, val)
	return l, root
}

func merge[K comparable, V any](l, r *node[K, V]) *node[K, V] {
	if l == nil {
		return r
	}
	if r == nil {
		return l
	}
	if l.prio > r.prio {
		l.right = merge(l.right, r)
		return l
	}
	r.left = merge(l, r.left)
	return r
}

func (t *Map[K, V]) remove(root, n *node[K, V]) *node[K, V] {
	if root == nil {
		return nil
	}
	if root == n {
		return merge(root.left, root.right)
	}
	if t.less(n, root) {
		root.left = t.remove(root.left, n)
	} else {
		root.right = t.remove(root.right, n)
	}
	return root
}
