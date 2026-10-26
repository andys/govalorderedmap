package valorderedmap

import (
	"math/rand/v2"
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
	n := 0
	for k, v := range m.All() {
		if ref[k] != v {
			t.Fatalf("iter %d=%d want %d", k, v, ref[k])
		}
		vals = append(vals, v)
		n++
	}
	if n != len(ref) || !sort.IntsAreSorted(vals) {
		t.Fatalf("iteration not sorted or wrong count: %v", vals)
	}
}

func TestRandomized(t *testing.T) {
	m := New[int, int]()
	ref := map[int]int{}
	for i := 0; i < 20000; i++ {
		k, v := rand.IntN(500), rand.IntN(1000)
		switch rand.IntN(10) {
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
		if i%500 == 0 {
			check(t, m, ref)
		}
	}
	check(t, m, ref)
}

func TestMinMax(t *testing.T) {
	m := New[string, float64]()
	if _, _, ok := m.Min(); ok {
		t.Fatal("empty Min")
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
