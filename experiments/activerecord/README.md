# Generated SQL repository experiment

Metago generates typed, immutable query scopes and repository operations while model structs remain plain Go data. Models and generated code live in the `models` subpackage, so application autocomplete only exposes its public API.

## Models

```go
//mgo:gen queries

type AgentStatus string

//mgo:model table=users
type User struct {
    ID        int64       //mgo:sql pk auto filter sort
    Email     string      //mgo:sql unique filter
    Status    AgentStatus //mgo:sql filter
    CreatedAt time.Time   //mgo:sql filter sort
}
```

Every exported field is persisted automatically. Fields may use scalars, named types, `time.Time`, `sql.Null*`, `[]byte`, and arbitrary `sql.Scanner`/`driver.Valuer` types. Use `//mgo:sql` only to configure capabilities such as `pk`, `auto`, `filter`, `sort`, or `unique`, or to override the column name.

## Plain records and persistence

Models contain no hidden database connection. All persistence goes through a database-scoped repository:

```go
Models := models.NewModels(db)
Users := Models.Users
user := models.User{Name: "Ada", Email: "ada@example.com"}

err := Users.Insert(ctx, &user) // assigns an automatic ID
user.Name = "Augusta"
err = Users.Update(ctx, &user)
err = Users.Reload(ctx, &user)
err = Users.DeleteRecord(ctx, &user)
```

`Create` is also available when a returned pointer is convenient:

```go
user, err := Users.Create(ctx, models.User{Name: "Ada", Email: "ada@example.com"})
```

Query deletion remains separate:

```go
count, err := Users.WhereActive.Eq(false).Delete(ctx)
```

## Typed queries

```go
list, err := Users.
    WhereAge.Gte(18).
    WhereActive.Eq(true).
    OrderByName.Asc().
    Limit(20).
    All(ctx)
```

Chained filters use `AND`. Use `Or` to combine predicate groups, then continue chaining normally:

```go
query := Users.
    WhereName.Eq("Ada").
    Or(Users.WhereActive.Eq(false)).
    WhereAge.Gte(18)
// (name = ? OR active = ?) AND age >= ?
```

`And` is also available for explicit composition. Queries are immutable. Create `Models` once per database scope and reuse its namespaced handles. `Models.With(tx)` creates the same namespace for a transaction.

## Static schema metadata and raw joins

`Tables` contains connection-independent, collision-safe schema metadata. Columns are unqualified by default:

```go
models.Tables.Users.Name       // "users"
models.Tables.Users.Col.Email  // "email"
models.Tables.Users.Columns    // "id, name, email, ..."
```

Call `Qualified` only when a join needs table prefixes:

```go
u := models.Tables.Users.Qualified()
p := models.Tables.Profiles.Qualified()

row := db.QueryRowContext(ctx, fmt.Sprint(`
    SELECT `, u.Columns, `, `, p.Columns, `
    FROM `, u.Name, `
    JOIN `, p.Name, ` ON `, p.Col.UserID, ` = `, u.Col.ID, `
    WHERE `, u.Col.Email, ` = ?
`), u.Col.Email.Val(email))

var user User
var profile Profile
dest := append(
    u.ScanDestinations(&user),
    p.ScanDestinations(&profile)...,
)
err := row.Scan(dest...)
```

The table descriptors also provide `InsertColumns`, `InsertPlaceholders`, `UpdateSet`, `InsertArgs`, `UpdateArgs`, `ScanRow`, and `ScanRows` for other hand-written SQL. Generated fragments keep schema names and scan order refactorable; SQL syntax is still checked by the database.

### Complex raw SQL returning one model

`ScanRow` works with any `QueryRowContext` result whose projection contains one complete model in `Columns` order. The query itself can use features outside the typed query API:

```go
u := models.Tables.Users.Qualified()
row := db.QueryRowContext(ctx, fmt.Sprint(`
    SELECT `, u.Columns, `
    FROM `, u.Name, `
    WHERE `, u.Col.Active, ` = ?
      AND `, u.Col.Score, ` = (
        SELECT MAX(candidate.score)
        FROM users AS candidate
        WHERE candidate.active = ?
      )
`), u.Col.Active.Val(true), u.Col.Active.Val(true))

var user User
err := u.ScanRow(row, &user)
```

### Complex raw SQL returning multiple models

`ScanRows` handles complete model rows returned by CTEs, window functions, unions, or other custom SQL:

```go
qualified := models.Tables.Users.Qualified()
base := models.Tables.Users
rows, err := db.QueryContext(ctx, fmt.Sprint(`
    WITH ranked_users AS (
        SELECT
            `, qualified.Columns, `,
            ROW_NUMBER() OVER (
                ORDER BY `, qualified.Col.Age, ` DESC
            ) AS age_rank
        FROM `, qualified.Name, `
        WHERE `, qualified.Col.Active, ` = ?
    )
    SELECT `, base.Columns, `
    FROM ranked_users
    WHERE age_rank <= ?
    ORDER BY age_rank
`), qualified.Col.Active.Val(true), 10)
if err != nil {
    return err
}
defer rows.Close()

users, err := base.ScanRows(rows)
```

The final projection is the important part: `ScanRow` and `ScanRows` expect every model column in generated `Columns` order. Partial projections and aggregate reports should use a purpose-built result struct and explicit `rows.Scan` calls. See `TestRawSQLScanRow`, `TestRawSQLScanRows`, and `TestRawSQLJoin` in `sql_test.go`.

## Generate and test

```sh
# from the Metago repository root
go run . ./experiments/activerecord/models

cd experiments/activerecord
go test ./...
```
