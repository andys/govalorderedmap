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

When two entries have equal values, the one that was `Set` earlier comes first.

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
