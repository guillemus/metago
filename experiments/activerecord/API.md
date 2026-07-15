# Generated repository API

```go
Models := models.NewModels(db)
Users := Models.Users
```

## Create and write plain records

```go
user := models.User{Name: "Ada", Email: "ada@example.com", Age: 36}
err := Users.Insert(ctx, &user)

user, err = Users.Create(ctx, models.User{Name: "Grace", Email: "grace@example.com"})

user.Age = 37
err = Users.Update(ctx, user)
err = Users.Reload(ctx, user)
err = Users.DeleteRecord(ctx, user)
```

Models contain no database field and have no persistence methods.

## Find and query

```go
user, err := Users.Find(ctx, 1)
user, err = Users.FindByEmail(ctx, "ada@example.com")

list, err := Users.
    WhereAge.Gte(18).
    WhereActive.Eq(true).
    OrderByName.Asc().
    Limit(20).
    Offset(0).
    All(ctx)

one, err := Users.WhereEmail.Eq("ada@example.com").First(ctx)
count, err := Users.WhereActive.Eq(true).Count(ctx)
exists, err := Users.WhereEmail.Eq("ada@example.com").Exists(ctx)
```

Scopes are immutable and reusable. Chained filters use `AND`; combine complete predicate groups with
`Or` or explicit `And`:

```go
query := Users.
    WhereName.Eq("Ada").
    Or(Users.WhereActive.Eq(false)).
    WhereAge.Gte(18)
// (name = ? OR active = ?) AND age >= ?
```

Only predicates are taken from the right-hand query; ordering and pagination remain those of the
receiver. `WhereRaw` remains available for conditions outside the generated operators.

## Delete a scope

```go
count, err := Users.WhereActive.Eq(false).Delete(ctx)
```

Unrestricted query deletion is refused. Use `DeleteRecord` for one model value.

## Transactions and grouped repositories

```go
Models := models.NewModels(db)
users, err := Models.Users.All(ctx)

TxModels := Models.With(tx)
err = TxModels.Users.Insert(ctx, &user)
```

## Raw SQL metadata

```go
u := models.Tables.Users
u.Name       // users
u.Col.Email  // email
u.Columns    // id, name, email, ...

uq := u.Qualified()
uq.Col.Email // users.email

// Columns carry their model field type into raw SQL arguments.
rows, err := db.QueryContext(ctx, fmt.Sprint(
    `SELECT `, u.Columns,
    ` FROM `, u.Name,
    ` WHERE `, u.Col.Email, ` = ?`,
), u.Col.Email.Val(email)) // email must be a string
```

`Column[T]` has `string` as its underlying type. Pass columns directly to `fmt.Sprint` when
assembling SQL; `Val` is an identity operation that enforces `T` at compile time.
