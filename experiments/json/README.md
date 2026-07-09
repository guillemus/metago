# JSON codec experiment

A reflection-free JSON codec generated entirely by metago templates. This is an experiment showing
what the tool can do — it is decoupled from the metago compiler and lives in its own Go module so
its benchmark dependencies never touch the main project.

The interesting part: there is **no runtime library**. The `jsonruntime` template emits the lexer
and marshal helpers into the generated `meta.go`, once per package, so the output is fully
self-contained — `go.mod` of a consumer would stay untouched.

## How it works

```go
//mgo:gen jsonruntime
//mgo:gen json
type User struct {
	ID   int64    `json:"id"`
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

//mgo:gen json
type Address struct { ... }
```

- `jsonruntime` (once per package) emits `jsonLexer` — a cursor over the input buffer with error
  latching, zero-copy string scanning, and an exact fast-path float parser — plus `appendJSONString`
  for encoding.
- `json` emits `MarshalJSON`/`UnmarshalJSON` per type: a byte-appending encoder and a key-switch
  decoder. `switch string(l.keyBytes())` compiles to allocation-free comparisons.
- Nested annotated types are discovered through `.Package.Metas`, so `User` calls
  `Address.unmarshalJSONLexer` directly — reflection-free recursion.
- Handled natively: strings, bools, ints, uints, floats, slices of those, annotated types, slices of
  annotated types, and `map[string]<scalar>`. Anything else falls back to `encoding/json` for that
  field only.

## Benchmarks

User feeds of 1 / 100 / 1,000 / 10,000 users (~0.4 KB / 40 KB / 400 KB / 4 MB), Apple M4 Pro, Go
1.26. Throughput in MB/s (higher is better), allocs/op alongside:

| Unmarshal MB/s  | 0.4 KB  | 40 KB   | 400 KB | 4 MB | allocs @ 4 MB |
| --------------- | ------- | ------- | ------ | ---- | ------------- |
| **metago**      | **549** | **570** | 543    | 586  | **40,018**    |
| goccy/go-json   | 531     | 554     | 566    | 591  | 150,017       |
| bytedance/sonic | 431     | 537     | 555    | 561  | 60,009        |
| jsoniter        | 428     | 453     | 443    | 470  | 280,025       |
| easyjson        | 416     | 381     | 369    | 374  | 200,020       |
| encoding/json   | 126     | 137     | 135    | 136  | 260,030       |

| Marshal MB/s    | 0.4 KB    | 40 KB   | 400 KB    | 4 MB      | allocs @ 4 MB |
| --------------- | --------- | ------- | --------- | --------- | ------------- |
| **metago**      | **1,447** | **976** | **1,123** | **1,364** | **34**        |
| easyjson        | 775       | 978     | 1,029     | 1,041     | 140           |
| goccy/go-json   | 941       | 925     | 952       | 985       | 10,003        |
| encoding/json   | 583       | 601     | 620       | 621       | 50,002        |
| bytedance/sonic | 387       | 415     | 422       | 414       | 10,027        |
| jsoniter        | 334       | 346     | 349       | 350       | 80,015        |

Encode is fastest at every size, with near-constant allocations (2 → 34 across a 10,000x payload
range). Decode is fastest at small and medium sizes and within 1–4% of goccy at 400 KB–4 MB, with
3.7–5x fewer allocations than anything else. All in portable Go with no unsafe, generated from a
template. The decode allocation profile comes from a string arena: the lexer makes one lazy string
copy of the input and every unescaped string value is a zero-alloc substring of it (trade-off listed
under deviations below). One payload shape, one machine — treat this as "competitive with the fast
codegen libraries", not a general ranking.

Reproduce with:

```sh
go test -bench=. .
```

## Known deviations from encoding/json

Deliberate simplifications, all covered by tests where behavior matches:

- Marshal does not HTML-escape `<`, `>`, `&` (like most alternatives).
- Marshal writes map entries in map iteration order, not sorted.
- An empty JSON array decodes into a nil-preserving `[:0]` slice instead of a freshly allocated
  empty slice.
- Raw control characters inside string values are accepted rather than rejected.
- Invalid UTF-8 in Go strings is passed through on encode, not replaced with U+FFFD.
- Decoded strings are substrings of one shared copy of the input, so retaining any decoded string
  keeps that whole copy alive. For small long-lived extracts from huge payloads, `strings.Clone` the
  field after decoding.

## Regenerating

From the repository root:

```sh
go run . experiments/json
```

`easy_types_easyjson.go` is the easyjson competitor codec, generated with
`go run github.com/mailru/easyjson/easyjson -all easy_types.go`.
