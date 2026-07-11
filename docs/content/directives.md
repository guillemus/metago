---
title: Directives
weight: 20
description: Generate sidecar and inline Go code with Metago directives.
---

# Directives

Metago annotations use Go directive comments: `//mgo:` with no space. Gofmt preserves them and Go documentation hides them from rendered API docs.

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

All `//mgo:gen` directives in one package write to the same `meta.go` file.

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

A directive in a type, function, or method doc comment is **anchored**. Its target is inferred, so every token after the template name is an argument:

```go
//mgo:gen get /posts/{postID} auth=required
func (p PostRoutes) Show() {}
```

A separated directive is **standalone**. Its first bare token can explicitly name a local or imported target:

```go
type Status string

//mgo:inline stringer Status
```

Targets include `User`, `BuildUser`, `Server.Serve`, `server.Server`, and `net/http.Client.Do`. A first token beginning with `/` or containing `{` is always a positional argument.

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

Any namespace besides `gen`, `inline`, and `end` attaches metadata to the nearest type, field, method, function, or interface method:

```go
//mgo:api owner=core
type User struct {
    //mgo:validate required max=100
    Name string `json:"name"`
    Age int `json:"age"` //mgo:validate min=0 max=150
}
```

Repeated namespaces merge: flags are unioned and later values replace earlier ones. Read them with `prop`, `props`, `propHas`, and `propExists`.
