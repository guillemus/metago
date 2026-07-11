package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"github.com/jmoiron/sqlx"
)

func testDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(bootstrapSchema); err != nil {
		t.Fatal(err)
	}
	return db
}

func insertUser(t *testing.T, db *sqlx.DB, user User) *User {
	t.Helper()
	u := Tables.Users
	result, err := db.NamedExec(`
		INSERT INTO `+u.Name+` (`+u.InsertColumns+`)
		VALUES (`+u.InsertValues+`)
	`, &user)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	user.ID = UserID(id)
	return &user
}

func seedUsers(t *testing.T, db *sqlx.DB) []*User {
	t.Helper()
	return []*User{
		insertUser(t, db, User{Name: "Ada", Email: "ada@example.com", Age: 37, Active: true, Score: 98.5}),
		insertUser(t, db, User{Name: "Bob", Email: "bob@example.com", Age: 15, Active: false, Score: 72}),
		insertUser(t, db, User{Name: "Grace", Email: "grace@example.com", Age: 45, Active: true, Score: 100}),
	}
}

func TestRawCRUDWithTaggedStructs(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	u := Tables.Users
	bio := "programmer"
	user := insertUser(t, db, User{Name: "Ada", Email: "ada@example.com", Age: 36, Active: true, Score: 9.5, Bio: &bio})

	var loaded User
	if err := db.GetContext(ctx, &loaded, `
		SELECT `+u.Columns+`
		FROM `+u.Name+`
		WHERE `+string(u.Col.ID)+` = ?
	`, u.Col.ID.Val(user.ID)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, *user) {
		t.Fatalf("inserted user = %#v, want %#v", loaded, *user)
	}

	user.Name, user.Age, user.Bio = "Augusta", 37, nil
	result, err := db.NamedExecContext(ctx, `
		UPDATE `+u.Name+`
		SET `+u.UpdateSet+`
		WHERE `+string(u.Col.ID)+` = :id
	`, user)
	if err != nil {
		t.Fatal(err)
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		t.Fatalf("updated rows = %d", rows)
	}
	if err := db.GetContext(ctx, &loaded, `
		SELECT `+u.Columns+`
		FROM `+u.Name+`
		WHERE `+string(u.Col.ID)+` = ?
	`, user.ID); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, *user) {
		t.Fatalf("updated user = %#v, want %#v", loaded, *user)
	}

	result, err = db.ExecContext(ctx, `
		DELETE FROM `+u.Name+`
		WHERE `+string(u.Col.ID)+` = ?
	`, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		t.Fatalf("deleted rows = %d", rows)
	}
	if err := db.GetContext(ctx, &loaded, `
		SELECT `+u.Columns+`
		FROM `+u.Name+`
		WHERE `+string(u.Col.ID)+` = ?
	`, user.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted lookup error = %v", err)
	}
}

func TestRawFilterOperatorMatrix(t *testing.T) {
	db := testDB(t)
	seedUsers(t, db)
	u := Tables.Users
	ctx := context.Background()

	tests := []struct {
		name  string
		where string
		args  []any
		want  int
	}{
		{"Eq", string(u.Col.Name + " = ?"), []any{u.Col.Name.Val("Ada")}, 1},
		{"Neq", string(u.Col.Name + " <> ?"), []any{u.Col.Name.Val("Ada")}, 2},
		{"In", string(u.Col.Age + " IN (?, ?)"), []any{u.Col.Age.Val(15), u.Col.Age.Val(45)}, 2},
		{"Gt", string(u.Col.Age + " > ?"), []any{u.Col.Age.Val(37)}, 1},
		{"Gte", string(u.Col.Age + " >= ?"), []any{u.Col.Age.Val(37)}, 2},
		{"Lt", string(u.Col.Age + " < ?"), []any{u.Col.Age.Val(37)}, 1},
		{"Lte", string(u.Col.Age + " <= ?"), []any{u.Col.Age.Val(37)}, 2},
		{"Like", string(u.Col.Name + " LIKE ?"), []any{u.Col.Name.Val("A%")}, 1},
		{"IsNull", string(u.Col.Bio + " IS NULL"), nil, 3},
		{"IsNotNull", string(u.Col.Bio + " IS NOT NULL"), nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var count int
			if err := db.GetContext(ctx, &count, `
				SELECT COUNT(*)
				FROM `+u.Name+`
				WHERE `+tt.where, tt.args...); err != nil {
				t.Fatal(err)
			}
			if count != tt.want {
				t.Fatalf("count = %d, want %d", count, tt.want)
			}
		})
	}
}

func TestRawPredicateGroupingOrderingAndPagination(t *testing.T) {
	db := testDB(t)
	seedUsers(t, db)
	u := Tables.Users
	query := `
		SELECT ` + u.Columns + `
		FROM ` + u.Name + `
		WHERE ((` + string(u.Col.Name) + ` = ? OR ` + string(u.Col.Active) + ` = ?) AND ` + string(u.Col.Age) + ` >= ?)
		ORDER BY ` + string(u.Col.Age) + ` DESC, ` + string(u.Col.ID) + ` ASC
		LIMIT ? OFFSET ?
	`
	var got []User
	if err := db.Select(&got, query,
		u.Col.Name.Val("Ada"), u.Col.Active.Val(false), u.Col.Age.Val(18), 2, 0,
	); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Ada" {
		t.Fatalf("grouped query = %#v", got)
	}

	got = nil
	if err := db.Select(&got, `
		SELECT `+u.Columns+`
		FROM `+u.Name+`
		ORDER BY `+string(u.Col.Age)+` DESC
		LIMIT 1 OFFSET 1
	`); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Ada" {
		t.Fatalf("paginated query = %#v", got)
	}
}

func TestRawJoinAndScanDestinations(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	user := insertUser(t, db, User{Name: "Ada", Email: "ada@example.com"})
	p := Tables.Profiles
	profile := Profile{UserID: user.ID, DisplayName: "Ada Lovelace"}
	result, err := db.NamedExecContext(ctx, `
		INSERT INTO `+p.Name+` (`+p.InsertColumns+`)
		VALUES (`+p.InsertValues+`)
	`, &profile)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	profile.ID = ProfileID(id)

	uq, pq := Tables.Users.Qualified(), Tables.Profiles.Qualified()
	row := db.QueryRowxContext(ctx, `
		SELECT `+uq.Columns+`, `+pq.Columns+`
		FROM `+uq.Name+`
		JOIN `+pq.Name+` ON `+string(pq.Col.UserID)+` = `+string(uq.Col.ID)+`
		WHERE `+string(uq.Col.Email)+` = ?
	`, uq.Col.Email.Val("ada@example.com"))
	var gotUser User
	var gotProfile Profile
	destinations := uq.ScanDestinations(&gotUser)
	destinations = append(destinations, pq.ScanDestinations(&gotProfile)...)
	if err := row.Scan(destinations...); err != nil {
		t.Fatal(err)
	}
	if gotUser.ID != user.ID || !reflect.DeepEqual(gotProfile, profile) {
		t.Fatalf("joined records = %#v, %#v", gotUser, gotProfile)
	}
}

func TestRawCTEAndScanRows(t *testing.T) {
	db := testDB(t)
	seedUsers(t, db)
	uq, u := Tables.Users.Qualified(), Tables.Users
	rows, err := db.Queryx(`
		WITH ranked_users AS (
			SELECT `+uq.Columns+`,
				ROW_NUMBER() OVER (
					ORDER BY `+string(uq.Col.Age)+` DESC, `+string(uq.Col.ID)+` ASC
				) AS age_rank
			FROM `+uq.Name+`
			WHERE `+string(uq.Col.Active)+` = ?
		)
		SELECT `+u.Columns+`
		FROM ranked_users
		WHERE age_rank <= ?
		ORDER BY age_rank
	`, uq.Col.Active.Val(true), 2)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got, err := u.ScanRows(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "Grace" || got[1].Name != "Ada" {
		t.Fatalf("ranked users = %#v", got)
	}
}

func TestThreeTableQueriesAndTransactions(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	user := insertUser(t, db, User{Name: "Ada", Email: "ada@example.com"})

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	team := Team{Name: "Core"}
	tm := Tables.Teams
	if _, err := tx.NamedExecContext(ctx, `
		INSERT INTO `+tm.Name+` (`+tm.InsertColumns+`)
		VALUES (`+tm.InsertValues+`)
	`, &team); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.Get(&count, `SELECT COUNT(*) FROM `+tm.Name); err != nil || count != 0 {
		t.Fatalf("rollback count = %d, %v", count, err)
	}

	p := Tables.Profiles
	profile := Profile{UserID: user.ID, DisplayName: "Ada"}
	if _, err := db.NamedExec(`
		INSERT INTO `+p.Name+` (`+p.InsertColumns+`)
		VALUES (`+p.InsertValues+`)
	`, &profile); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&count, `
		SELECT COUNT(*) FROM `+p.Name+`
		WHERE `+string(p.Col.UserID)+` = ?
	`, p.Col.UserID.Val(user.ID)); err != nil || count != 1 {
		t.Fatalf("profile count = %d, %v", count, err)
	}
}

func TestMetadataAndErrors(t *testing.T) {
	u := Tables.Users
	if u.Name != "users" || u.Columns != "id, name, email, age, active, score, bio" {
		t.Fatalf("user metadata = %#v", u)
	}
	if u.InsertValues != ":name, :email, :age, :active, :score, :bio" {
		t.Fatalf("insert values = %q", u.InsertValues)
	}
	if got := u.Qualified().Qualified(); got.Col.ID != "users.id" {
		t.Fatalf("Qualified is not idempotent: %q", got.Col.ID)
	}
	if Tables.Profiles.Col.UserID != "user_id" || Tables.Teams.Col.Description != "description" {
		t.Fatal("three-table metadata is incomplete")
	}

	db := testDB(t)
	seedUsers(t, db)
	if _, err := db.NamedExec(`
		INSERT INTO `+u.Name+` (`+u.InsertColumns+`)
		VALUES (`+u.InsertValues+`)
	`, &User{Name: "Duplicate", Email: "ada@example.com"}); err == nil {
		t.Fatal("unique constraint violation succeeded")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	var users []User
	if err := db.SelectContext(cancelled, &users, `
		SELECT `+u.Columns+`
		FROM `+u.Name+`
	`); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled query error = %v", err)
	}
}
