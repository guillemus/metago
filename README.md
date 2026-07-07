# Metago

Metago is a Go code generation tool meant to be used alongside your code before compiling. You write
small annotations in Go comments, write reusable Go `text/template` templates in `.metago` files,
then run `metago` to generate or update ordinary Go source.

## Usage

```sh
metago              # scan current directory recursively
metago ./path       # scan another root recursively
metago -v           # verbose logs
metago --verbose
```

Successful runs are silent by default.

Templates can live anywhere under the root you pass to `metago`. For example, running `metago .` can
use templates from `metago/stringer.metago`, `views/fields.metago`, or any other `.metago` file
under `.`. Template names come from `{{ define "name" }}` blocks.

## Annotation modes

### Generate a sidecar file: `//#`

```go
//#stringer Status
type Status string
```

If this is in `status.go`, Metago writes `status_meta.go`.

### Inline into the same file: `//@`

```go
type Status string

//@stringer Status
```

After running Metago:

```go
type Status string

//@stringer Status

func (s Status) String() string { return string(s) }

//end
```

Metago inserts `//end` automatically. On later runs, it replaces the block between `//@...` and
`//end`. Inline templates cannot use `imports`.

`//#` and `//@` must have no space after `//`. `// #...` is ignored.

## Annotation syntax

```text
//#templateName TargetName key=value
//@templateName TargetName key=value
```

`TargetName` is optional. If omitted, Metago uses the nearest type.

## Template example

```gotemplate
{{ define "stringer" }}
func ({{ receiver . }} {{ name . }}) String() string {
    return string({{ receiver . }})
}
{{ end }}
```

## Template data

Each template receives:

| Field                 | Meaning                              |
| --------------------- | ------------------------------------ |
| `.Package`            | Package metadata.                    |
| `.Meta`               | Current annotation metadata.         |
| `.Type`               | Target type metadata.                |
| `.Name` / `.TypeName` | Target type name.                    |
| `.Kind`               | `struct`, `interface`, or `type`.    |
| `.Args`               | Annotation key/value args.           |
| `.Fields`             | Struct fields.                       |
| `.Methods`            | Methods on the target type.          |
| `.Values`             | Typed constants for enum-like types. |

## Utilities

Metago templates include normal Go template funcs like `printf`, `len`, `index`, `eq`, `and`, and
`or`, plus these helpers.

### Metadata

| Helper          | Does                                                             | Use when                  |
| --------------- | ---------------------------------------------------------------- | ------------------------- |
| `name .`        | Returns the name of a type, field, method, value, or invocation. | Emitting Go identifiers.  |
| `typeof .`      | Returns the underlying type, field type, or value type.          | Type-specific generation. |
| `keys .`        | Field names for types/invocations, sorted keys for maps.         | Stable output ordering.   |
| `fieldNames .`  | Field names for a type/invocation.                               | Building field lists.     |
| `methodNames .` | Comma-joined method names.                                       | Summaries/debug output.   |

### Imports

| Helper                              | Does                                                  | Use when                        |
| ----------------------------------- | ----------------------------------------------------- | ------------------------------- |
| `imports "strconv"`                 | Adds an import to sidecar output; emits empty string. | Generated code needs imports.   |
| `imports "encoding/json" "stdjson"` | Adds an aliased import.                               | Avoiding import name conflicts. |

### Struct tags

| Helper                        | Does                                | Use when                               |
| ----------------------------- | ----------------------------------- | -------------------------------------- |
| `tag . "json"`                | Raw tag value, e.g. `id,omitempty`. | You need the full tag.                 |
| `tagName . "json"`            | First tag part, e.g. `id`.          | JSON/db/form field names.              |
| `tagOpts . "json"`            | Options after the first comma.      | Option-driven behavior.                |
| `tagHas . "json" "omitempty"` | Checks if a tag option exists.      | Handling `omitempty`, `string`, etc.   |
| `tagExists . "json"`          | Checks if the tag key exists.       | Distinguishing absent vs present tags. |

Example:

```go
ID int `json:"id,omitempty"`
```

```gotemplate
{{ tag . "json" }}      {{/* id,omitempty */}}
{{ tagName . "json" }}  {{/* id */}}
{{ tagHas . "json" "omitempty" }}
```

### Field filters

These accept `.`, `.Type`, or a `[]Field`.

| Helper                      | Does                      | Use when                      |
| --------------------------- | ------------------------- | ----------------------------- |
| `fieldsWithTag . "json"`    | Fields with a tag key.    | JSON/db/form mappers.         |
| `fieldsWithoutTag . "json"` | Fields without a tag key. | Filling defaults.             |
| `exportedFields .`          | Exported fields.          | Cross-package generated code. |
| `unexportedFields .`        | Unexported fields.        | Same-package helpers.         |
| `embeddedFields .`          | Embedded fields.          | Flattening/forwarding.        |
| `nonEmbeddedFields .`       | Non-embedded fields.      | Normal struct field loops.    |

### Naming

| Helper             | Does                           | Use when                   |
| ------------------ | ------------------------------ | -------------------------- |
| `snake .Name`      | `UserID` → `user_id`.          | DB columns, JSON defaults. |
| `kebab .Name`      | `UserID` → `user-id`.          | HTML/CSS/CLI names.        |
| `camel .Name`      | `user_id` → `userId`.          | JS-facing names.           |
| `pascal .Name`     | `user_id` → `UserId`.          | Exported Go identifiers.   |
| `initial .Name`    | `User` → `u`.                  | Short receivers.           |
| `receiver .`       | `UserProfile` → `up`.          | Method receivers.          |
| `exported .Name`   | True if name starts uppercase. | Visibility checks.         |
| `unexported .Name` | Lowercases first rune.         | Private helper names.      |

### Strings

| Helper                | Does               | Use when              |
| --------------------- | ------------------ | --------------------- |
| `lower s`             | Lowercase.         | Simple names.         |
| `upper s`             | Uppercase.         | Constants/text.       |
| `contains s sub`      | Substring check.   | Conditional output.   |
| `hasPrefix s prefix`  | Prefix check.      | Name conventions.     |
| `hasSuffix s suffix`  | Suffix check.      | Name conventions.     |
| `trimPrefix s prefix` | Remove prefix.     | Deriving names.       |
| `trimSuffix s suffix` | Remove suffix.     | Deriving names.       |
| `replace s old new`   | Replace all.       | Name cleanup.         |
| `split s sep`         | Split string.      | Small lists.          |
| `join list sep`       | Join strings.      | Emitting lists.       |
| `quote s`             | Go-quote a string. | Safe string literals. |

### Types

| Helper        | Does                         | Use when                   |
| ------------- | ---------------------------- | -------------------------- |
| `isString .`  | Type is `string`.            | String-specific code.      |
| `isInt .`     | Type is an int/uint.         | Numeric code.              |
| `isBool .`    | Type is `bool`.              | Boolean code.              |
| `isFloat .`   | Type is `float32`/`float64`. | Numeric code.              |
| `isSlice .`   | Type starts with `[]`.       | Collections.               |
| `isMap .`     | Type starts with `map[`.     | Maps.                      |
| `isPointer .` | Type starts with `*`.        | Nil checks/deref.          |
| `elem .`      | Element of `[]T` or `*T`.    | Collection/pointer code.   |
| `zero .`      | Go zero value.               | Defaults and initializers. |

### Data

| Helper                   | Does                                      | Use when                          |
| ------------------------ | ----------------------------------------- | --------------------------------- |
| `dict "k" v`             | Creates a map.                            | Passing data to nested templates. |
| `list "a" "b"`           | Creates a list.                           | Inline enumerations.              |
| `get m "key"`            | Reads a map key or exported struct field. | Optional/dynamic lookups.         |
| `default fallback value` | Returns fallback if value is zero.        | Optional args.                    |

Examples:

```gotemplate
{{ default "users" (get .Args "table") }}
{{ get .Args "table" | default "users" }}
```

## Testing

```sh
go test ./...
UPDATE_GOLDEN=3 go test ./...
```
