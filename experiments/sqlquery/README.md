# Typed `database/sql` query experiment

This experiment generates typed methods directly from annotated SQL constants. It uses only the
standard `database/sql` package at runtime.

```go
//mgo:gen sql.one in=Session.Token,Session.Expiry out=Session.Data
const findSessionSQL = `
    SELECT data FROM sessions
    WHERE token = :token AND expiry > :expiry
`
```

The templates cover:

- `sql.exec`: execute without scanning rows and return `sql.Result`.
- `sql.one`: scan one scalar or structured row.
- `sql.many`: scan scalar or structured rows into a slice.

A scalar output uses `out=Type.Field`. Structured output uses `out=Type` plus an explicit
`scan=Type.Field,...` list in SELECT-column order.

Generate and test from the repository root:

```sh
go run . ./experiments/sqlquery
cd experiments/sqlquery
go test ./...
```
