# Raw SQL + Metago names experiment

You write the SQL. Metago generates table/column name fragments and scan helpers so schema renames stay compile-safe. No query builder.

## Annotations

```go
//mgo:gen tables

//mgo:props model table=users
type User struct {
	ID     int64  //mgo:props sql pk auto
	Name   string //mgo:props sql
	Email  string //mgo:props sql
	Age    int    //mgo:props sql
	Active bool   //mgo:props sql
	Score  float64
	Bio    *string
}
```

Generated surface (in `meta.go`):

```go
Users.Table               // "users"
Users.Name                // "name"
Users.Columns             // "id, name, email, ..."  — matches ScanUser
Users.InsertColumns       // without auto pk
Users.InsertPlaceholders  // "?, ?, ..."
Users.UpdateSet           // "name = ?, email = ?, ..."

ScanUser(row, &u)
ScanUsers(rows)
UserInsertArgs(u)
UserUpdateArgs(u) // non-pk fields, then pk
```

## Usage

```go
query := `
select ` + Users.Columns + `
from ` + Users.Table + `
where ` + Users.Age + ` >= ?
  and ` + Users.Active + ` = ?
order by ` + Users.Name + ` asc
`
rows, err := db.QueryContext(ctx, query, 18, true)
list, err := ScanUsers(rows)
```

```go
insert := `
insert into ` + Users.Table + ` (
  ` + Users.InsertColumns + `
) values (
  ` + Users.InsertPlaceholders + `
)`
_, err := db.ExecContext(ctx, insert, UserInsertArgs(u)...)
```

## Generate / run

```sh
# from the Metago repository root
go run . ./experiments/squirrel

# from this module
go test ./...
go run .
```
