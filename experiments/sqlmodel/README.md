# SQL Model

SQL Model uses Metago to generate typed SQL repositories and query APIs from plain Go structs. It
remains under `x` until its API and semantics are stable.

## Layout

- `templates.metago` generates model metadata and typed query APIs.
- `runtime.metago` supplies the generated SQL/query runtime.
- `testmodels/models.go` declares dedicated fixture models.
- `testmodels/*_test.go` verifies generated behavior, including joins.
- `testmodels/meta.go` is generated output.

The models under `testmodels` exist only to exercise the template surface. The separate models under
`experiments/activerecord/models` demonstrate consumption from an application and are not used as
generator fixtures.

## Generate and test

Generate from the repository root so the shared templates are visible to both independent model
packages. Template names are globally unique within that scan; a duplicate name is a compile error.

```sh
go run . .
(cd experiments/sqlmodel && go test ./...)
(cd experiments/activerecord && go test ./...)
```
