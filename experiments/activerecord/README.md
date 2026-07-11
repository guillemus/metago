# Generated SQL repository experiment

Metago generates typed, immutable query scopes and repository operations while model structs remain plain Go data.

## Models

```go
//mgo:gen queries

type AgentStatus string

//mgo:props model table=users
type User struct {
    ID        int64       //mgo:props sql pk auto filter sort
    Email     string      //mgo:props sql unique filter
    Status    AgentStatus //mgo:props sql filter
    CreatedAt time.Time   //mgo:props sql filter sort
}
```

Every persisted field is explicitly marked with `//mgo:props sql`. This works for scalars, named types, `time.Time`, `sql.Null*`, `[]byte`, and arbitrary `sql.Scanner`/`driver.Valuer` types without guessing. `filter` and `sort` remain explicit capabilities.

## Plain records and persistence

Models contain no hidden database connection. All persistence goes through a database-scoped repository:

```go
users := Users(db)
user := User{Name: "Ada", Email: "ada@example.com"}

err := users.Insert(ctx, &user) // assigns an automatic ID
user.Name = "Augusta"
err = users.Update(ctx, &user)
err = users.Reload(ctx, &user)
err = users.DeleteRecord(ctx, &user)
```

`Create` is also available when a returned pointer is convenient:

```go
user, err := users.Create(ctx, User{Name: "Ada", Email: "ada@example.com"})
```

Query deletion remains separate:

```go
count, err := users.WhereActive.Eq(false).Delete(ctx)
```

## Typed queries

```go
list, err := Users(db).
    WhereAge.Gte(18).
    WhereActive.Eq(true).
    OrderByName.Asc().
    Limit(20).
    All(ctx)
```

Queries are immutable. `NewModels(db)` groups reusable handles, and `models.With(tx)` scopes the same repositories to a transaction.

## Static schema metadata and raw joins

`Tables` contains connection-independent, collision-safe schema metadata. Columns are unqualified by default:

```go
Tables.Users.Name       // "users"
Tables.Users.Col.Email  // "email"
Tables.Users.Columns    // "id, name, email, ..."
```

Call `Qualified` only when a join needs table prefixes:

```go
u := Tables.Users.Qualified()
p := Tables.Profiles.Qualified()

row := db.QueryRowContext(ctx, `
    SELECT `+u.Columns+`, `+p.Columns+`
    FROM `+u.Name+`
    JOIN `+p.Name+` ON `+p.Col.UserID+` = `+u.Col.ID+`
    WHERE `+u.Col.Email+` = ?
`, email)

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
u := Tables.Users.Qualified()
row := db.QueryRowContext(ctx, `
    SELECT `+u.Columns+`
    FROM `+u.Name+`
    WHERE `+u.Col.Active+` = ?
      AND `+u.Col.Score+` = (
        SELECT MAX(candidate.score)
        FROM users AS candidate
        WHERE candidate.active = ?
      )
`, true, true)

var user User
err := u.ScanRow(row, &user)
```

### Complex raw SQL returning multiple models

`ScanRows` handles complete model rows returned by CTEs, window functions, unions, or other custom SQL:

```go
qualified := Tables.Users.Qualified()
base := Tables.Users
rows, err := db.QueryContext(ctx, `
    WITH ranked_users AS (
        SELECT
            `+qualified.Columns+`,
            ROW_NUMBER() OVER (
                ORDER BY `+qualified.Col.Age+` DESC
            ) AS age_rank
        FROM `+qualified.Name+`
        WHERE `+qualified.Col.Active+` = ?
    )
    SELECT `+base.Columns+`
    FROM ranked_users
    WHERE age_rank <= ?
    ORDER BY age_rank
`, true, 10)
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
go run . ./experiments/activerecord

cd experiments/activerecord
go test ./...
```
