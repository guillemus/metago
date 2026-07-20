# MapStruct experiment

> **Experimental:** this generator is not part of Metago's built-in `std.` templates. Its API is
> undecided and may change or be removed without notice.

This is a Go experiment for generating typed `Decode` and `Encode` methods between structs and
`map[string]any`. Run its fixture with:

```sh
go generate
```

## Attribution

This independent experiment is inspired by [MapStruct](https://mapstruct.org/), the Java
compile-time bean-mapping generator created by [Gunnar Morling](https://mapstruct.org/development/team/).
No MapStruct source code is included here.
