---
title: Reference
description:
    Complete reference for Metago directives, metadata, helpers, configuration, and standard
    templates.
eyebrow: API reference
---

# Reference

The complete public surface of Metago in one searchable page. Use browser find to jump directly to a
directive, metadata field, helper, argument, or standard template.

## Contents

- [Command and discovery](#usage)
- [Project configuration](#project-template-defaults)
- [Directives and target resolution](#directives)
- [Annotation syntax](#annotation-syntax)
- [Template data](#template-data)
- [Template helpers](#utilities)
- [Standard templates](#standard-templates)

## Usage

```sh
metago              # scan the current directory, find all templates, and generate code
metago ./path       # scan another root and find all its templates
metago -v           # show verbose logs
metago --verbose
```

From the project root, invoke `metago` without arguments. It automatically discovers every `.metago`
template beneath that root; you do not need to name or pass template files. Metago accepts one
optional scan root, is silent on success unless verbose logging is enabled, and requires at least
one ordinary Go package beneath that root.

Templates can live anywhere under the root you pass to `metago`, except inside `vendor`, `testdata`,
or hidden directories. For example, running `metago` from a project root can use templates from
`metago/stringer.metago` or `views/fields.metago`. Template names come from `{{ define "name" }}`
blocks and must be unique across the scan root. User templates cannot use the reserved `std.`
prefix.

Package discovery follows the same directory exclusions. Within a package, Metago scans ordinary and
`_test.go` files while ignoring generated Metago sidecars. Test directives generate test-only
sidecars: `meta_test.go` for the package under test and `meta_<package>_test.go` for its external
`<package>_test` package.

Generation is atomic across the scan root. If any package fails, Metago changes no files. Successful
runs remove stale Metago-generated sidecars and preserve other files.

## Project template defaults

`metago.toml` configures default named arguments for your templates and must live at the project
root:

```toml
[templates."std.serde".args]
runtime = "example.com/project/internal/serdejson"
```

Explicit arguments on `//mgo:gen` and `//mgo:inline` override configured defaults.

## Directives

Metago annotations start with `//mgo:` and contain no space after `//`:

```go
//mgo:gen stringer
```

The form `// mgo:gen stringer` is ignored.

| Directive                               | Purpose                                                      |
| --------------------------------------- | ------------------------------------------------------------ |
| `//mgo:gen template [Target] [args]`    | Run a template; write output to the package `meta.go`.       |
| `//mgo:inline template [Target] [args]` | Run a template; insert output inline, up to `//mgo:end`.     |
| `//mgo:end`                             | Terminates an inline block. Inserted automatically.          |
| `//mgo:<namespace> [flags] [key=value]` | Attach metadata to its documented symbol. Generates nothing. |

A name that is neither an implemented nor reserved directive is a property namespace.

### Generate a sidecar file: `//mgo:gen`

```go
//mgo:gen stringer
type Status string
```

A directive in a declaration's doc comment targets that declaration. This attached form is called an
_anchored directive_. Every token after the template name is an argument:

```go
//mgo:gen std.stringer trimprefix=Status
type Status int
```

Here, `Status` is the target and `trimprefix=Status` is an argument. A directive in the package doc
comment has no symbol target:

```go
//mgo:gen runtime
package jsonruntime
```

Metago writes package-level generated code to `meta.go`. In test files it writes `meta_test.go` for
internal tests or `meta_<package>_test.go` for external tests. All `//mgo:gen` annotations in the
same compilation package share one sidecar. Package-level `//mgo:inline` is not supported.

### Generate inline: `//mgo:inline`

```go
//mgo:inline stringer
type Status string
```

After running Metago:

```go
//mgo:inline stringer
type Status string

func (s Status) String() string { return string(s) }

//mgo:end
```

An anchored `//mgo:inline` inserts its output after the annotated symbol. Metago inserts `//mgo:end`
automatically; on later runs it replaces only the block between the symbol and `//mgo:end` — the
symbol itself is never touched. Inline templates may use `imports`; Metago adds missing imports to
the same source file.

### Anchored vs standalone

An anchored directive sits in a declaration's doc comment with no blank line before the declaration.
Metago infers the target, and every token after the template name is an argument:

```go
//mgo:gen get /posts/{postID} auth=required
func (p PostRoutes) Show(w http.ResponseWriter, r *http.Request) { ... }
```

This invokes `get` for `PostRoutes.Show` with the positional argument `/posts/{postID}` and the
named argument `auth=required`.

A directive outside a declaration's doc comment is standalone. To name its target explicitly, put
the target after the template name:

```go
type Status string

//mgo:inline stringer Status

func (s Status) String() string { return string(s) }

//mgo:end
```

A declaration containing multiple names also requires an explicit target:

```go
//mgo:gen describe Min
const Min, Max = 1, 10
```

A directive attached to one spec in a declaration block targets that value:

```go
const (
	//mgo:gen describe
	DefaultTimeout = 30

	MaxAttempts = 3
)
```

Inline output for a spec in a declaration block is inserted after the complete block.

### Stacking directives

Anchored directives compose. Several `//mgo:gen`/`//mgo:inline` lines may stack on one symbol;
inline outputs land one after another, in directive order, sharing a single region and a single
`//mgo:end`. Property lines must come after the gen/inline directives in a stack — properties before
a gen/inline directive in the same comment block are an error.

```go
//mgo:inline signals
//mgo:gen validator
//mgo:api owner=core
type AuthSignals struct {
	Email string `json:"sig_email"`
}
```

### Attach metadata: property namespaces

Every `//mgo:<namespace>` comment other than an implemented or reserved directive attaches metadata
to a type, struct field, method, function, interface method, package-level const, or package-level
var. Properties generate nothing themselves. Use them for data only generation cares about; keep
struct tags for what the runtime reads (like `json:`). Package properties are not supported.

```go
//mgo:api owner=core
type User struct {
	//mgo:validate required max=100
	Name string `json:"name"`

	Age int `json:"age"` //mgo:validate min=0 max=150
}
```

The namespace (`api`, `validate`) follows `//mgo:`. Subsequent bare words are flags and `key=value`
pairs are args. Properties must be syntactically attached through a declaration's documentation or a
field's trailing comment; metago does not attach a separated property to the nearest declaration. An
unattached property reports `property "<namespace>" has no symbol to attach to`. Repeating a
namespace on one symbol merges it: flags union, later keys win.

The following names cannot be used as property namespaces: `build`, `config`, `file`, `format`,
`generate`, `import`, `include`, `option`, `options`, `output`, `package`, `plugin`, `profile`, and
`use`. The following named arguments are reserved on `gen` and `inline`: `build`, `dir`, `file`,
`format`, `group`, `mode`, `order`, `output`, `package`, `scope`, `tags`, and the `mgo` namespace
(`mgo`, `mgo.*`, `mgo_*`, `mgo-*`). Property arguments and positional generation arguments remain
unrestricted.

Templates read props with the `prop`, `props`, `propHas`, and `propExists` helpers:

```gotemplate
{{ define "validator" }}
func (v {{ name . }}) Validate() []string {
	var errs []string
{{- range .Fields }}
{{- if propHas . "validate" "required" }}
	if v.{{ name . }} == {{ zero . }} {
		errs = append(errs, "{{ tagName . "json" }} is required")
	}
{{- end }}
{{- end }}
	return errs
}
{{ end }}
```

## Annotation syntax

```text
package:    //mgo:gen templateName positional key=value      (in the package doc comment)
anchored:   //mgo:gen templateName positional key=value      (in a symbol's doc comment)
standalone: //mgo:gen templateName TargetName positional key=value
```

Package-anchored directives have no symbol target and only support `//mgo:gen`. Other anchored
directives target the symbol they document. Every remaining token is an argument.

In the standalone form `TargetName` is optional — if omitted, Metago uses the nearest type,
function, const, or var. A target can be a local type (`User`), top-level function (`BuildUser`),
package-level value (`DefaultTimeout`), local type method (`Server.Serve`), local package target
(`server.Server`, `server.DefaultTimeout`, `server.Server.Serve`), or full import-path target
(`net/http.Client`, `net/http.MethodGet`, `net/http.Client.Do`). A first token that starts with `/`
or contains `{` is always a positional arg, never a target.

In both forms, `key=value` parts are available in `.Args`; other parts are positional args available
in `.Argv` and with `arg`.

## Aggregating annotations: `.Package.Metas`

Every template can read all generation annotations in the package via `.Package.Metas`, sorted by
file then line. Each entry has `.Template`, `.Target`, `.Args`, `.Argv`, `.File`, `.Line`,
`.Inline`, `.Anchored`, and `.PackageScoped`. This lets one annotation generate a single artifact
from many others — route tables, registries, spec files:

```go
//mgo:gen get /posts/{postID}
func ShowPost() { ... }

//mgo:gen post /posts
func CreatePost() { ... }

//mgo:gen server
type Server struct{}
```

```gotemplate
{{ define "get" }}{{ end }}
{{ define "post" }}{{ end }}
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

The empty `get` and `post` templates produce no code. Their directives still appear in
`.Package.Metas`, where `server` collects them into one route list.

`.Package.Metas` contains generation directives only. Read property annotations from their attached
symbols with `prop`, `props`, `propHas`, or `propExists`.

## Template example

```gotemplate
{{ define "stringer" }}
func ({{ receiver . }} {{ name . }}) String() string {
    return string({{ receiver . }})
}
{{ end }}
```

## Template data

Each template receives an invocation object:

| Field                                 | Meaning                                                                            |
| ------------------------------------- | ---------------------------------------------------------------------------------- |
| `.Package`                            | Package metadata.                                                                  |
| `.Meta`                               | The current generation annotation after configured argument defaults are applied.  |
| `.Type`                               | Target type metadata for type and method targets; otherwise `nil`.                 |
| `.Method`                             | Target method metadata for a method target; otherwise `nil`.                       |
| `.Function`                           | Target function metadata for a function target; otherwise `nil`.                   |
| `.Value`                              | Target package-level const/var metadata for a value target; otherwise `nil`.       |
| `.Name`                               | Target symbol or method name; empty for package-scoped invocations.                |
| `.Kind`                               | `package`, `struct`, `interface`, `type`, `method`, `function`, `const`, or `var`. |
| `.TypeName`                           | Target type, enclosing receiver type, or explicitly declared value type.           |
| `.Args`                               | Named `key=value` arguments, including project defaults.                           |
| `.Argv`                               | Positional annotation arguments.                                                   |
| `.Fields`                             | Target struct fields; also populated for method targets from the receiver type.    |
| `.Methods`                            | Concrete or interface methods on a type/method target.                             |
| `.Functions`                          | All top-level functions in the package.                                            |
| `.Params`, `.Results`                 | Parameters and results for a function/method target.                               |
| `.Body`                               | Function/method source text inside the braces only.                                |
| `.Expr`                               | Const/var initializer source text.                                                 |
| `.Values`                             | Typed constants discovered for a type/method target.                               |
| `.IsPackage`                          | Whether the invocation is package-scoped.                                          |
| `.IsType`, `.IsMethod`, `.IsFunction` | Target-kind booleans.                                                              |
| `.IsValue`, `.IsConst`, `.IsVar`      | Package-value target booleans.                                                     |

The nested metadata is also public template data.

### Package

| Field         | Meaning                                                                                     |
| ------------- | ------------------------------------------------------------------------------------------- |
| `.Name`       | Go package name.                                                                            |
| `.Dir`        | Package directory for packages beneath the scan root.                                       |
| `.ImportPath` | Module import path when Metago can derive or load one.                                      |
| `.Types`      | All declared package types.                                                                 |
| `.Functions`  | All top-level package functions.                                                            |
| `.Values`     | All package-level const and var symbols.                                                    |
| `.Metas`      | Generation annotations in deterministic file/line order. Property annotations are excluded. |

### Annotation (`.Meta` and `.Package.Metas` entries)

| Field            | Meaning                                                                                                                                         |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `.Template`      | Selected template name.                                                                                                                         |
| `.Target`        | Resolved source target text; empty for package-scoped directives.                                                                               |
| `.Args`, `.Argv` | Named and positional arguments. `.Meta.Args` includes project defaults; entries reached through `.Package.Metas` contain source arguments only. |
| `.File`, `.Line` | Directive source file and 1-based line.                                                                                                         |
| `.Inline`        | Whether the directive uses `//mgo:inline`.                                                                                                      |
| `.Anchored`      | Whether it is syntactically attached to a declaration.                                                                                          |
| `.PackageScoped` | Whether it is attached to the package declaration.                                                                                              |
| `.EndLine`       | Existing `//mgo:end` line, or zero when no region is bound.                                                                                     |
| `.AnchorEnd`     | Anchored declaration end line; zero for standalone directives.                                                                                  |
| `.AnchorLine`    | Internal attachment bookkeeping; generation annotations currently expose zero.                                                                  |

### Type

| Field                            | Meaning                                                               |
| -------------------------------- | --------------------------------------------------------------------- |
| `.Name`                          | Declared type name.                                                   |
| `.Kind`                          | `struct`, `interface`, or `type` for another defined underlying type. |
| `.Underlying`                    | Underlying source-level type expression.                              |
| `.Fields`, `.Methods`, `.Values` | Fields, declared methods, and discovered typed constants.             |
| `.Props`                         | Property namespaces attached to the type.                             |
| `.File`, `.Line`                 | Declaration source file and 1-based line.                             |

### Field

| Field            | Meaning                                                                             |
| ---------------- | ----------------------------------------------------------------------------------- |
| `.Name`, `.Type` | Field name and declared source-level type expression.                               |
| `.Underlying`    | Resolved underlying type for local named types; empty when unchanged or unresolved. |
| `.TypeKind`      | Resolved local kind such as `struct`; empty when not resolved to a local type.      |
| `.Fields`        | Nested fields when the field resolves to a local named struct.                      |
| `.Tag`           | Complete unquoted struct tag.                                                       |
| `.Embedded`      | Whether the field is embedded.                                                      |
| `.Props`         | Property namespaces attached to the field.                                          |
| `.Line`          | 1-based declaration line.                                                           |

### Method, function, parameter, and value

| Object           | Fields and behavior                                                                                                                                   |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| Method           | `.Name`, `.Receiver`, `.ReceiverType`, `.Params`, `.Results`, `.Body`, `.Props`, `.File`, `.Line`. Interface methods have empty receiver/body fields. |
| Function         | `.Name`, `.Params`, `.Results`, `.Body`, `.Props`, `.File`, `.Line`.                                                                                  |
| Parameter/result | `.Name`, `.Type`, `.Variadic`. Unnamed parameters/results have an empty name.                                                                         |
| Const/var value  | `.Name`, `.Type`, `.Value`, `.Expr`, `.Kind`, `.Props`, `.File`, `.Line`.                                                                             |
| Property group   | `.Group`, `.Args`, `.Argv`.                                                                                                                           |

For values, `.Value` and `.Expr` both contain the initializer's source expression, not its evaluated
value. `.Type` contains explicit source-level type text and is empty for inferred or untyped values.
Metago discovers explicitly typed const specs and later specs that inherit the type in the same
const block for a type target's `.Values`. It does not infer typed constants written only as a
conversion, such as `const Answer = Code(42)`.

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

| Helper                              | Does                                                    | Use when                        |
| ----------------------------------- | ------------------------------------------------------- | ------------------------------- |
| `imports "strconv"`                 | Adds an import to generated output; emits empty string. | Generated code needs imports.   |
| `imports "encoding/json" "stdjson"` | Adds an aliased import.                                 | Avoiding import name conflicts. |

### Deduplicated fragments

Use `emitOnce` to emit a shared declaration once per generated output:

```gotemplate
{{ if emitOnce "example.decodeField" }}
func decodeField(...) { ... }
{{ end }}
```

Namespace keys by template. An empty key is an execution error.

### Template diagnostics

Use `fail` to reject an unsupported invocation:

```gotemplate
{{ if not (isInt .) }}
    {{ fail "requires an integer-backed target" }}
{{ end }}
```

Metago reports all template failures and changes no generated files if any invocation fails.

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

### Props

These accept a field, type, method, function, package-level value, or invocation. All are safe on
symbols without props.

| Helper                            | Does                                       | Use when                        |
| --------------------------------- | ------------------------------------------ | ------------------------------- |
| `prop . "validate" "max"`         | A key=value from a props group, or `""`.   | Reading generation metadata.    |
| `props . "validate"`              | The whole group, with `.Args` and `.Argv`. | Ranging over a group's data.    |
| `propHas . "validate" "required"` | Checks if a group contains a bare flag.    | Boolean markers.                |
| `propExists . "pii"`              | Checks if the group exists at all.         | Distinguishing absent vs empty. |

Example:

```go
//mgo:validate required max=100
Name string `json:"name"`
```

```gotemplate
{{ prop . "validate" "max" }}          {{/* 100 */}}
{{ propHas . "validate" "required" }}  {{/* true */}}
{{ propExists . "db" }}                {{/* false */}}
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
| `camel .Name`      | `user_id` → `userID`.          | JS-facing names.           |
| `pascal .Name`     | `user_id` → `UserID`.          | Exported Go identifiers.   |
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

| Helper          | Does                                                                          | Use when                       |
| --------------- | ----------------------------------------------------------------------------- | ------------------------------ |
| `isString .`    | Resolved type is `string`.                                                    | String-specific code.          |
| `isBool .`      | Resolved type is `bool`.                                                      | Boolean code.                  |
| `isInt .`       | Resolved type is any signed or unsigned integer, including `uintptr`.         | Any integer-specific code.     |
| `isUint .`      | Resolved type is `uint`, `uint8`, `uint16`, `uint32`, `uint64`, or `uintptr`. | Unsigned formatting or bounds. |
| `isFloat .`     | Resolved type is `float32` or `float64`.                                      | Floating-point code.           |
| `isComplex .`   | Resolved type is `complex64` or `complex128`.                                 | Complex-number code.           |
| `isPrimitive .` | String, bool, integer, float, or complex.                                     | Primitive-only templates.      |
| `isSlice .`     | Resolved type starts with `[]`.                                               | Collections.                   |
| `isMap .`       | Resolved type starts with `map[`.                                             | Maps.                          |
| `isPointer .`   | Resolved type starts with `*`.                                                | Nil checks/dereferencing.      |
| `elem .`        | Element of a resolved `[]T` or `*T`; otherwise `""`.                          | Collection/pointer code.       |
| `zero .`        | Go zero-value expression for metadata or a raw type string.                   | Defaults and initializers.     |

Type predicates resolve local named types. For example, `type Count uint64` satisfies both `isInt`
and `isUint`; a field declared as `Count` still resolves to `uint64`. `typeof`, by contrast, returns
the target type's underlying expression but preserves a field, parameter, or value's declared type
text.

### Data

| Helper                   | Does                                       | Use when                          |
| ------------------------ | ------------------------------------------ | --------------------------------- |
| `dict "k" v`             | Creates a map.                             | Passing data to nested templates. |
| `list "a" "b"`           | Creates a list.                            | Inline enumerations.              |
| `get m "key"`            | Reads a map key or exported struct field.  | Optional/dynamic lookups.         |
| `arg 0` / `arg "key"`    | Reads positional or named annotation args. | Annotation args.                  |
| `default fallback value` | Returns fallback if value is zero.         | Optional args.                    |

Examples:

```gotemplate
{{ default "users" (arg "table") }}
{{ arg "table" | default "users" }}
{{ arg 0 }}
```

## Standard templates

Standard templates are embedded in the `metago` binary, work without local template files, and use
the reserved `std.` namespace. User templates cannot define names beginning with `std.`.

### `std.stringer`

```go
//mgo:gen std.stringer trimprefix=Status
type Status int
```

Generates `String() string` for a primitive-backed defined type: string, bool, signed/unsigned
integer, float, or complex. Declared typed constants become switch cases, with unknown values shown
as `Type(value)` and strings quoted. A type without constants returns its underlying value's
standard text representation directly. `trimprefix=value` removes that prefix from constant names in
the returned strings. It defaults to no trimming.

### `std.enum`

```go
//mgo:gen std.enum
type Status int
```

Supports string-, signed integer-, unsigned integer-, and float-backed types with at least one
discovered typed constant. It generates:

```go
func (v Status) String() string
func ParseStatus(value string) (Status, error)
func (v Status) Valid() bool
func StatusValues() []Status
func (v Status) MarshalJSON() ([]byte, error)
func (v *Status) UnmarshalJSON(data []byte) error
```

Integer and float enums strip the type name from constant names by default; override the prefix with
`trimprefix=value`. String enums use each constant's string value. JSON uses the same string form
and rejects unknown values.

### `std.mock`

```go
//mgo:gen std.mock
type Store interface {
    Get(id string) (User, error)
    Save(user User) error
}
```

Generates `MockStore`, one `MethodFunc` field per discovered method, and forwarding methods that
satisfy the interface. Assign function fields directly in tests. Interface method parameters should
be named; embedded interface methods are not expanded, and variadic forwarding is not currently
special-cased.

### `std.mapstruct`

```go
//mgo:gen std.mapstruct allowmissing
type Config struct {
    Host string `mapstructure:"host,required"`
    Port int    `mapstructure:"port"`
}
```

Generates:

```go
func (v *Config) Decode(input map[string]any) error
func (v *Config) Encode() map[string]any
```

It operates on exported fields, uses `mapstructure` tag names, ignores `mapstructure:"-"`, and
recurses into local named struct fields. Decode uses exact Go type assertions rather than numeric or
string conversion. By default every included field is required. The positional `allowmissing` flag
makes fields optional unless their tag contains `required`. Decode is transactional: it updates the
receiver only after every field succeeds. A nested input must be a `map[string]any`.

### `std.serde.jsonruntime` and `std.serde`

Serde is a reflection-free JSON coder-decoder generated entirely by Metago templates. First generate
the shared runtime:

```go
// Package jsonruntime contains generated JSON support.
//
//mgo:gen std.serde.jsonruntime
package jsonruntime
```

Configure its import path once:

```toml
[templates."std.serde".args]
runtime = "example.com/project/internal/jsonruntime"
```

Then derive codecs:

```go
//mgo:gen std.serde
type User struct {
    ID   int64    `json:"id"`
    Name string   `json:"name"`
    Tags []string `json:"tags"`
}
```

`std.serde` generates `MarshalJSON` and `UnmarshalJSON`. Without a `runtime` argument, generated
codecs expect `std.serde.jsonruntime` in the same package. An explicit directive argument overrides
`metago.toml`.

Named arguments:

| Argument              | Default      | Behavior                                                                                    |
| --------------------- | ------------ | ------------------------------------------------------------------------------------------- |
| `runtime=import/path` | Same package | Import the generated runtime from this path.                                                |
| `strict=true          | false`       | `false`                                                                                     | Reject unknown object fields when true; unknown values are otherwise skipped only after full syntax validation. |
| `maxinput=N`          | Disabled     | Reject input larger than `N` bytes before receiver-state allocation. Zero disables the cap. |
| `maxdepth=N`          | `10000`      | Maximum JSON nesting depth. Zero keeps the default.                                         |

`strict` accepts only `true` or `false`. `maxinput` and `maxdepth` must be unsigned 64-bit decimal
integers; invalid values fail generation.

Generated paths cover built-in and methodless named scalars, pointers, slices, arrays, bytes,
`json.RawMessage`, string-keyed maps, nested generated types, and common combinations of those
shapes. Unsupported fields use `encoding/json` and continue to support `json.Marshaler`,
`json.Unmarshaler`, `encoding.TextMarshaler`, and `encoding.TextUnmarshaler`. Structs containing
anonymous fields use `encoding/json` for the complete struct.

Serde follows `encoding/json` behavior for field names and visibility, `-`, `omitempty`, `omitzero`,
and supported `string` options. Decode failures are transactional: the receiver is unchanged.
Decoded retained strings do not alias the input. Recursive generated pointers and containers detect
cycles while encoding, and errors include field/type/JSON-kind context and offsets.

See [`std/serde`](https://github.com/guillemus/metago/tree/main/std/serde) for implementation notes,
compatibility policy, reliability tests, benchmarks, and reproducible benchmark commands.
