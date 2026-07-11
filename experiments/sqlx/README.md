# sqlx + generated raw-SQL metadata experiment

This experiment implements the same representative queries as `../activerecord`, but does not generate repositories or a query DSL. SQL remains handwritten and `sqlx` maps rows and named parameters through `db` struct tags.

```go
type User struct {
    ID     UserID `db:"id"`
    Name   string `db:"name"`
    Email  string `db:"email"`
}
```

Metago reads those tags and generates connection-independent metadata for the three initial tables: `User`, `Profile`, and `Team`.

```go
u := Tables.Users

var users []User
err := db.SelectContext(ctx, &users, `
    SELECT `+u.Columns+`
    FROM `+u.Name+`
    WHERE `+string(u.Col.Age)+` >= ?
    ORDER BY `+string(u.Col.Name)+` ASC
`, u.Col.Age.Val(18))
```

`Column[T].Val` checks SQL arguments against the tagged field's Go type. `Qualified`, `ScanRow`, `ScanRows`, and `ScanDestinations` support complete-model projections and joins.

`InsertColumns`, `InsertValues`, and `UpdateSet` use sqlx named parameters:

```go
_, err := db.NamedExecContext(ctx, `
    INSERT INTO `+u.Name+` (`+u.InsertColumns+`)
    VALUES (`+u.InsertValues+`)
`, &user)
```

The initial convention is intentionally small:

- Any struct containing `db` tags is a model.
- Table names are pluralized snake-case type names.
- `db:"-"` fields are ignored.
- A field named `ID` is treated as the auto-generated primary key.
- SQL, predicates, grouping, ordering, pagination, and persistence remain explicit.

Generate and test from the repository root:

```sh
go run . ./experiments/sqlx
cd experiments/sqlx
go test ./...
```
