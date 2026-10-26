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
