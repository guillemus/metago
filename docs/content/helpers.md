---
title: Template helpers
weight: 40
description: Reference for Metago template helper functions.
---

# Template helpers

Metago includes standard Go template functions plus helpers for metadata, names, tags, properties, types, imports, and collections.

## Metadata

| Helper | Result |
|---|---|
| `name .` | Name of an invocation, type, field, method, or value. |
| `typeof .` | Underlying or field type. |
| `keys .` | Field names, or sorted map keys. |
| `fieldNames .` | Target field names. |
| `methodNames .` | Comma-separated method names. |

## Struct tags

For `ID int \`json:"id,omitempty"\``:

| Helper | Result |
|---|---|
| `tag . "json"` | `id,omitempty` |
| `tagName . "json"` | `id` |
| `tagOpts . "json"` | Options after the first comma. |
| `tagHas . "json" "omitempty"` | Whether the option exists. |
| `tagExists . "json"` | Whether the tag key exists. |

## Properties

| Helper | Result |
|---|---|
| `prop . "validate" "max"` | A named property value, or empty string. |
| `props . "validate"` | The complete group with `.Args` and `.Argv`. |
| `propHas . "validate" "required"` | Whether a bare flag exists. |
| `propExists . "pii"` | Whether the namespace exists. |

These accept fields, types, methods, functions, and invocations.

## Field filters

`fieldsWithTag`, `fieldsWithoutTag`, `exportedFields`, `unexportedFields`, `embeddedFields`, and `nonEmbeddedFields` accept an invocation, `.Type`, or a field slice.

```go-html-template
{{ range exportedFields (fieldsWithTag . "json") }}
    {{ name . }}
{{ end }}
```

## Naming

| Helper | Example |
|---|---|
| `snake .Name` | `UserID` → `user_id` |
| `kebab .Name` | `UserID` → `user-id` |
| `camel .Name` | `user_id` → `userID` |
| `pascal .Name` | `user_id` → `UserID` |
| `initial .Name` | `User` → `u` |
| `receiver .` | `UserProfile` → `up` |
| `exported .Name` | Tests exported visibility. |
| `unexported .Name` | Lowercases the first rune. |

## Strings

`lower`, `upper`, `contains`, `hasPrefix`, `hasSuffix`, `trimPrefix`, `trimSuffix`, `replace`, `split`, `join`, and `quote` provide common string operations. `quote` emits a safe Go string literal.

## Types

Use `isString`, `isInt`, `isBool`, `isFloat`, `isSlice`, `isMap`, and `isPointer` for branches. `elem` returns the element of `[]T` or `*T`; `zero` returns a Go zero value.

```go-html-template
{{ if isPointer . }}
if value == nil { return {{ zero (elem .) }} }
{{ end }}
```

## Data

| Helper | Purpose |
|---|---|
| `dict "k" v` | Create a map. |
| `list "a" "b"` | Create a list. |
| `get m "key"` | Read a map key or exported struct field. |
| `arg 0`, `arg "key"` | Read directive arguments. |
| `default fallback value` | Replace a zero value. |
