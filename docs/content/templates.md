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
| `.Type`, `.Method`, `.Function` | Resolved target metadata. |
| `.Name`, `.TypeName`, `.Kind` | Target identity and kind. |
| `.Args`, `.Argv` | Named and positional arguments. |
| `.Fields`, `.Methods`, `.Functions` | Symbols visible to generation. |
| `.Params`, `.Results`, `.Body` | Function or method details. |
| `.IsType`, `.IsMethod`, `.IsFunction` | Target-kind booleans. |
| `.Values` | Constants discovered for a target type. |
| `.Package.Metas` | Package generation annotations in file/line order. |

Fields expose `.Name`, `.Type`, `.Tag`, `.Embedded`, and `.Props`. Methods expose receiver, parameters, results, body, and props. Interface methods have no receiver or body. Constant `.Value` is the source expression, not an evaluated number.

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
