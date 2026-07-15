# Serde

Serde is a reflection-free JSON coder-decoder generated entirely by metago templates. Its name and
annotation API are inspired by Rust's Serde: annotate a type with `serde` to derive its codec. The
standard template, generated example, behavioral tests, and benchmarks live together in `std/serde`
as part of the main metago module.

There is **no external runtime library**. `std.serde.jsonruntime` generates the shared runtime in a
project package, while `std.serde` generates codecs that import it. The repository's root
`metago.toml` demonstrates setting that runtime import once for every codec invocation.

## How it works

Runtime package:

```go
// Package jsonruntime contains the generated shared JSON support.
//
//mgo:gen std.serde.jsonruntime
package jsonruntime
```

Project configuration:

```toml
[templates."std.serde".args]
runtime = "example.com/project/internal/jsonruntime"
```

Model package:

```go
//mgo:gen std.serde
type User struct {
	ID   int64    `json:"id"`
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}
```

Without a configured `runtime` argument, codecs instead expect the runtime to have been generated in
the same package. An explicit directive argument overrides `metago.toml`.

Set `strict=true` on `std.serde`—directly or through the same template-default configuration—to
reject unknown object fields. The default is `false`, matching `encoding/json`; unknown values are
still fully syntax-validated before being ignored. Values other than `true` or `false` fail
generation.

Use `maxinput=N` to reject JSON inputs larger than `N` bytes before decoding allocates receiver
state. It is disabled when omitted or zero. Use `maxdepth=N` to override the default 10,000-level
nesting limit; zero retains that default. Both arguments are validated as unsigned 64-bit integers
during generation and apply to generated recursion, unknown values, raw messages, and fallback
fields during decoding. An input byte cap also bounds the bytes available to decoded strings and
collections; application-specific semantic length limits belong in validation after decoding.

- `std.serde.jsonruntime` emits `Lexer`, a cursor over the input buffer with error latching and an
  exact fast-path float parser, plus `AppendString` for encoding.
- `std.serde` derives `MarshalJSON`/`UnmarshalJSON` per type: a byte-appending encoder and a
  key-switch decoder.
- Codec imports are registered only by emitted branches; isolated generation tests assert exact
  stable import sets for native, fallback, embedded, strict, and configured codecs.
- Nested annotated types are discovered through `.Package.Metas`, so `User` calls
  `Address.unmarshalJSONLexer` directly — reflection-free recursion.
- Handled natively: built-in and methodless named scalars across direct fields, pointers through
  three levels, slices (including double-pointer elements), fixed arrays, and string-keyed scalar
  maps (including triple-pointer values); annotated types across fields and supported containers;
  bytes and raw messages. Anything else falls back to `encoding/json` for that field only. That
  fallback honors field types implementing `json.Marshaler`, `json.Unmarshaler`,
  `encoding.TextMarshaler`, or `encoding.TextUnmarshaler`, including pointer allocation and `null`
  behavior.
- Structs containing anonymous fields deliberately use a whole-struct `encoding/json` fallback so
  Go's promotion and dominance rules remain canonical. Generated decoders clone existing anonymous
  pointers before that fallback, preserving receiver state if decoding fails.

## Reliability suite

The package includes table-driven compatibility tests, regression tests, property tests, and fuzz
targets. The suite covers RFC 8259, Go's `encoding/json`, and relevant behavior from serde_json,
Sonic, goccy/go-json, jsoniter, and easyjson. It also runs every accepted and rejected JSONTestSuite
parsing subject from the pinned, digest-verified local archive.

`testdata` contains the curated fixtures and `testdata/PROVENANCE.md` records their upstream source,
revision, adaptation, and licensing. The tests assert the codec's required behavior directly, while
fuzz targets compare supported decoding and semantic encoder output against `encoding/json`.

## Benchmarks

User feeds of 1 / 100 / 1,000 / 10,000 users (~0.4 KB / 40 KB / 400 KB / 4 MB), Apple M4 Pro, Go
1.26. Throughput in MB/s (higher is better), allocs/op alongside:

| Unmarshal MB/s  |  0.4 KB |   40 KB |  400 KB |    4 MB | allocs @ 4 MB |
| --------------- | ------: | ------: | ------: | ------: | ------------: |
| **serde**       | **755** | **828** | **875** | **967** |    **20,004** |
| goccy/go-json   |     512 |     542 |     553 |     528 |       150,017 |
| bytedance/sonic |     425 |     512 |     509 |     512 |        60,010 |
| jsoniter        |     434 |     429 |     438 |     405 |       280,025 |
| easyjson        |     379 |     365 |     360 |     342 |       200,020 |
| encoding/json   |     123 |     129 |     125 |     122 |       260,030 |

| Marshal MB/s    |    0.4 KB |     40 KB |    400 KB |      4 MB | allocs @ 4 MB |
| --------------- | --------: | --------: | --------: | --------: | ------------: |
| **serde**       | **1,147** | **1,029** | **1,109** | **1,160** |         **1** |
| easyjson        |       712 |       910 |       965 |       969 |           140 |
| goccy/go-json   |       855 |       869 |       918 |       933 |        10,003 |
| encoding/json   |       527 |       567 |       584 |       581 |        50,002 |
| bytedance/sonic |       345 |       370 |       390 |       382 |        10,027 |
| jsoniter        |       304 |       324 |       326 |       322 |        80,018 |

Encode and decode are fastest at every measured feed size. Encode performs one allocation for the
returned buffer. All serde implementation code is portable Go with no unsafe and is generated from a
template. Retained decoded strings share a decoder-owned buffer, never the input, so callers may
reuse or release the input without changing decoded fields or pinning the complete payload. Nested
generated slices use package-typed backing slabs; the user-facing types and API are unchanged.

The compatibility-shape benchmark covers escaped/Unicode strings, a 64 KiB byte payload plus scalar
containers, nested pointer maps, sparse nil/empty values, and numeric boundaries. A three-sample
profile on the same Apple M4 Pro and Go 1.26.2 produced these median results:

| Shape                   | Marshal MB/s (allocs) | Unmarshal MB/s (allocs) |
| ----------------------- | --------------------: | ----------------------: |
| Escaped strings         |             1,247 (5) |                749 (15) |
| Scalar containers       |             3,617 (4) |              1,545 (12) |
| Pointer and nested maps |             2,307 (5) |                934 (15) |
| Sparse nil/empty        |             2,984 (3) |               1,039 (8) |
| Numeric boundaries      |             1,501 (2) |                  28 (1) |

These profiles cover multiple payload shapes on one machine; treat the comparative feed table as
evidence of competitiveness, not a general library ranking.

Reproduce with:

```sh
go test -bench=. ./std/serde
go test -bench=BenchmarkCompatibilityShapes -benchmem ./std/serde
```

## Compatibility policy

No intentional JSON behavior deviation from `encoding/json` is currently retained. Historical
differences—HTML escaping, map ordering, empty-slice decoding, raw control characters, malformed
UTF-8 output, and input-pinning strings—now follow the canonical behavior and have executable
coverage. Unsupported generated field shapes may still use the documented per-field `encoding/json`
fallback.

## Regenerating

From the repository root:

```sh
go run . .
```

`easy_types_easyjson.go` is the easyjson competitor codec, generated with
`go run github.com/mailru/easyjson/easyjson -all easy_types.go`.
