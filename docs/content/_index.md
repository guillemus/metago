---
title: Metago
---

## A tiny compiler before your compiler

Metago scans Go packages and `*.metago` templates, resolves directive targets, and writes formatted Go. Generated files stay transparent: inspect them, test them, and compile them with the normal Go toolchain.

```go
//mgo:gen stringer
type Status int
```

```go-html-template
{{ define "stringer" }}
func (v {{ name . }}) String() string { return string(v) }
{{ end }}
```
