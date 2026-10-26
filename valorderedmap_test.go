package valorderedmap

import (
	"cmp"
	"math"
	"math/rand/v2"
	"slices"
	"sort"
	"testing"
)

func check(t *testing.T, m *Map[int, int], ref map[int]int) {
	t.Helper()
	if m.Len() != len(ref) {
		t.Fatalf("len %d want %d", m.Len(), len(ref))
	}
	for k, v := range ref {
		if got, ok := m.Get(k); !ok || got != v {
			t.Fatalf("Get(%d)=%d,%v want %d", k, got, ok, v)
		}
	}
	var vals []int
	seen := make(map[int]bool)
	n := 0
	for k, v := range m.All() {
		want, ok := ref[k]
		if !ok || want != v || seen[k] {
			t.Fatalf("iter %d=%d want %d", k, v, ref[k])
		}
		seen[k] = true
		vals = append(vals, v)
		n++
	}
	if n != len(ref) || !sort.IntsAreSorted(vals) {
		t.Fatalf("iteration not sorted or wrong count: %v", vals)
	}
	visited := make(map[*node[int, int]]bool)
	var walk func(*node[int, int], *node[int, int], *node[int, int])
	walk = func(n, lower, upper *node[int, int]) {
		if n == nil {
			return
		}
		if visited[n] {
			t.Fatal("duplicate node or cycle")
		}
		visited[n] = true
		if m.m[n.key] != n {
			t.Fatal("tree node missing from hash map")
		}
		if (lower != nil && !m.less(lower, n)) || (upper != nil && !m.less(n, upper)) {
			t.Fatal("invalid tree ordering")
		}
		if (n.left != nil && n.left.prio > n.prio) || (n.right != nil && n.right.prio > n.prio) {
			t.Fatal("invalid heap ordering")
		}
		walk(n.left, lower, n)
		walk(n.right, n, upper)
	}
	walk(m.root, nil, nil)
	if len(visited) != len(m.m) {
		t.Fatal("hash map contains nodes absent from tree")
	}
}

func TestRandomized(t *testing.T) {
	const seed = 42
	t.Logf("seed: %d", seed)
	rng := rand.New(rand.NewPCG(seed, 0))
	m := New[int, int]()
	m.rng = rand.NewPCG(seed, 1)
	ref := map[int]int{}
	for i := 0; i < 20000; i++ {
		k, v := rng.IntN(500), rng.IntN(1000)
		switch rng.IntN(10) {
		case 0:
			if m.Delete(k) != (func() bool { _, ok := ref[k]; return ok })() {
				t.Fatal("Delete result mismatch")
			}
			delete(ref, k)
		case 1:
			want := 0
			for rk, rv := range ref {
				if rv < v {
					delete(ref, rk)
					want++
				}
			}
			if got := m.DeleteBefore(v); got != want {
				t.Fatalf("DeleteBefore removed %d want %d", got, want)
			}
		default:
			m.Set(k, v)
			ref[k] = v
		}
		check(t, m, ref)
	}
	check(t, m, ref)
}

func TestClone(t *testing.T) {
	m := New[int, int]()
	for i := 0; i < 100; i++ {
		m.Set(i, i%5)
	}
	m.Delete(50)
	m.Set(0, 3)
	clone := m.Clone()
	if !slices.Equal(m.Entries(), clone.Entries()) || clone.seq != m.seq {
		t.Fatal("clone did not preserve entries and sequence")
	}
	for k, n := range m.m {
		copied := clone.m[k]
		if copied == n || copied.seq != n.seq || copied.prio != n.prio {
			t.Fatal("clone shared a node or changed its metadata")
		}
	}
	if clone.rng == m.rng || *clone.rng != *m.rng {
		t.Fatal("random-generator state was not independently copied")
	}
	before := m.Entries()
	clone.Set(100, 3)
	clone.Set(1, 9)
	clone.Delete(2)
	clone.DeleteBefore(2)
	ref := make(map[int]int)
	for _, entry := range clone.Entries() {
		ref[entry.Key] = entry.Value
	}
	check(t, clone, ref)
	if !slices.Equal(m.Entries(), before) {
		t.Fatal("clone mutations affected original")
	}
	cloneBefore := clone.Entries()
	m.Set(3, 10)
	m.Delete(4)
	m.Clear()
	if !slices.Equal(clone.Entries(), cloneBefore) {
		t.Fatal("original mutations affected clone")
	}
	clone.Clear()
	check(t, clone, map[int]int{})
}

func TestCloneEmpty(t *testing.T) {
	m := New[int, int]()
	clone := m.Clone()
	check(t, clone, map[int]int{})
	clone.Set(1, 2)
	check(t, clone, map[int]int{1: 2})
	check(t, m, map[int]int{})
}

func TestCloneCustomComparator(t *testing.T) {
	type value struct {
		priority int
		payload  *int
	}
	m := NewFunc[string, value](func(a, b value) int {
		return cmp.Compare(b.priority, a.priority)
	})
	payload := 42
	m.Set("a", value{1, &payload})
	m.Set("b", value{2, &payload})
	m.Set("c", value{2, &payload})
	clone := m.Clone()
	clone.Set("d", value{2, &payload})
	clone.Set("b", value{2, &payload})
	var keys []string
	for k := range clone.All() {
		keys = append(keys, k)
	}
	if !slices.Equal(keys, []string{"b", "c", "d", "a"}) {
		t.Fatalf("clone order = %v", keys)
	}
	if v, _ := clone.Get("a"); v.payload != &payload {
		t.Fatal("payload was not copied by assignment")
	}
}

func TestMerge(t *testing.T) {
	m := New[int, int]()
	m.Set(1, 10)
	m.Set(2, 20)
	other := New[int, int]()
	other.Set(2, 5)
	other.Set(3, 15)
	before := other.Entries()
	m.Merge(other)
	check(t, m, map[int]int{1: 10, 2: 5, 3: 15})
	if !slices.Equal(other.Entries(), before) {
		t.Fatal("merge changed source")
	}
	m.Set(3, 30)
	m.Delete(2)
	check(t, other, map[int]int{2: 5, 3: 15})
	other.Set(1, 40)
	other.Clear()
	check(t, m, map[int]int{1: 10, 3: 30})
}

func TestMergeEmptyAndSelf(t *testing.T) {
	m := New[int, int]()
	other := New[int, int]()
	m.Merge(other)
	check(t, m, map[int]int{})
	other.Set(1, 2)
	other.Set(2, 2)
	m.Merge(other)
	check(t, m, map[int]int{1: 2, 2: 2})
	before := m.Entries()
	seq := m.seq
	m.Merge(m)
	m.Merge(New[int, int]())
	if !slices.Equal(m.Entries(), before) || m.seq != seq {
		t.Fatal("self or empty merge changed map")
	}
	check(t, m, map[int]int{1: 2, 2: 2})
}

func TestMergeCustomComparator(t *testing.T) {
	type value struct {
		priority int
		payload  string
	}
	m := NewFunc[string, value](func(a, b value) int {
		return cmp.Compare(b.priority, a.priority)
	})
	other := NewFunc[string, value](func(a, b value) int {
		return cmp.Compare(a.priority, b.priority)
	})
	m.Set("existing", value{2, "old"})
	m.Set("retained", value{2, "retained"})
	other.Set("first", value{2, "first"})
	other.Set("existing", value{2, "new"})
	other.Set("second", value{2, "second"})
	other.Set("high", value{3, "high"})
	m.Merge(other)
	var keys []string
	for key := range m.All() {
		keys = append(keys, key)
	}
	if !slices.Equal(keys, []string{"high", "existing", "retained", "first", "second"}) {
		t.Fatalf("merged order = %v", keys)
	}
	if got, _ := m.Get("existing"); got.payload != "new" {
		t.Fatal("merge did not overwrite comparator-equal value")
	}
}

func TestMinMax(t *testing.T) {
	m := New[string, float64]()
	if _, _, ok := m.Min(); ok {
		t.Fatal("empty Min")
	}
	if _, _, ok := m.Max(); ok {
		t.Fatal("empty Max")
	}
	m.Set("a", 3)
	m.Set("b", 1)
	m.Set("c", 2)
	if k, _, _ := m.Min(); k != "b" {
		t.Fatal(k)
	}
	if k, _, _ := m.Max(); k != "a" {
		t.Fatal(k)
	}
	if n := m.DeleteBefore(2.5); n != 2 || m.Len() != 1 || !m.Has("a") {
		t.Fatal(n, m.Len())
	}
}

func TestTieOrder(t *testing.T) {
	type value struct {
		priority int
		payload  string
	}
	m := NewFunc[string, value](func(a, b value) int {
		return cmp.Compare(a.priority, b.priority)
	})
	m.Set("a", value{1, "old"})
	m.Set("b", value{1, "b"})
	m.Set("a", value{1, "new"})
	assertOrder := func(want []string) {
		t.Helper()
		var got []string
		for k := range m.All() {
			got = append(got, k)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("order %v want %v", got, want)
		}
	}
	assertOrder([]string{"a", "b"})
	if got, ok := m.Get("a"); !ok || got.payload != "new" {
		t.Fatal("comparator-equal update did not replace payload")
	}
	m.Set("a", value{2, "changed"})
	m.Set("a", value{1, "returned"})
	assertOrder([]string{"b", "a"})
	if got := m.DeleteBefore(value{1, "ignored"}); got != 0 {
		t.Fatalf("removed equal values: %d", got)
	}
	if got := m.DeleteBefore(value{2, "ignored"}); got != 2 || m.Len() != 0 {
		t.Fatalf("removed %d, remaining %d", got, m.Len())
	}
}

func TestRejectNonReflexiveKeys(t *testing.T) {
	for _, key := range []any{
		math.NaN(),
		float32(math.NaN()),
		[1]float64{math.NaN()},
		struct{ F float64 }{math.NaN()},
	} {
		m := New[any, int]()
		m.Set("existing", 1)
		func() {
			defer func() {
				if r := recover(); r != "valorderedmap: non-reflexive key" {
					t.Fatalf("panic = %v for %T", r, key)
				}
			}()
			m.Set(key, 2)
		}()
		if m.Len() != 1 || !m.Has("existing") {
			t.Fatal("rejected insertion changed map")
		}
		if got := m.DeleteBefore(3); got != 1 || m.Len() != 0 {
			t.Fatal("rejected insertion left inconsistent indexes")
		}
	}
}

func TestClearAndEarlyStop(t *testing.T) {
	m := New[int, int]()
	for i := 0; i < 10; i++ {
		m.Set(i, i)
	}
	calls := 0
	for k, v := range m.All() {
		calls++
		if k != 0 || v != 0 {
			t.Fatal("iteration did not start at minimum")
		}
		break
	}
	if calls != 1 {
		t.Fatalf("yield called %d times", calls)
	}
	check(t, m, map[int]int{0: 0, 1: 1, 2: 2, 3: 3, 4: 4, 5: 5, 6: 6, 7: 7, 8: 8, 9: 9})
	m.Clear()
	m.Clear()
	check(t, m, map[int]int{})
	if _, _, ok := m.Min(); ok {
		t.Fatal("Min after Clear")
	}
	if _, _, ok := m.Max(); ok {
		t.Fatal("Max after Clear")
	}
	if m.Has(0) || m.Delete(0) || m.DeleteBefore(100) != 0 {
		t.Fatal("cleared entry still present")
	}
	m.Set(1, 2)
	check(t, m, map[int]int{1: 2})
}

func TestNaNValues(t *testing.T) {
	m := New[string, float64]()
	m.Set("nan", math.NaN())
	m.Set("zero", 0)
	if k, _, ok := m.Min(); !ok || k != "nan" {
		t.Fatal("NaN should sort before finite values")
	}
	if got := m.DeleteBefore(math.NaN()); got != 0 {
		t.Fatal("NaN threshold removed entries")
	}
	if got := m.DeleteBefore(0); got != 1 || m.Has("nan") || !m.Has("zero") {
		t.Fatal("finite threshold did not remove only NaN")
	}
}

func BenchmarkSet(b *testing.B) {
	m := New[int, int]()
	for i := 0; i < b.N; i++ {
		m.Set(i, rand.Int())
	}
}

func BenchmarkSlidingWindow(b *testing.B) {
	m := New[int, int]()
	for i := 0; i < b.N; i++ {
		m.Set(i, i)
		if i%100 == 0 {
			m.DeleteBefore(i - 10000)
		}
	}
}
