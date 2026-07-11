---
title: Getting started
weight: 10
description: Install Metago and generate your first Go code.
---

# Getting started

Metago runs alongside the Go compiler. You describe code generation with source comments and reusable Go templates; Metago emits ordinary Go.

> Metago is early-stage software. APIs and directives may evolve.

## Install

```sh
go install github.com/guillemus/metago@latest
```

Optionally install the Metago skill for supported coding agents:

```sh
npx skills add guillemus/metago --skill metago
```

## Your first generator

Create `stringer.metago` anywhere beneath the directory you plan to scan:

```go-html-template
{{ define "stringer" }}
func (v {{ name . }}) String() string {
    switch v {
    {{- range .Values }}
    case {{ .Name }}:
        return {{ quote .Name }}
    {{- end }}
    default:
        return "unknown"
    }
}
{{ end }}
```

Annotate a Go type:

```go
package example

//mgo:gen stringer
type Status int

const (
    StatusPending Status = iota
    StatusRunning
    StatusDone
)
```

Run Metago from the scan root:

```sh
metago .
```

Metago creates a package-level `meta.go` containing the generated `String` method.

## CLI

```sh
metago              # scan the current directory recursively
metago ./path       # scan another root
metago -v           # show verbose logs
metago --verbose
```

Templates may live anywhere under the scan root. Metago skips hidden directories, `vendor`, and `testdata`. It ignores `_test.go`, `meta.go`, and `*_meta.go` while scanning packages. Template names must be unique across the root.
