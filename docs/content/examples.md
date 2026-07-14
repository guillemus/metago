---
title: Examples
description: Complete Metago patterns ready to adapt.
eyebrow: Patterns
---

# Examples

Small, complete patterns that show how directives, metadata, and templates fit together. Copy one, rename its template, and adapt the generated API to your project.

## Enum stringer

Use the embedded template when you need the conventional implementation:

```go
package status

//mgo:gen std.stringer trimprefix=Status
type Status int

const (
    StatusPending Status = iota
    StatusRunning
    StatusDone
)
```

Running `metago .` generates `String() string`. For unknown values it returns `Status(value)`.

A custom template gives you complete control:

```go-html-template
{{ define "short-status" }}
func (v {{ name . }}) String() string {
    switch v {
    {{- range .Values }}
    case {{ .Name }}:
        return {{ quote (trimPrefix .Name "Status") }}
    {{- end }}
    default:
        return "unknown"
    }
}
{{ end }}
```

```go
//mgo:gen short-status
type Status int
```

## Interface mock

```go
package users

//mgo:gen std.mock
type Store interface {
    Find(id string) (User, error)
    Save(user User) error
}
```

Metago generates a `MockStore` with one function field per method:

```go
func TestService(t *testing.T) {
    store := MockStore{
        FindFunc: func(id string) (User, error) {
            return User{ID: id}, nil
        },
        SaveFunc: func(user User) error {
            return nil
        },
    }

    service := NewService(store)
    // ...
}
```

Use named, non-variadic interface parameters. Embedded interface methods are not expanded by the standard mock template.

## Reflection-free JSON

Generate the project-owned runtime once:

```go
// Package jsonruntime contains generated JSON support.
//
//mgo:gen std.serde.jsonruntime
package jsonruntime
```

Set the runtime import for every codec in `metago.toml`:

```toml
[templates."std.serde".args]
runtime = "example.com/project/internal/jsonruntime"
strict = "true"
maxinput = "1048576"
```

Derive codecs on models:

```go
package users

//mgo:gen std.serde
type User struct {
    ID      int64    `json:"id"`
    Name    string   `json:"name"`
    Tags    []string `json:"tags,omitempty"`
    Address Address  `json:"address"`
}

//mgo:gen std.serde
type Address struct {
    City string `json:"city"`
}
```

The generated types implement `json.Marshaler` and `json.Unmarshaler`. Nested generated types call each other directly. Unsupported field shapes fall back to `encoding/json` for that field.

## Struct map codec

```go
package config

//mgo:gen std.mapstruct allowmissing
type Server struct {
    Host string `mapstructure:"host,required"`
    Port int    `mapstructure:"port"`
    TLS  TLS    `mapstructure:"tls"`
}

type TLS struct {
    Enabled bool   `mapstructure:"enabled"`
    CertFile string `mapstructure:"cert_file"`
}
```

Generated code decodes exact Go values transactionally:

```go
var server Server
err := server.Decode(map[string]any{
    "host": "127.0.0.1",
    "port": 8080,
    "tls": map[string]any{
        "enabled": true,
    },
})

encoded := server.Encode()
```

`allowmissing` makes untagged fields optional; the `required` tag option keeps `host` mandatory.

## Validation properties

Properties let one declaration feed multiple generators without coupling it to a fixed schema:

```go
//mgo:gen validate
type Signup struct {
    Email string `json:"email"` //mgo:validate required format=email
    Name  string `json:"name"`  //mgo:validate required max=80
    Age   int    `json:"age"`   //mgo:validate min=18
}
```

```go-html-template
{{ define "validate" }}
func (v {{ name . }}) Validate() error {
    {{- range .Fields }}
    {{- if propHas . "validate" "required" }}
    if v.{{ .Name }} == {{ zero . }} {
        return fmt.Errorf({{ quote (printf "%s is required" (tagName . "json")) }})
    }
    {{- end }}
    {{- end }}
    return nil
}
{{ imports "fmt" }}
{{ end }}
```

The `imports` call can appear after the generated code; it records the import and emits no text.

## Package registry

One aggregate invocation can inspect marker directives across a package:

```go
//mgo:gen route-table
package api

//mgo:gen route GET /users
func ListUsers() {}

//mgo:gen route POST /users
func CreateUser() {}
```

```go-html-template
{{ define "route" }}{{ end }}

{{ define "route-table" }}
type Route struct {
    Method string
    Path   string
}

var Routes = []Route{
{{- range .Package.Metas }}
{{- if eq .Template "route" }}
    {Method: {{ quote (index .Argv 0) }}, Path: {{ quote (index .Argv 1) }}},
{{- end }}
{{- end }}
}
{{ end }}
```

The empty `route` template acts as a marker. `.Package.Metas` remains stable in file and line order.

## Inline generated code

Inline output is useful when generated declarations are easier to understand beside their source:

```go
//mgo:inline constructor
type Client struct {
    BaseURL string
    Token   string
}
```

```go-html-template
{{ define "constructor" }}
func New{{ name . }}(
{{- range .Fields }}
    {{ unexported .Name }} {{ typeof . }},
{{- end }}
) *{{ name . }} {
    return &{{ name . }}{
    {{- range .Fields }}
        {{ .Name }}: {{ unexported .Name }},
    {{- end }}
    }
}
{{ end }}
```

Metago inserts the constructor after `Client` and closes the managed region with `//mgo:end`. Later runs replace only that region.

## Test-only generation

Directives in test files stay in test builds:

```go
package service_test

//mgo:gen std.mock
// Define a test-only interface with named parameters.
type Clock interface {
    Now() time.Time
}
```

For an external `service_test` package, Metago writes `meta_service_test.go`. Internal test directives write `meta_test.go`. Production `meta.go` remains unaffected.

For exact supported targets, helper signatures, and standard-template options, use the [reference](/reference/).
