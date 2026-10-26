# govalorderedmap

[![Go Reference](https://pkg.go.dev/badge/github.com/andys/govalorderedmap.svg)](https://pkg.go.dev/github.com/andys/govalorderedmap)

A generic Go map that keeps its entries **sorted by value**, while still giving
O(1) lookup by key.

Its main feature is `DeleteBefore(v)`, which removes every entry whose value is
less than `v` in **O(log n + k)** time. It only visits the entries it removes.
This makes it a good fit for:

- TTL / expiry caches (value = expiry time)
- Sliding time windows (value = last-seen timestamp)
- Priority-ordered sets where low-priority items are pruned in bulk

## Install

```sh
go get github.com/andys/govalorderedmap
```

Requires Go 1.23+ (uses range-over-func iterators).

## Usage

```go
package main

import (
	"fmt"

	valorderedmap "github.com/andys/govalorderedmap"
)

func main() {
	m := valorderedmap.New[string, int]()

	m.Set("alice", 30)
	m.Set("bob", 10)
	m.Set("carol", 20)

	v, ok := m.Get("carol") // 20, true
	fmt.Println(v, ok)

	for k, v := range m.All() { // ascending by value
		fmt.Println(k, v) // bob 10, carol 20, alice 30
	}

	n := m.DeleteBefore(25) // removes bob and carol
	fmt.Println(n, m.Len()) // 2 1
}
```

### Expiry cache example

```go
seen := valorderedmap.New[string, int64]()

seen.Set("session-a", time.Now().Unix())
// ...
expired := seen.DeleteBefore(time.Now().Add(-10 * time.Minute).Unix())
```

### Custom value ordering

For value types that aren't `cmp.Ordered`, pass a comparison function:

```go
type Job struct {
	Priority int
	Name     string
}

jobs := valorderedmap.NewFunc[string, Job](func(a, b Job) int {
	return cmp.Compare(a.Priority, b.Priority)
})
```

### JSON serialization

A `Map` implements `json.Marshaler` and `json.Unmarshaler`. It encodes as an
array of `{"key", "value"}` objects in ascending value order, so the order of
equal values is preserved across a round trip:

```go
m := valorderedmap.New[string, int]()
m.Set("bob", 10)
m.Set("alice", 30)
m.Set("carol", 10)

data, err := json.Marshal(m)
// [{"key":"bob","value":10},{"key":"carol","value":10},{"key":"alice","value":30}]

restored := valorderedmap.New[string, int]() // must be created with New/NewFunc
err = json.Unmarshal(data, restored)
```

The comparator can't be serialized, so you must decode into a map created
with `New` or `NewFunc`. This also applies to struct fields:

```go
type Message struct {
	Jobs *valorderedmap.Map[string, Job] `json:"jobs"`
}

msg := Message{Jobs: valorderedmap.NewFunc[string, Job](compareJobs)}
err := json.Unmarshal(data, &msg)
```

Decoding replaces the map's existing contents. JSON `null` decodes to an empty
map. Entries whose keys can't be stored are skipped rather than causing a panic:

- non-reflexive keys, such as NaN;
- keys that aren't comparable at runtime, such as JSON arrays or objects
  decoded into an `any` key;
- repeated keys (the first occurrence wins).

`Unmarshal` returns an error only if the JSON is malformed or has the wrong
types, or if the map is uninitialized (`ErrUninitialized`). In both cases the
map is left unchanged.

For other formats (gob, CBOR, MessagePack, ...), use `Entries()` to get a
`[]Entry[K, V]` snapshot and `Load(entries)` to restore it. `Load` returns the
number of entries it skipped.

## API

| Method                     | Description                                   | Complexity         |
|----------------------------|-----------------------------------------------|--------------------|
| `New[K, V cmp.Ordered]()`  | Create a map with natural value ordering      |                    |
| `NewFunc[K, V](compare)`   | Create a map with a custom comparator         |                    |
| `Get(key) (V, bool)`       | Look up a value by key                        | O(1)               |
| `Has(key) bool`            | Report whether a key exists                   | O(1)               |
| `Len() int`                | Number of entries                             | O(1)               |
| `Set(key, val)`            | Insert or update                              | O(log n) expected  |
| `Delete(key) bool`         | Remove by key                                 | O(log n) expected  |
| `DeleteBefore(val) int`    | Remove all entries with value `< val`         | O(log n + k)       |
| `Min() (K, V, bool)`       | Entry with the smallest value                 | O(log n) expected  |
| `Max() (K, V, bool)`       | Entry with the largest value                  | O(log n) expected  |
| `All()`                    | Iterator in ascending value order             | O(n)               |
| `Clear()`                  | Remove everything                             | O(1)*              |
| `Entries() []Entry[K, V]`  | Snapshot of all entries in value order        | O(n)               |
| `Load(entries) (int, error)` | Replace contents; returns number skipped    | O(n log n) expected |
| `MarshalJSON` / `UnmarshalJSON` | JSON encoding as an array of entries     | O(n) / O(n log n)  |

Equal values retain insertion order. Comparator-equal updates preserve position;
updates that change ordering count as reinsertion.

Use the constructors; the zero value is not ready for insertion. Do not copy a
`Map` after first use. `Set` panics for non-reflexive keys (such as NaN).
Comparators must define a stable strict weak ordering and must not mutate the
map. Do not mutate stored values in ways that change their ordering.

A `Map` is **not** safe for concurrent use. Wrap it in a `sync.Mutex` if you
share it between goroutines. Don't modify the map while ranging over `All()`.

\* `Clear` cost depends on the runtime's built-in `clear` for maps.

## How it works

The map combines two structures:

1. A built-in Go `map[K]*node` for O(1) key lookup.
2. A **treap** (a randomized balanced binary search tree) of the same nodes,
   ordered by value.

`DeleteBefore(v)` splits the treap once at `v`, which takes O(log n). It keeps
the right half and walks the detached left half to remove those keys from the
hash map. The entries that stay in the map are never visited.

## Benchmarks

```
BenchmarkSet-16              200000    473.8 ns/op   // random values
BenchmarkSlidingWindow-16    200000    160.1 ns/op   // monotonic inserts + periodic DeleteBefore
```

Run them yourself:

```sh
go test -bench .
```

## License

MIT. See [LICENSE](LICENSE).
