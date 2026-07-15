---
name: metago
description:
    'Use this skill whenever the user mentions Metago, .metago templates, //mgo: directives, Go code
    generation from source comments, generated meta.go files, or asks to add/debug/refactor
    metaprogramming templates for Go. This skill explains how to use the metago tool, write
    annotations and templates, use template metadata/helpers like name/typeof/keys/tag/imports, and
    test generation with golden fixtures.'
---

# Metago

Metago is a small Go metaprogramming tool used alongside Go code before compiling. It reads
`//mgo:gen`, `//mgo:inline`, and namespaced property annotations plus reusable `*.metago`
`text/template` files, then writes ordinary Go code.

Use this skill to help users write or debug Metago annotations, templates, generated code, tests, or
the Metago tool itself.

## Source of truth

When working inside a Metago repository, inspect the repository files first, especially `README.md`,
`main.go`, `utils.go`, `main_test.go`, and `utilities_test.go`. Treat those files as authoritative
if they differ from this skill. If Metago is only installed as a dependency or command, use this
skill as the reference and consult the upstream repository at `https://github.com/guillemus/metago`
when current implementation details matter.

## Core workflow

1. Add an anchored annotation directly above the Go type it extends:

```go
//mgo:gen stringer
type Status string
```

This directive targets `Status`. In an anchored directive, every token after `stringer` is an
argument.

2. Define a matching template in a `*.metago` file under the Metago invocation root (outside skipped
   directories):

```gotemplate
{{ define "stringer" }}
func (x {{ name . }}) String() string {
    return string(x)
}
{{ end }}
```

3. Run the installed command against the package or project root:

```sh
metago ./path/to/package
```

Users can install it with `go install github.com/guillemus/metago@latest` or download a ready-to-use
binary from [GitHub Releases](https://github.com/guillemus/metago/releases).

With Go 1.24 or later, a project can pin and invoke Metago through `go generate`:

```sh
go get -tool github.com/guillemus/metago@latest
```

```go
//go:generate go tool metago .
```

Add one directive at the project root. Metago scans that root recursively. `go generate` runs from
the directive's package directory, so adjust `.` when the scan root is elsewhere.

From a checkout of the Metago tool itself, the equivalent development command is:

```sh
go run . ./path/to/package
```

4. The tool writes sidecar generated Go to one package-level file:

```text
meta.go
```

All production `//mgo:gen` directives in the same package share that `meta.go` file. Test directives
write to `meta_test.go` for the internal package or `meta_<package>_test.go` for an external
`<package>_test` package. Generated output should be ordinary formatted Go in the same package as
the annotated source. Successful runs are silent by default; use `-v` or `--verbose` to see colored
debug logs.

Treat generated sidecars and all code between `//mgo:inline` and `//mgo:end` as read-only. Never
edit generated code directly: change the originating directive or `*.metago` template, then rerun
Metago. Direct edits will be overwritten.

Metago recursively skips `vendor`, `testdata`, and hidden directories. Package scanning ignores
generated Metago sidecars and processes `_test.go` files in their separate Go compilation packages.

An optional `metago.toml` at the project root configures default named arguments for templates.
Explicit arguments on `//mgo:gen` and `//mgo:inline` override configured defaults.

## Annotation rules

Use `//mgo:gen` to generate package-level `meta.go` and `//mgo:inline` to inline into the same file
between the directive and an auto-inserted `//mgo:end` block. Every other `//mgo:<namespace>`
annotation attaches properties to a symbol. Metago comments must have no space after `//`:

```go
// Anchored: the target is inferred from the declaration below.
//mgo:gen stringer
type Status string

//mgo:inline validator strict
//mgo:gen sqlite.crud table=users
type User struct{}

// Standalone: the target is explicit.
type Code int

//mgo:gen stringer Code
```

When a declaration has documentation, put it before Metago directives:

```go
// Status is the state of a job.
//mgo:gen stringer
type Status string
```

Do not use this form; it is ignored:

```go
// mgo:gen stringer Status
```

Annotation shapes:

```text
package:    //mgo:gen templateName positional key=value
anchored:   //mgo:gen templateName positional key=value
standalone: //mgo:gen templateName [TargetName] positional key=value
```

- `templateName` selects `{{ define "templateName" }}` from a `.metago` file. The built-in JSON
  codec is `std.serde`; its shared runtime template is `std.serde.jsonruntime`.
- The built-in `std.stringer` supports primitive-backed enums and ordinary value types. Enum
  constants become named cases with `Type(value)` fallback; types without constants return the
  underlying value's standard text directly.
- A directive in the package doc comment is package-scoped: it has no symbol target, exposes
  `.Package`, sets `.Kind` to `package` and `.IsPackage` to true, and only supports `//mgo:gen`.
- An anchored directive in a type, function, method, package-level const, or package-level var doc
  comment targets that symbol. Every token after the template name is an argument.
- A standalone directive may explicitly target a local type (`User`), top-level function
  (`BuildUser`), package-level value (`DefaultTimeout`), local method (`Server.Serve`), local
  package symbol (`server.Server` or `server.DefaultTimeout`), or full import-path symbol
  (`net/http.Client.Do`). Without a target, Metago uses the nearest type, function, const, or var.
  The first bare token is treated as a target unless it starts with `/` or contains `{`, in which
  case it is a positional path argument.
- A const/var declaration with multiple names requires the standalone form with an explicit value
  name. A directive on one spec inside a parenthesized declaration targets that value, and inline
  output is inserted after the complete declaration block.
- `key=value` pairs are available with `{{ arg "key" }}` and in `.Args`.
- Other tokens are positional args available with `{{ arg 0 }}`, `{{ arg 1 }}`, and in `.Argv`.

## Template data available

Templates receive an invocation object. Common fields:

```gotemplate
{{ .Name }}       {{/* target name */}}
{{ .Kind }}       {{/* package, struct, interface, type, method, function, const, or var */}}
{{ .TypeName }}   {{/* enclosing or declared type name */}}
{{ .Type }}       {{/* target type, for type/method targets */}}
{{ .Method }}     {{/* target method, for Type.Method targets */}}
{{ .Function }}   {{/* target function, for function targets */}}
{{ .Value }}      {{/* target value metadata, for const/var targets */}}
{{ .Expr }}       {{/* const/var initializer source text */}}
{{ .Meta }}       {{/* current annotation metadata */}}
{{ .Args }}       {{/* annotation key=value map */}}
{{ .Argv }}       {{/* positional annotation args */}}
{{ .Fields }}     {{/* struct fields */}}
{{ .Methods }}    {{/* concrete or interface methods on the target type, including params/results/body */}}
{{ .Functions }}  {{/* top-level package functions, including params/results/body */}}
{{ .Params }}     {{/* target function/method params */}}
{{ .Results }}    {{/* target function/method results */}}
{{ .Body }}       {{/* target function/method source text inside braces only */}}
{{ .IsPackage }} {{ .IsType }} {{ .IsMethod }} {{ .IsFunction }}
{{ .IsValue }} {{ .IsConst }} {{ .IsVar }}
{{ .Values }}     {{/* discovered constants of the target type */}}
{{ .Package.Name }}
{{ .Package.Metas }}  {{/* all generation annotations in file/line order */}}
{{ .Package.Values }} {{/* all package-level const and var symbols */}}
```

Value objects expose `.Name`, `.Type`, `.Value`, `.Expr`, `.Kind`, and `.Props`. `.Value` and
`.Expr` are initializer source text, not an evaluated value; `.Kind` is `const` or `var`. `.Type` is
explicit source-level type text and is empty for inferred or untyped declarations. Discovery for a
type target's `.Values` covers explicitly typed const specs and inherited specs in the same block,
but not conversion-only declarations such as `const Answer = Code(42)`.

Field objects include:

```gotemplate
{{ .Name }}
{{ .Type }}
{{ .Tag }}
{{ .Embedded }}
{{ .Props }}
```

Method objects include; interface methods have empty `.Receiver`, `.ReceiverType`, and `.Body`:

```gotemplate
{{ .Name }}
{{ .Receiver }}
{{ .ReceiverType }}
{{ .Body }} {{/* source text inside the method braces only */}}
{{ range .Params }}{{ .Name }} {{ .Type }} variadic={{ .Variadic }}{{ end }}
{{ range .Results }}{{ .Name }} {{ .Type }}{{ end }}
```

Top-level function objects are available as `.Functions` and include `.Name`, `.Params`, `.Results`,
`.Body`, and `.Props`. Method objects also expose `.Props`. Property annotations do not appear in
`.Package.Metas`; they attach directly to their target symbols.

Example:

```gotemplate
{{ define "fields" }}
func (x {{ name . }}) FieldNames() []string {
    return []string{ {{ range .Fields }}{{ printf "%q" .Name }}, {{ end }} }
}
{{ end }}
```

## Helper functions

Use these helpers in templates:

```gotemplate
{{ name . }}              {{/* target/field/method/value name */}}
{{ typeof . }}            {{/* underlying type, field type, or value type */}}
{{ keys . }}              {{/* field names, or sorted string map keys */}}
{{ fieldNames . }} {{ methodNames . }}

{{ tag . "json" }} {{ tagName . "json" }} {{ tagOpts . "json" }}
{{ tagExists . "json" }} {{ tagHas . "json" "omitempty" }}
{{ prop . "validate" "max" }} {{ props . "validate" }}
{{ propHas . "validate" "required" }} {{ propExists . "validate" }}

{{ fieldsWithTag . "json" }} {{ fieldsWithoutTag . "json" }}
{{ exportedFields . }} {{ unexportedFields . }}
{{ embeddedFields . }} {{ nonEmbeddedFields . }}

{{ snake .Name }} {{ camel .Name }} {{ pascal .Name }} {{ kebab .Name }}
{{ initial .Name }} {{ receiver . }} {{ exported .Name }} {{ unexported .Name }}
{{ lower "Name" }} {{ upper "name" }}
{{ contains "UserID" "ID" }} {{ hasPrefix "UserID" "User" }} {{ hasSuffix "UserID" "ID" }}
{{ trimPrefix "UserID" "User" }} {{ trimSuffix "UserID" "ID" }} {{ replace "UserID" "ID" "Id" }}
{{ split "a,b" "," }} {{ join (keys .) "," }} {{ quote "literal" }}

{{ isString . }} {{ isInt . }} {{ isBool . }} {{ isFloat . }}
{{ isSlice . }} {{ isMap . }} {{ isPointer . }} {{ elem . }} {{ zero . }}
{{ arg 0 }} {{ arg "table" }}
{{ dict "k" "v" }} {{ list "a" "b" }} {{ get .Args "table" }} {{ default "users" (arg "table") }}
{{ imports "strconv" }}   {{/* emits empty string; works in sidecar and inline templates */}}
{{ emitOnce "mytemplate.helper" }} {{/* true once per generated output */}}
{{ fail "unsupported target" }} {{/* rejects this invocation */}}
```

`imports` is intentionally side-effectful and returns an empty string. Put it inside the branch that
needs the import:

```gotemplate
{{ define "stringer" }}
func (x {{ name . }}) String() string {
    {{ if eq (typeof .) "int" -}}
    {{ imports "strconv" -}}
    return strconv.Itoa(int(x))
    {{ else -}}
    return string(x)
    {{ end -}}
}
{{ end }}
```

To import with an alias:

```gotemplate
{{ imports "encoding/json" "stdjson" }}
```

Use `emitOnce` to guard declarations shared by multiple invocations in the same generated output:

```gotemplate
{{ if emitOnce "mytemplate.decodeField" }}
func decodeField[T any](...) error { ... }
{{ end }}
```

Namespace keys by template. Deduplication is per generated output.

## Keep template expressions readable

Do not produce dense one-line actions that combine several helper calls, conditions, formatting
operations, and interpolations. Give semantic intermediate values names near the top of the relevant
scope, then compose generated code from those variables. If a generated Go call has many arguments,
format it across multiple lines as normal readable Go.

Avoid:

```gotemplate
if err := decode({{ $input }}, {{ quote $key }}, {{ quote $path }}, &{{ $access }}, {{ if or (not $allowMissing) (tagHas $field "mapstructure" "required") }}true{{ else }}false{{ end }}); err != nil {
```

Prefer:

```gotemplate
{{- $quotedKey := quote $key -}}
{{- $quotedPath := quote $path -}}
{{- $required := or (not $allowMissing) (tagHas $field "mapstructure" "required") -}}
{{- $destination := printf "&%s" $access }}
if err := decode(
    {{ $input }},
    {{ $quotedKey }},
    {{ $quotedPath }},
    {{ $destination }},
    {{ $required }},
); err != nil {
```

Prefer variables named after their meaning (`$required`, `$destination`, `$quotedPath`) rather than
abbreviations or variables named after implementation mechanics. Read the `*.metago` source after
generation; clean generated Go does not compensate for an unreadable template.

## Preserve template indentation

Format `*.metago` files for humans as carefully as the Go they generate. Indent template control
actions (`if`, `else`, `range`, `with`, and their matching `end`) according to their nesting level
in the template, including when they surround indented Go code. Keep sibling control actions
aligned. Do not flatten every `{{ ... }}` action to column zero merely because whitespace-trimming
markers keep it out of the generated output; that makes nested template logic difficult to follow.

Prefer:

```gotemplate
func (v {{ name . }}) String() string {
    switch v {
    {{- range .Values }}
    case {{ .Name }}:
        return {{ quote .Name }}
    {{- end }}
    default:
        {{- if isString . }}
        return string(v)
        {{- else if isInt . }}
        return strconv.FormatInt(int64(v), 10)
        {{- end }}
    }
}
```

When editing an existing template, preserve its established indentation style and review the
template source itself after generation tests pass. Generated Go formatting does not validate
whether the `*.metago` source remains readable.

## Struct tags

Prefer `tag` over manually parsing `.Tag`:

```gotemplate
{{ range .Fields }}
// {{ .Name }} json={{ tag . "json" }} db={{ tag . "db" }}
{{ end }}
```

For:

```go
ID int `json:"id,omitempty" db:"user_id"`
```

`{{ tag . "json" }}` returns `id,omitempty`; `{{ tagName . "json" }}` returns `id`;
`{{ tagExists . "json" }}` and `{{ tagHas . "json" "omitempty" }}` return `true`.

## Property namespaces

Every `//mgo:<namespace>` annotation other than the reserved `gen`, `inline`, and `end` directives
attaches generation metadata to its documented type, field, method, function, interface method,
package-level const, or package-level var. Bare words after the namespace are flags and `key=value`
tokens are arguments:

```go
//mgo:api owner=core
type User struct {
    Name string //mgo:validate required max=100
}
```

```gotemplate
{{ prop . "validate" "max" }}
{{ propHas . "validate" "required" }}
{{ props . "validate" }}
{{ propExists . "validate" }}
```

Repeated lines for the same namespace merge: flags are unioned and later values replace earlier
values. In a stacked doc comment, generation directives must come before property annotations.

The following names cannot be used as property namespaces: `build`, `config`, `file`, `format`,
`generate`, `import`, `include`, `option`, `options`, `output`, `package`, `plugin`, `profile`, and
`use`. The following named arguments are reserved on `gen` and `inline`: `build`, `dir`, `file`,
`format`, `group`, `mode`, `order`, `output`, `package`, `scope`, `tags`, plus the `mgo` namespace
(`mgo`, `mgo.*`, `mgo_*`, `mgo-*`).

## Golden testing pattern

Metago uses fixture directories under `testdata/`:

```text
testdata/basic/
├── model.go
├── templates.metago
└── meta.go.golden
```

The test should call the generator directly, compare bytes to the golden file, and support updating
goldens with:

```sh
UPDATE_GOLDEN=1 go test ./...
```

After changing template output or helpers, run:

```sh
go test ./...
staticcheck ./...
```

## Debugging checklist

When generation does not happen:

- Check the annotation is exactly `//mgo:gen...` or `//mgo:inline...`, not `// mgo:...`.
- Check the template file ends in `.metago` and is under the invocation root.
- Check the annotation template name matches `{{ define "..." }}`.
- Check the target type exists in the same package.
- Check generated code compiles after formatting.
- If an import is missing, add `{{ imports "pkg/path" }}` in the template branch that emits code
  using it.

When changing Metago internals:

- Preserve silent success: no stdout on successful generation unless `-v`/`--verbose` is set.
- Generated code should be formatted with `go/format`.
- Generated code should use the annotated package name.
- Keep generated artifacts ordinary Go.
- Prefer adding fixture coverage in `testdata/` plus golden output.
