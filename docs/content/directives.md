---
title: Directives
weight: 20
description: Generate sidecar and inline Go code with metago directives.
---

# Directives

metago annotations use Go directive comments: `//mgo:` with no space. Gofmt preserves them and Go documentation hides them from rendered API docs.

| Directive | Purpose |
|---|---|
| `//mgo:gen template [Target] [args]` | Render into the package `meta.go`. |
| `//mgo:inline template [Target] [args]` | Insert output in the source file. |
| `//mgo:end` | End an inline block; inserted automatically. |
| `//mgo:<namespace> [flags] [key=value]` | Attach metadata without generating code. |

`// mgo:gen` is prose and is ignored because it contains a space.

## Sidecar generation

```go
//mgo:gen stringer
type Status string
```

Directives in ordinary source files write to `meta.go`. Directives in internal test files write to
`meta_test.go`; directives in an external `<package>_test` package write to
`meta_<package>_test.go`. This keeps generated test helpers out of production builds while allowing
both Go test package styles in one directory.

## Inline generation

```go
//mgo:inline stringer
type Status string
```

After generation, output follows the symbol and ends at an automatically managed marker:

```go
//mgo:inline stringer
type Status string

func (s Status) String() string { return string(s) }

//mgo:end
```

Later runs replace only this generated region. Inline templates can call `imports`; missing imports are added to the source file.

## Anchored and standalone

A directive in a type, function, method, package-level const, or package-level var doc comment is
**anchored**. Its target is inferred, so every token after the template name is an argument:

```go
//mgo:gen get /posts/{postID} auth=required
func (p PostRoutes) Show() {}
```

A separated directive is **standalone**. Its first bare token can explicitly name a local or imported target:

```go
type Status string

//mgo:inline stringer Status
```

Targets include types, functions, methods, and package-level values: `User`, `BuildUser`,
`DefaultTimeout`, `Server.Serve`, `server.Server`, `server.DefaultTimeout`, and
`net/http.Client.Do`. A first token beginning with `/` or containing `{` is always a positional
argument.

A const/var declaration containing multiple names has no single inferred target, so use the
standalone form and name the value explicitly:

```go
//mgo:gen describe First
const First, Second = 1, 2
```

A directive attached to one spec inside a parenthesized const/var block is anchored to that value.
For `//mgo:inline`, generated output is inserted after the complete declaration block so it remains
valid package-level Go:

```go
const (
    //mgo:inline describe
    DefaultTimeout = 30
)
```

## Stack directives

Directives compose in source order:

```go
//mgo:inline signals
//mgo:gen validator
//mgo:api owner=core
type AuthSignals struct {
    Email string `json:"sig_email"`
}
```

Property lines must follow generation lines within the same comment block.

## Property namespaces

Any namespace besides an implemented or reserved directive is a property namespace. Properties must
be syntactically attached to a type, field, method, function, interface method, package-level const,
or package-level var:

```go
//mgo:api owner=core
type User struct {
    //mgo:validate required max=100
    Name string `json:"name"`
    Age int `json:"age"` //mgo:validate min=0 max=150
}
```

A separated property is not attached to the nearest declaration and package properties are not yet
supported:

```go
//mgo:api owner=core

func Unrelated() {}
```

This reports `property "api" has no symbol to attach to`. Repeated namespaces on one symbol merge:
flags are unioned and later values replace earlier ones. Read them with `prop`, `props`, `propHas`,
and `propExists`.

Property flags and `key=value` arguments are user-owned and unrestricted.

## Project template defaults

An optional `metago.toml` directly inside the root passed to metago supplies project-wide defaults
for named template arguments:

```toml
[templates."std.serde".args]
runtime = "example.com/project/internal/serdejson"
```

Explicit `key=value` arguments on `//mgo:gen` and `//mgo:inline` override these values. All configured
values must currently be quoted TOML strings. The file does not configure positional arguments,
bare flags, discovery, output, or other metago behavior. metago reads only this one location and does
not search for or merge other configuration files.

## Reserved names

The following directive names are reserved for future metago features and cannot be used as property
namespaces:

```text
build config file format generate import include option options output package plugin profile use
```

Using one before it is implemented reports, for example,
`directive "output" is reserved for future metago features`.

Named arguments on `gen` and `inline` reserve these keys:

```text
build dir file format group mode order output package scope tags
```

The metago namespace is also reserved: `mgo`, `mgo.*`, `mgo_*`, and `mgo-*`. For example,
`file=generated.go` reports `argument "file" is reserved for future metago features`. These
restrictions apply only to `key=value` generation arguments; positional arguments and all property
arguments remain unrestricted.
