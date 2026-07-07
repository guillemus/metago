# Mission

Build a Go metaprogramming tool that generates ordinary Go code from colocated annotations, while preserving Go's simplicity and tooling.

The system uses lightweight meta comments placed directly beside the code they extend, for example `//#validate.struct User` or `//#sqlite.crud User table=users`. These comments are intentionally minimal: they identify which metaprogram to run and the arguments to pass.

Metaprograms are written as reusable Go `text/template` templates stored in `*.metago` files. During generation, the tool scans the package, parses all Go types, fields, tags, methods, constants, and meta comments, constructs typed metadata objects, executes the referenced templates, and emits generated Go files alongside the original source as `*_meta.go`.

Templates are given rich compile-time metadata about the Go program, allowing them to generate code based on struct fields, tags, enums, methods, imports, and other language constructs. They may also use a small standard library of helper functions for common metaprogramming tasks such as type inspection, tag parsing, name transformations, import management, and code emission.

The goal is not to replace Go with a new language, but to eliminate repetitive, mechanical code while keeping the generated output explicit, readable, debuggable, and fully compatible with the standard Go toolchain. Every generated artifact is ordinary Go code that can be inspected, type-checked, tested, and edited if necessary.

The philosophy is to provide Rust-like derive ergonomics without modifying the Go compiler: lightweight macro invocation, powerful reusable templates, and generated code that remains completely idiomatic Go.
