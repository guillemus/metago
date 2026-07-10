package main

import (
	"context"
	"testing"
)

func TestCreateListDelete(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(bootstrapSchema); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	u := &User{Name: "Ada", Email: "ada@example.com", Age: 36, Active: true}
	insertSQL := `
insert into ` + Users.Table + ` (
  ` + Users.InsertColumns + `
) values (
  ` + Users.InsertPlaceholders + `
)`
	result, err := db.ExecContext(ctx, insertSQL, UserInsertArgs(u)...)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	u.ID = id

	listSQL := `
select ` + Users.Columns + `
from ` + Users.Table + `
where ` + Users.Age + ` >= ?
  and ` + Users.Active + ` = ?
order by ` + Users.Name + ` asc`
	rows, err := db.QueryContext(ctx, listSQL, 18, true)
	if err != nil {
		t.Fatal(err)
	}
	list, err := ScanUsers(rows)
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "Ada" || list[0].ID != u.ID {
		t.Fatalf("list = %#v", list)
	}

	deleteSQL := `
delete from ` + Users.Table + `
where ` + Users.ID + ` = ?`
	if _, err := db.ExecContext(ctx, deleteSQL, u.ID); err != nil {
		t.Fatal(err)
	}

	rows, err = db.QueryContext(ctx, listSQL, 18, true)
	if err != nil {
		t.Fatal(err)
	}
	list, err = ScanUsers(rows)
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list after delete, got %#v", list)
	}
}

func TestGeneratedNames(t *testing.T) {
	if Users.Table != "users" {
		t.Fatalf("Table = %q", Users.Table)
	}
	if Users.Columns != "id, name, email, age, active, score, bio" {
		t.Fatalf("Columns = %q", Users.Columns)
	}
	if Users.Email != "email" || Users.Age != "age" {
		t.Fatalf("Users = %+v", Users)
	}
	if Users.InsertColumns != "name, email, age, active, score, bio" {
		t.Fatalf("InsertColumns = %q", Users.InsertColumns)
	}
	if Users.InsertPlaceholders != "?, ?, ?, ?, ?, ?" {
		t.Fatalf("InsertPlaceholders = %q", Users.InsertPlaceholders)
	}
	if Users.UpdateSet != "name = ?, email = ?, age = ?, active = ?, score = ?, bio = ?" {
		t.Fatalf("UpdateSet = %q", Users.UpdateSet)
	}
}
