# SQL generation experiment

Type-safe SQL model and query helpers generated entirely by Metago. `//mgo:gen queries` emits one shared generic runtime plus model-specific glue into `meta.go`; no handwritten runtime is required.

## Annotations

```go
//mgo:gen queries

//mgo:props model table=users
type User struct {
	ID     int64  //mgo:props sql pk auto filter sort
	Name   string //mgo:props sql filter sort
	Email  string //mgo:props sql unique filter
	Age    int    //mgo:props sql filter sort
	Active bool   //mgo:props sql filter
	Score  float64
	db     DBTX
}
```

- `//mgo:gen queries` runs once and finds every type with `//mgo:props model`.
- Exported primitive fields and pointers to primitives are columns by default.
- Column names default to `snake(FieldName)`; use `column=...` to override.
- `//mgo:props sql ...` is a trailing field annotation with optional flags:
  - `pk` — primary key
  - `auto` — database-generated key
  - `unique` — generate `FindBy<Field>`
  - `filter` — generate `Where<Field>`
  - `sort` — generate `OrderBy<Field>`
  - `column=name` — override the inferred column name
  - `fk=table.column` — retain foreign-key relationship metadata in the generated model descriptor
- Unsupported/non-column fields such as `db DBTX` are ignored without an exclusion annotation.

Queries are immutable: every filter, sort, limit, and offset returns a new scope. `NewModels(db)` groups all generated query handles for reuse by a server or service:

```go
models := NewModels(db)
users, err := models.Users.WhereAge.Gte(18).All(ctx)
```

Use `models.With(tx)` to create the same collection scoped to a transaction.

The scaling fixture contains ten related models: `User`, `Profile`, `Team`, `Membership`, `Project`, `Post`, `Comment`, `Tag`, `PostTag`, and `Activity`. Foreign keys describe the graph, but joins and association loading are intentionally not generated.

## New records

Use the generated model handle to construct an attached record:

```go
u := Users(db).New()
u.Name = "Ada"
u.Email = "ada@example.com"
err := u.Save(ctx)
```

Calling a record method on an unattached value is a programmer error and panics with constructor guidance.

## Generate / run

```sh
# from the Metago repository root
go run . ./experiments/sql

# from this module
go test ./...
go run .
```
