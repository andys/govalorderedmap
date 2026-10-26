package valorderedmap

import (
	"encoding/json"
	"errors"
	"math"
	"slices"
	"testing"
)

func TestJSONRoundTrip(t *testing.T) {
	m := New[string, int]()
	m.Set("b", 2)
	m.Set("x", 1)
	m.Set("a", 1)
	m.Set("c", 3)
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"key":"x","value":1},{"key":"a","value":1},{"key":"b","value":2},{"key":"c","value":3}]`
	if string(data) != want {
		t.Fatalf("got %s", data)
	}
	r := New[string, int]()
	r.Set("stale", 9)
	if err := json.Unmarshal(data, r); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.Entries(), m.Entries()) {
		t.Fatalf("got %v want %v", r.Entries(), m.Entries())
	}
	// Restored map must remain fully functional.
	r.Set("d", 0)
	if k, _, _ := r.Min(); k != "d" || r.DeleteBefore(2) != 3 || r.Len() != 2 {
		t.Fatal("restored map broken")
	}
}

func TestJSONInStruct(t *testing.T) {
	type msg struct {
		M *Map[int, string] `json:"m"`
	}
	in := msg{New[int, string]()}
	in.M.Set(1, "z")
	in.M.Set(2, "a")
	data, _ := json.Marshal(in)
	out := msg{New[int, string]()}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(out.M.Entries(), []Entry[int, string]{{2, "a"}, {1, "z"}}) {
		t.Fatal(out.M.Entries())
	}
}

func TestJSONNullAndEmpty(t *testing.T) {
	m := New[int, int]()
	m.Set(1, 1)
	if err := json.Unmarshal([]byte("null"), m); err != nil || m.Len() != 0 {
		t.Fatal(err, m.Len())
	}
	data, _ := json.Marshal(New[int, int]())
	if string(data) != "[]" {
		t.Fatalf("got %s", data)
	}
}

func TestJSONMalformedLeavesMapUnchanged(t *testing.T) {
	cases := []string{
		`[{"key":1,"value":"x"}]`,
		`{"1":1}`,
		`[`,
	}
	for _, c := range cases {
		m := New[int, int]()
		m.Set(5, 5)
		if err := json.Unmarshal([]byte(c), m); err == nil {
			t.Fatalf("%s: expected error", c)
		}
		if v, ok := m.Get(5); !ok || v != 5 || m.Len() != 1 {
			t.Fatalf("%s: map modified", c)
		}
	}
}

func TestJSONDuplicateKeysFirstWins(t *testing.T) {
	m := New[int, int]()
	if err := json.Unmarshal([]byte(`[{"key":1,"value":1},{"key":1,"value":2},{"key":2,"value":0}]`), m); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Entries(), []Entry[int, int]{{2, 0}, {1, 1}}) {
		t.Fatal(m.Entries())
	}
}

func TestJSONUninitialized(t *testing.T) {
	var m Map[int, int]
	if err := json.Unmarshal([]byte(`[]`), &m); !errors.Is(err, ErrUninitialized) {
		t.Fatal(err)
	}
	if _, err := m.Load(nil); !errors.Is(err, ErrUninitialized) {
		t.Fatal(err)
	}
}

func TestLoadSkipsInvalidKeys(t *testing.T) {
	m := New[float64, int]()
	n, err := m.Load([]Entry[float64, int]{{math.NaN(), 1}, {1, 1}})
	if err != nil || n != 1 || m.Len() != 1 {
		t.Fatal(n, err, m.Len())
	}

	type pair struct{ A, B any }
	a := New[any, int]()
	n, err = a.Load([]Entry[any, int]{
		{[]int{1}, 1},
		{map[int]int{}, 2},
		{pair{1, []int{1}}, 3},
		{[1]any{func() {}}, 4},
		{math.NaN(), 5},
		{"ok", 6},
		{pair{1, 2}, 7},
	})
	if err != nil || n != 5 {
		t.Fatal(n, err)
	}
	if !slices.Equal(a.Entries(), []Entry[any, int]{{"ok", 6}, {pair{1, 2}, 7}}) {
		t.Fatal(a.Entries())
	}

	// JSON-decoded interface keys: arrays and objects become slices/maps.
	j := New[any, int]()
	if err := json.Unmarshal([]byte(`[{"key":[1],"value":1},{"key":{"a":1},"value":2},{"key":"s","value":3}]`), j); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(j.Entries(), []Entry[any, int]{{"s", 3}}) {
		t.Fatal(j.Entries())
	}
}
