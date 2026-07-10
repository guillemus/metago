package main

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(bootstrapSchema); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func insertTestUser(t *testing.T, db DBTX, name, email string, age int, active bool) *User {
	t.Helper()
	u := Users(db).New()
	u.Name = name
	u.Email = email
	u.Age = age
	u.Active = active
	if err := u.Save(context.Background()); err != nil {
		t.Fatal(err)
	}
	return u
}

func TestQueryScopesAreImmutable(t *testing.T) {
	db := testDB(t)
	insertTestUser(t, db, "Ada", "ada@example.com", 37, true)
	insertTestUser(t, db, "Bob", "bob@example.com", 15, false)
	insertTestUser(t, db, "Grace", "grace@example.com", 45, true)

	base := Users(db)
	adults := base.WhereAge.Gte(18)
	activeAdults := adults.WhereActive.Eq(true)

	assertCount(t, base, 3)
	assertCount(t, adults, 2)
	assertCount(t, activeAdults, 2)
	assertCount(t, base, 3)
}

func TestModelsGroupsReusableQueryHandles(t *testing.T) {
	db := testDB(t)
	models := NewModels(db)
	insertTestUser(t, db, "Ada", "ada@example.com", 37, true)
	insertTestUser(t, db, "Bob", "bob@example.com", 15, false)

	assertCount(t, models.Users, 2)
	assertCount(t, models.Users.WhereAge.Gte(18), 1)
	assertCount(t, models.Users, 2)

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	txModels := models.With(tx)
	assertCount(t, txModels.Users, 2)
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
}

func TestOffsetWithoutLimitAndFirst(t *testing.T) {
	db := testDB(t)
	first := insertTestUser(t, db, "Ada", "ada@example.com", 37, true)
	second := insertTestUser(t, db, "Bob", "bob@example.com", 15, false)
	third := insertTestUser(t, db, "Grace", "grace@example.com", 45, true)

	scope := Users(db).OrderByID.Asc().Offset(1)
	rows, err := scope.All(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != second.ID || rows[1].ID != third.ID {
		t.Fatalf("offset rows = %#v; first inserted ID was %d", rows, first.ID)
	}

	row, err := scope.First(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if row.ID != second.ID {
		t.Fatalf("First with offset returned ID %d, want %d", row.ID, second.ID)
	}

	_, err = Users(db).Limit(0).First(context.Background())
	if err != sql.ErrNoRows {
		t.Fatalf("Limit(0).First error = %v, want sql.ErrNoRows", err)
	}
}

func TestForeignKeyMetadata(t *testing.T) {
	if len(postModel.foreignKeys) != 2 {
		t.Fatalf("Post foreign keys = %#v", postModel.foreignKeys)
	}
	if got := postModel.foreignKeys[0]; got.column != "project_id" || got.references != "projects.id" {
		t.Fatalf("first Post foreign key = %#v", got)
	}
	if got := commentModel.foreignKeys[2]; got.column != "parent_id" || got.references != "comments.id" {
		t.Fatalf("Comment parent foreign key = %#v", got)
	}
}

func TestUnattachedRecordPanics(t *testing.T) {
	defer func() {
		value := recover()
		if value == nil {
			t.Fatal("Save did not panic")
		}
		if message := value.(string); !strings.Contains(message, "Users(db).New()") {
			t.Fatalf("panic = %q", message)
		}
	}()

	_ = (&User{Name: "Ada"}).Save(context.Background())
}

func assertCount(t *testing.T, query *UserQuery, want int64) {
	t.Helper()
	got, err := query.Count(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Count = %d, want %d", got, want)
	}
}
