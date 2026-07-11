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

1. Add an anchored annotation directly above the Go type it extends:

```go
//mgo:gen stringer
type Status string
```

Because this directive is anchored to `Status`, tokens after `stringer` are arguments, not a target.

2. Define a matching template in a `*.metago` file under the Metago invocation root (outside skipped directories):

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

From a checkout of the Metago tool itself, the equivalent development command is:

```sh
go run . ./path/to/package
```

4. The tool writes sidecar generated Go to one package-level file:

```text
meta.go
```

All `//mgo:gen` directives in the same package share that `meta.go` file. Generated output should be ordinary formatted Go in the same package as the annotated source. Successful runs are silent by default; use `-v` or `--verbose` to see colored debug logs.

Metago recursively skips `vendor`, `testdata`, and hidden directories. Package scanning also ignores `_test.go`, `meta.go`, and `*_meta.go`, so test-only symbols and directives are not processed.

## Annotation rules

Use `//mgo:gen` to generate package-level `meta.go`, `//mgo:inline` to inline into the same file between the directive and an auto-inserted `//mgo:end` block, and `//mgo:props` to attach metadata to symbols. Metago comments must have no space after `//`:

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

Do not use this form; it is ignored:

```go
// mgo:gen stringer Status
```

Annotation shapes:

```text
anchored:   //mgo:gen templateName positional key=value
standalone: //mgo:gen templateName [TargetName] positional key=value
```

- `templateName` selects `{{ define "templateName" }}` from a `.metago` file.
- An anchored directive is in a type, function, or method doc comment. Its target is that symbol, and every token after the template name is an argument.
- A standalone directive may explicitly target a local type (`User`), top-level function (`BuildUser`), local method (`Server.Serve`), local package symbol (`server.Server`), or full import-path symbol (`net/http.Client.Do`). Without a target, Metago uses the nearest type or function. The first bare token is treated as a target unless it starts with `/` or contains `{`, in which case it is a positional path argument.
- `key=value` pairs are available with `{{ arg "key" }}` and in `.Args`.
- Other tokens are positional args available with `{{ arg 0 }}`, `{{ arg 1 }}`, and in `.Argv`.

## Template data available

Templates receive an invocation object. Common fields:

```gotemplate
{{ .Name }}       {{/* target name */}}
{{ .Kind }}       {{/* struct, interface, type, method, or function */}}
{{ .TypeName }}   {{/* enclosing type name for type/method targets */}}
{{ .Type }}       {{/* target type, for type/method targets */}}
{{ .Method }}     {{/* target method, for Type.Method targets */}}
{{ .Function }}   {{/* target function, for function targets */}}
{{ .Meta }}       {{/* current annotation metadata */}}
{{ .Args }}       {{/* annotation key=value map */}}
{{ .Argv }}       {{/* positional annotation args */}}
{{ .Fields }}     {{/* struct fields */}}
{{ .Methods }}    {{/* concrete or interface methods on the target type, including params/results/body */}}
{{ .Functions }}  {{/* top-level package functions, including params/results/body */}}
{{ .Params }}     {{/* target function/method params */}}
{{ .Results }}    {{/* target function/method results */}}
{{ .Body }}       {{/* target function/method source text inside braces only */}}
{{ .IsType }} {{ .IsMethod }} {{ .IsFunction }}
{{ .Values }}     {{/* discovered constants of the target type */}}
{{ .Package.Name }}
{{ .Package.Metas }} {{/* all generation annotations in file/line order */}}
```

Value objects expose `.Name`, `.Type`, and `.Value`; `.Value` is source text, not an evaluated numeric value. Discovery covers explicitly typed const specs and inherited specs in the same block, but not conversion-only declarations such as `const Answer = Code(42)`.

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

Top-level function objects are available as `.Functions` and include `.Name`, `.Params`, `.Results`, `.Body`, and `.Props`. Method objects also expose `.Props`. `//mgo:props` annotations do not appear in `.Package.Metas`; they attach directly to their target symbols.

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

## Props

`//mgo:props` attaches grouped generation metadata to the nearest type, field, method, function, or interface method. The group is mandatory; bare words are flags and `key=value` tokens are arguments:

```go
//mgo:props api owner=core
type User struct {
    Name string //mgo:props validate required max=100
}
```

```gotemplate
{{ prop . "validate" "max" }}
{{ propHas . "validate" "required" }}
{{ props . "validate" }}
{{ propExists . "validate" }}
```

Repeated lines for the same group merge: flags are unioned and later values replace earlier values. In a stacked doc comment, generation directives must come before props directives.

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
