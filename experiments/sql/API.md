# SQL model API shape

```go
users := Users(db)
```

## Create

```go
// Preferred when the record will use Save/Update/Delete/Reload.
u := Users(db).New()
u.Name = "Ada"
u.Email = "ada@example.com"
u.Age = 36
err := u.Save(ctx)

// Direct create/insert remain available.
u, err := Users(db).Create(ctx, User{
    Name:  "Ada",
    Email: "ada@example.com",
    Age:   36,
})

u = &User{Name: "Ada", Email: "ada@example.com", Age: 36}
err = Users(db).Insert(ctx, u)
```

Record methods panic when called on a value that was not constructed, inserted, or loaded through a model handle.

## Find

```go
u, err := Users(db).Find(ctx, 1)
u, err := Users(db).FindByEmail(ctx, "ada@example.com")
```

## Filter / list

Query scopes are immutable and can be safely reused.

```go
all, err := Users(db).All(ctx)

list, err := Users(db).
    WhereAge.Gte(18).
    WhereActive.Eq(true).
    WhereName.Eq("Ada").
    OrderByName.Asc().
    Limit(20).
    Offset(0).
    All(ctx)

one, err := Users(db).
    WhereEmail.Eq("ada@example.com").
    First(ctx)

n, err := Users(db).WhereActive.Eq(true).Count(ctx)
ok, err := Users(db).WhereEmail.Eq("ada@example.com").Exists(ctx)
```

## Field ops (typed)

```go
WhereAge.Eq(18)
WhereAge.Neq(0)
WhereAge.Gte(18)
WhereAge.Lte(65)
WhereAge.Gt(17)
WhereAge.Lt(66)
WhereAge.In(18, 21, 30)

WhereName.Eq("Ada")
WhereName.Neq("Bob")
WhereName.Like("Ad%")
WhereName.In("Ada", "Grace")

WhereBio.IsNull()
WhereBio.IsNotNull()

WhereRaw("age > ? OR name = ?", 18, "Ada")
```

## Order

```go
OrderByName.Asc()
OrderByName.Desc()
OrderByAge.Desc()
```

## Record writes

```go
u, err := Users(db).Find(ctx, 1)

u.Name = "Augusta"
err = u.Update(ctx)
err = u.Save(ctx)
err = u.Reload(ctx)
err = u.Delete(ctx)
```

## Bulk via query

```go
n, err := Users(db).WhereActive.Eq(false).Delete(ctx)
```

## Full loop

```go
users := Users(db)

u, err := users.Create(ctx, User{
    Name: "Ada", Email: "ada@example.com", Age: 36, Active: true,
})

u.Age = 37
err = u.Update(ctx)

list, err := users.
    WhereAge.Gte(18).
    WhereActive.Eq(true).
    OrderByName.Asc().
    All(ctx)

err = u.Delete(ctx)
```
