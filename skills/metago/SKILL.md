---
name: metago
description: "Use this skill whenever the user mentions Metago, .metago templates, //mgo: directives, Go code generation from source comments, generated meta.go files, or asks to add/debug/refactor metaprogramming templates for Go. This skill explains how to use the metago tool, write annotations and templates, use template metadata/helpers like name/typeof/keys/tag/imports, and test generation with golden fixtures."
---

# Metago

Metago is a small Go metaprogramming tool used alongside Go code before compiling. It reads `//mgo:gen`, `//mgo:inline`, and `//mgo:props` directives plus reusable `*.metago` `text/template` files, then writes ordinary Go code.

Use this skill to help users write or debug Metago annotations, templates, generated code, tests, or the Metago tool itself.

## Source of truth

When working inside a Metago repository, inspect the repository files first, especially `README.md`, `main.go`, `utils.go`, `main_test.go`, and `utilities_test.go`. Treat those files as authoritative if they differ from this skill. If Metago is only installed as a dependency or command, use this skill as the reference and consult the upstream repository at `https://github.com/guillemus/metago` when current implementation details matter.

## Core workflow

1. Add an annotation directly beside the Go type it extends:

```go
//mgo:gen stringer Status
type Status string
```

2. Define a matching template in any `*.metago` file under the Metago invocation root:

```gotemplate
{{ define "stringer" }}
func (x {{ name . }}) String() string {
    return string(x)
}
{{ end }}
```

3. Run Metago in that package directory:

```sh
go run . ./path/to/package
```

or, from inside the package/tool repo when appropriate:

```sh
go run .
```

4. The tool writes sidecar generated Go to one package-level file:

```text
meta.go
```

All `//mgo:gen` directives in the same package share that `meta.go` file. Generated output should be ordinary formatted Go in the same package as the annotated source. Successful runs are silent by default; use `-v` or `--verbose` to see colored debug logs.

## Annotation rules

Use `//mgo:gen` to generate package-level `meta.go`, `//mgo:inline` to inline into the same file between the directive and an auto-inserted `//mgo:end` block, and `//mgo:props` to attach metadata to symbols. Metago comments must have no space after `//`:

```go
//mgo:gen stringer Status
//mgo:inline stringer Status
//mgo:gen sqlite.crud User table=users
```

Do not use this form; it is ignored:

```go
// mgo:gen stringer Status
```

Annotation shape:

```text
//mgo:gen templateName TargetName positional key=value another=value
//mgo:inline templateName TargetName positional key=value another=value
```

- `templateName` selects `{{ define "templateName" }}` from a `.metago` file.
- `TargetName` is not counted as a positional arg. It can be a local type (`User`), top-level function (`BuildUser`), local type method (`Server.Serve`), local package target (`server.Server`, `server.Server.Serve`), or full import-path target (`net/http.Client`, `net/http.Client.Do`). If omitted, Metago uses the nearest type or function.
- `key=value` pairs are available with `{{ arg "key" }}` and in `.Args`.
- Non-`key=value` extras are positional args available with `{{ arg 0 }}`, `{{ arg 1 }}`, and in `.Argv`.

## Template data available

Templates receive an invocation object. Common fields:

```gotemplate
{{ .Name }}       {{/* target name */}}
{{ .Kind }}       {{/* struct, interface, type, method, or function */}}
{{ .TypeName }}   {{/* enclosing type name for type/method targets */}}
{{ .Type }}       {{/* target type, for type/method targets */}}
{{ .Method }}     {{/* target method, for Type.Method targets */}}
{{ .Function }}   {{/* target function, for function targets */}}
{{ .Args }}       {{/* annotation key=value map */}}
{{ .Argv }}       {{/* positional annotation args */}}
{{ .Fields }}     {{/* struct fields */}}
{{ .Methods }}    {{/* concrete or interface methods on the target type, including params/results/body */}}
{{ .Functions }}  {{/* top-level package functions, including params/results/body */}}
{{ .Params }}     {{/* target function/method params */}}
{{ .Results }}    {{/* target function/method results */}}
{{ .Body }}       {{/* target function/method source text inside braces only */}}
{{ .IsType }} {{ .IsMethod }} {{ .IsFunction }}
{{ .Values }}     {{/* constants of the target type */}}
{{ .Package.Name }}
```

Field objects include:

```gotemplate
{{ .Name }}
{{ .Type }}
{{ .Tag }}
{{ .Embedded }}
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

Top-level function objects are available as `.Functions` and include `.Name`, `.Params`, `.Results`, and `.Body`.

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
{{ keys . }}              {{/* field names for a type/invocation, sorted map keys for maps */}}
{{ fieldNames . }}        {{/* field names */}}
{{ methodNames . }}       {{/* comma-joined method names */}}
{{ tag . "json" }}        {{/* full struct tag value */}}
{{ tagName . "json" }}    {{/* first comma-separated tag part */}}
{{ tagOpts . "json" }}    {{/* tag options after the first comma */}}
{{ tagExists . "json" }}  {{/* true if the tag key exists */}}
{{ tagHas . "json" "omitempty" }} {{/* true if the tag has an option */}}
{{ fieldsWithTag . "json" }}
{{ fieldsWithoutTag . "json" }}
{{ exportedFields . }}
{{ embeddedFields . }}
{{ snake .Name }} {{ camel .Name }} {{ pascal .Name }} {{ kebab .Name }}
{{ receiver . }}
{{ isString . }} {{ isInt . }} {{ isSlice . }} {{ elem . }} {{ zero . }}
{{ arg 0 }} {{ arg "table" }} {{/* positional or named annotation args */}}
{{ dict "k" "v" }} {{ list "a" "b" }} {{ get .Args "table" }} {{ default "users" (arg "table") }}
{{ imports "strconv" }}   {{/* emits empty string; works in sidecar and inline templates */}}
{{ lower "Name" }} {{ upper "name" }} {{ quote "literal" }} {{ join (keys .) "," }}
```

`imports` is intentionally side-effectful and returns an empty string. Put it inside the branch that needs the import:

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

`{{ tag . "json" }}` returns `id,omitempty`; `{{ tagName . "json" }}` returns `id`; `{{ tagExists . "json" }}` and `{{ tagHas . "json" "omitempty" }}` return `true`.

## Golden testing pattern

Metago uses fixture directories under `testdata/`:

```text
testdata/basic/
├── model.go
├── templates.metago
└── meta.go.golden
```

The test should call the generator directly, compare bytes to the golden file, and support updating goldens with:

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
- If an import is missing, add `{{ imports "pkg/path" }}` in the template branch that emits code using it.

When changing Metago internals:

- Preserve silent success: no stdout on successful generation unless `-v`/`--verbose` is set.
- Generated code should be formatted with `go/format`.
- Generated code should use the annotated package name.
- Keep generated artifacts ordinary Go.
- Prefer adding fixture coverage in `testdata/` plus golden output.
