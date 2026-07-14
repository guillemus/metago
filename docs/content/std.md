---
title: Built-in templates
description: The templates embedded in Metago's std namespace.
toc: true
eyebrow: Standard library
---

# Built-in templates

Metago embeds its standard templates in the binary. They work without local `.metago` files and use the reserved `std.` namespace, which user templates cannot define.

| Template | Generates |
| --- | --- |
| [`std.stringer`](#stdstringer) | A `String` method for primitive-backed types. |
| [`std.enum`](#stdenum) | String conversion, parsing, validation, values, and JSON for enums. |
| [`std.mock`](#stdmock) | Function-backed mocks for interfaces. |
| [`std.mapstruct`](#stdmapstruct) | Typed struct-to-map encoding and decoding. |
| [`std.serde`](#stdserde) | Reflection-free JSON codecs. |
| [`std.serde.jsonruntime`](#stdserdejsonruntime) | The shared, project-owned runtime used by `std.serde`. |

## `std.stringer`

Generate `String() string` for a primitive-backed defined type:

```go
//mgo:gen std.stringer trimprefix=Status
type Status int

const (
    StatusPending Status = iota
    StatusRunning
    StatusDone
)
```

Declared typed constants become switch cases:

```go
func (v Status) String() string {
    switch v {
    case StatusPending:
        return "Pending"
    case StatusRunning:
        return "Running"
    case StatusDone:
        return "Done"
    default:
        return "Status(" + strconv.FormatInt(int64(v), 10) + ")"
    }
}
```

Supported underlying types are string, bool, signed and unsigned integers, floats, and complex numbers. Unknown values use `Type(value)`; unknown strings are quoted.

| Argument | Default | Behavior |
| --- | --- | --- |
| `trimprefix=value` | No trimming | Removes the prefix from constant names returned by `String`. |

The target must be primitive-backed. Other targets fail generation.

## `std.enum`

Generate the conventional enum API from a defined type and its typed constants:

```go
//mgo:gen std.enum
type Status int

const (
    StatusPending Status = iota
    StatusRunning
    StatusDone
)
```

The generated API is:

```go
func (v Status) String() string
func ParseStatus(value string) (Status, error)
func (v Status) Valid() bool
func StatusValues() []Status
func (v Status) MarshalJSON() ([]byte, error)
func (v *Status) UnmarshalJSON(data []byte) error
```

Supported underlying types are strings, signed integers, unsigned integers, and floats. The type must have at least one discovered typed constant.

Integer and float enums strip the type name from constant names by default. String enums use each constant's string value. JSON uses the same string form and rejects unknown values.

| Argument | Default | Behavior |
| --- | --- | --- |
| `trimprefix=value` | The target type name | Changes the prefix removed from integer and float constant names. |

## `std.mock`

Generate a function-backed mock for an interface:

```go
//mgo:gen std.mock
type Store interface {
    Get(id string) (User, error)
    Save(user User) error
}
```

The generated mock has one function field per discovered method and forwarding methods that satisfy the interface:

```go
type MockStore struct {
    GetFunc  func(id string) (User, error)
    SaveFunc func(user User) error
}

func (m *MockStore) Get(id string) (User, error) {
    return m.GetFunc(id)
}

func (m *MockStore) Save(user User) error {
    return m.SaveFunc(user)
}
```

Assign the function fields directly in tests:

```go
store := &MockStore{
    GetFunc: func(id string) (User, error) {
        return User{ID: id}, nil
    },
    SaveFunc: func(user User) error {
        return nil
    },
}
```

Interface method parameters should be named. Embedded interface methods are not expanded, and variadic forwarding is not specially handled.

## `std.mapstruct`

Generate typed conversion between a struct and `map[string]any`:

```go
//mgo:gen std.mapstruct allowmissing
type Config struct {
    Host string `mapstructure:"host,required"`
    Port int    `mapstructure:"port"`
}
```

The generated API is:

```go
func (v *Config) Decode(input map[string]any) error
func (v *Config) Encode() map[string]any
```

The template:

- Operates on exported fields.
- Uses `mapstructure` tag names.
- Ignores `mapstructure:"-"` fields.
- Recurses into local named struct fields.
- Requires nested inputs to be `map[string]any`.
- Uses exact Go type assertions instead of numeric or string conversions.
- Decodes transactionally, updating the receiver only after every field succeeds.

By default every included field is required. The positional `allowmissing` flag makes fields optional unless their `mapstructure` tag contains `required`.

```go
var config Config
err := config.Decode(map[string]any{
    "host": "127.0.0.1",
    "port": 8080,
})

encoded := config.Encode()
```

## `std.serde`

Generate reflection-free `MarshalJSON` and `UnmarshalJSON` methods plus package-private helpers:

```go
//mgo:gen std.serde
type User struct {
    ID   int64    `json:"id"`
    Name string   `json:"name"`
    Tags []string `json:"tags,omitempty"`
}
```

Generated paths cover built-in and methodless named scalars, pointers, slices, arrays, bytes, `json.RawMessage`, string-keyed maps, nested generated types, and common combinations of those shapes.

Unsupported fields use `encoding/json` and continue to support `json.Marshaler`, `json.Unmarshaler`, `encoding.TextMarshaler`, and `encoding.TextUnmarshaler`. Structs containing anonymous fields use `encoding/json` for the complete struct.

Serde follows `encoding/json` behavior for field names and visibility, `-`, `omitempty`, `omitzero`, and supported `string` options. Decode failures are transactional, and retained decoded strings do not alias the input. Recursive generated pointers and containers detect cycles during encoding. Errors include field, Go type, JSON kind, and offset context.

| Argument | Default | Behavior |
| --- | --- | --- |
| `runtime=import/path` | Same package | Imports the generated runtime using the internal alias `serdejsonruntime`. |
| `strict=true\|false` | `false` | Rejects unknown object fields when true. Otherwise unknown values are skipped after full syntax validation. |
| `maxinput=N` | Disabled | Rejects input larger than `N` bytes before receiver-state allocation. Zero disables the cap. |
| `maxdepth=N` | `10000` | Sets the maximum JSON nesting depth. Zero keeps the default. |

`strict` accepts only `true` or `false`. `maxinput` and `maxdepth` must be unsigned 64-bit decimal integers. Invalid values fail generation. Directive-local arguments override defaults from `metago.toml`.

### Shared runtime

For larger projects, generate the runtime once in a dedicated package:

```go
// Package jsonruntime contains generated JSON support.
//
//mgo:gen std.serde.jsonruntime
package jsonruntime
```

Then configure its import path:

```toml
[templates."std.serde".args]
runtime = "example.com/project/internal/jsonruntime"
```

Nested generated types call one another directly. Every package deriving codecs should resolve the same runtime import.

Without a `runtime` argument, codecs expect `std.serde.jsonruntime` to be generated in the same package.

## `std.serde.jsonruntime`

`std.serde.jsonruntime` generates the shared runtime used by `std.serde`.

Generate it once in the runtime package:

```go
//mgo:gen std.serde.jsonruntime
package jsonruntime
```

Application code uses the `MarshalJSON` and `UnmarshalJSON` methods generated by `std.serde`.

For compatibility policy, reliability tests, and benchmarks, see the [`std/serde` implementation notes](https://github.com/guillemus/metago/tree/main/std/serde).
