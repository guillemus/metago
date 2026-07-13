---
title: Templates and metadata
weight: 30
description: Understand Metago templates, invocation data, and package aggregation.
---

# Templates and metadata

A `*.metago` file uses Go's `text/template` syntax. Every named definition is available to a matching directive.

```go-html-template
{{ define "stringer" }}
func ({{ receiver . }} {{ name . }}) String() string {
    return string({{ receiver . }})
}
{{ end }}
```

## Invocation data

| Field | Meaning |
|---|---|
| `.Package` | Package metadata. |
| `.Meta` | Current annotation. |
| `.Type`, `.Method`, `.Function`, `.Value` | Resolved target metadata. |
| `.Name`, `.TypeName`, `.Kind` | Target identity and kind; package-scoped invocations have kind `package` and no target name. |
| `.Args`, `.Argv` | Named and positional arguments. |
| `.Fields`, `.Methods`, `.Functions` | Symbols visible to generation. |
| `.Params`, `.Results`, `.Body` | Function or method details. |
| `.Expr` | Const/var initializer source text. |
| `.IsPackage` | Package-scoped invocation boolean. |
| `.IsType`, `.IsMethod`, `.IsFunction` | Type, method, and function target booleans. |
| `.IsValue`, `.IsConst`, `.IsVar` | Package value target booleans. |
| `.Values` | Constants discovered for a target type. |
| `.Package.Metas` | Package generation annotations in file/line order. |
| `.Package.Values` | All package-level const and var symbols. |

Fields expose `.Name`, `.Type`, `.Tag`, `.Embedded`, and `.Props`. Methods expose receiver,
parameters, results, body, and props. Interface methods have no receiver or body. Value metadata
exposes `.Name`, `.Type`, `.Value`, `.Expr`, `.Kind`, and `.Props`; `.Value` and `.Expr` are the
initializer's source expression, not an evaluated value. `.Type` is source-level declared type text
and is empty for inferred or untyped declarations.

For example:

```go-html-template
{{ define "describe-value" }}
const {{ .Name }}Description = {{ quote (printf "%s %s = %s" .Kind .TypeName .Expr) }}
{{ end }}
```

## Arguments

```text
//mgo:gen templateName positional table=users
```

Use `.Argv` or `arg 0` for positional arguments. Use `.Args`, `arg "table"`, or `get .Args "table"` for named arguments.

```go-html-template
{{ default "users" (arg "table") }}
```

## Aggregate a package

`.Package.Metas` lets one template build registries, route tables, and other artifacts from many annotations:

```go-html-template
{{ define "server" }}
func (s Server) Routes() []string {
    return []string{
    {{- range .Package.Metas }}
    {{- if or (eq .Template "get") (eq .Template "post") }}
        "{{ upper .Template }} {{ index .Argv 0 }}",
    {{- end }}
    {{- end }}
    }
}
{{ end }}
```

Empty templates are valid and can act only as aggregate markers. Property annotations are attached to symbols and do not appear in `.Package.Metas`.

## Imports

The `imports` helper records required imports and emits an empty string:

```go-html-template
{{ imports "strconv" }}
{{ imports "encoding/json" "stdjson" }}
```

Place it inside the branch that emits code requiring that import.

## Template diagnostics

Templates can reject unsupported invocations:

```go-html-template
{{ fail "requires an integer-backed type" }}
```

`fail` stops only the current invocation and discards all of its output and registered imports.
Metago continues executing other directives so it can report all failures together. If any
invocation fails, the command exits unsuccessfully without writing generated files.
