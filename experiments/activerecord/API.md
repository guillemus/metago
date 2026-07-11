# Generated repository API

```go
users := Users(db)
```

## Create and write plain records

```go
user := User{Name: "Ada", Email: "ada@example.com", Age: 36}
err := users.Insert(ctx, &user)

user, err = users.Create(ctx, User{Name: "Grace", Email: "grace@example.com"})

user.Age = 37
err = users.Update(ctx, user)
err = users.Reload(ctx, user)
err = users.DeleteRecord(ctx, user)
```

Models contain no database field and have no persistence methods.

## Find and query

```go
user, err := users.Find(ctx, 1)
user, err = users.FindByEmail(ctx, "ada@example.com")

list, err := users.
    WhereAge.Gte(18).
    WhereActive.Eq(true).
    OrderByName.Asc().
    Limit(20).
    Offset(0).
    All(ctx)

one, err := users.WhereEmail.Eq("ada@example.com").First(ctx)
count, err := users.WhereActive.Eq(true).Count(ctx)
exists, err := users.WhereEmail.Eq("ada@example.com").Exists(ctx)
```

Scopes are immutable and reusable. `WhereRaw` remains available for conditions outside the generated operators.

## Delete a scope

```go
count, err := users.WhereActive.Eq(false).Delete(ctx)
```

Unrestricted query deletion is refused. Use `DeleteRecord` for one model value.

## Transactions and grouped repositories

```go
models := NewModels(db)
users, err := models.Users.All(ctx)

txModels := models.With(tx)
err = txModels.Users.Insert(ctx, &user)
```

## Raw SQL metadata

```go
u := Tables.Users
u.Name       // users
u.Col.Email  // email
u.Columns    // id, name, email, ...

uq := u.Qualified()
uq.Col.Email // users.email
```
