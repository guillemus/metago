package models

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"
)

func insertTestUser(t *testing.T, Users *UserQuery, name, email string, age int, active bool) *User {
	t.Helper()
	u := &User{Name: name, Email: email, Age: age, Active: active}
	if err := Users.Insert(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	return u
}

func TestQueryScopesAreImmutable(t *testing.T) {
	db := testDB(t)
	Users := NewModels(db).Users
	insertTestUser(t, Users, "Ada", "ada@example.com", 37, true)
	insertTestUser(t, Users, "Bob", "bob@example.com", 15, false)
	insertTestUser(t, Users, "Grace", "grace@example.com", 45, true)

	base := Users
	adults := base.WhereAge.Gte(18)
	activeAdults := adults.WhereActive.Eq(true)

	assertCount(t, base, 3)
	assertCount(t, adults, 2)
	assertCount(t, activeAdults, 2)
	assertCount(t, base, 3)
}

func TestQueryPredicateComposition(t *testing.T) {
	db := testDB(t)
	Users := NewModels(db).Users
	insertTestUser(t, Users, "Ada", "ada@example.com", 37, true)
	insertTestUser(t, Users, "Bob", "bob@example.com", 15, false)
	insertTestUser(t, Users, "Grace", "grace@example.com", 45, true)

	t.Run("chained filters use AND", func(t *testing.T) {
		assertCount(t, Users.WhereActive.Eq(true).WhereAge.Gte(40), 1)
	})

	t.Run("Or combines complete predicate groups", func(t *testing.T) {
		olderActive := Users.WhereAge.Gte(40).WhereActive.Eq(true)
		youngerInactive := Users.WhereAge.Lt(18).WhereActive.Eq(false)
		assertCount(t, olderActive.Or(youngerInactive), 2)
	})

	t.Run("filters after Or use AND", func(t *testing.T) {
		query := Users.
			WhereName.Eq("Ada").
			Or(Users.WhereActive.Eq(false)).
			WhereAge.Gte(18)
		assertCount(t, query, 1)
	})

	t.Run("And composes queries", func(t *testing.T) {
		active := Users.WhereActive.Eq(true)
		older := Users.WhereAge.Gte(40)
		assertCount(t, active.And(older), 1)
	})

	t.Run("raw predicates compose and preserve argument order", func(t *testing.T) {
		query := Users.
			WhereRaw("name = ?", "Ada").
			Or(Users.WhereRaw("age = ?", 15))
		assertCount(t, query, 2)
	})

	t.Run("empty predicates are composition identities", func(t *testing.T) {
		empty := Users
		active := Users.WhereActive.Eq(true)
		assertCount(t, empty.Or(active), 2)
		assertCount(t, active.Or(empty), 2)
		assertCount(t, empty.And(active), 2)
	})

	t.Run("composition is immutable", func(t *testing.T) {
		active := Users.WhereActive.Eq(true)
		combined := active.Or(Users.WhereAge.Lt(18))
		assertCount(t, active, 2)
		assertCount(t, combined, 3)
	})
}

func TestEmptyOrDoesNotBypassDeleteProtection(t *testing.T) {
	db := testDB(t)
	Users := NewModels(db).Users
	insertTestUser(t, Users, "Ada", "ada@example.com", 37, true)

	if _, err := Users.Or(Users).Delete(context.Background()); err == nil {
		t.Fatal("empty OR should remain an unrestricted delete")
	}
	assertCount(t, Users, 1)
}

func TestModelsGroupsReusableQueryHandles(t *testing.T) {
	db := testDB(t)
	Models := NewModels(db)
	Users := Models.Users
	insertTestUser(t, Users, "Ada", "ada@example.com", 37, true)
	insertTestUser(t, Users, "Bob", "bob@example.com", 15, false)

	assertCount(t, Users, 2)
	assertCount(t, Users.WhereAge.Gte(18), 1)
	assertCount(t, Users, 2)

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	TxModels := Models.With(tx)
	assertCount(t, TxModels.Users, 2)
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
}

func TestOffsetWithoutLimitAndFirst(t *testing.T) {
	db := testDB(t)
	Users := NewModels(db).Users
	first := insertTestUser(t, Users, "Ada", "ada@example.com", 37, true)
	second := insertTestUser(t, Users, "Bob", "bob@example.com", 15, false)
	third := insertTestUser(t, Users, "Grace", "grace@example.com", 45, true)

	scope := Users.OrderByID.Asc().Offset(1)
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

	_, err = Users.Limit(0).First(context.Background())
	if err != sql.ErrNoRows {
		t.Fatalf("Limit(0).First error = %v, want sql.ErrNoRows", err)
	}
}

func TestRawSQLJoin(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Models := NewModels(db)
	user := insertTestUser(t, Models.Users, "Ada", "ada@example.com", 37, true)
	profile, err := Models.Profiles.Create(ctx, Profile{UserID: user.ID, DisplayName: "Ada Lovelace"})
	if err != nil {
		t.Fatal(err)
	}

	u, p := Tables.Users.Qualified(), Tables.Profiles.Qualified()
	row := db.QueryRowContext(ctx, fmt.Sprint(`
		SELECT `, u.Columns, `, `, p.Columns, `
		FROM `, u.Name, `
		JOIN `, p.Name, ` ON `, p.Col.UserID, ` = `, u.Col.ID, `
		WHERE `, u.Col.Email, ` = ?
	`), u.Col.Email.Val("ada@example.com"))

	var gotUser User
	var gotProfile Profile
	destinations := u.ScanDestinations(&gotUser)
	destinations = append(destinations, p.ScanDestinations(&gotProfile)...)
	if err := row.Scan(destinations...); err != nil {
		t.Fatal(err)
	}
	if gotUser.ID != user.ID || gotProfile.ID != profile.ID || gotProfile.DisplayName != "Ada Lovelace" {
		t.Fatalf("joined records = %#v, %#v", gotUser, gotProfile)
	}
}

func TestRawSQLScanRow(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Users := NewModels(db).Users

	for _, user := range []User{
		{Name: "Ada", Email: "ada@example.com", Age: 37, Active: true, Score: 98.5},
		{Name: "Bob", Email: "bob@example.com", Age: 29, Active: true, Score: 72},
		{Name: "Grace", Email: "grace@example.com", Age: 45, Active: false, Score: 100},
	} {
		if _, err := Users.Create(ctx, user); err != nil {
			t.Fatal(err)
		}
	}

	u := Tables.Users.Qualified()
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

	var got User
	if err := u.ScanRow(row, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "Ada" || got.Score != 98.5 {
		t.Fatalf("highest-scoring active user = %#v", got)
	}
}

func TestRawSQLScanRows(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Users := NewModels(db).Users

	for _, user := range []User{
		{Name: "Ada", Email: "ada@example.com", Age: 37, Active: true},
		{Name: "Bob", Email: "bob@example.com", Age: 29, Active: true},
		{Name: "Grace", Email: "grace@example.com", Age: 45, Active: true},
		{Name: "Linus", Email: "linus@example.com", Age: 24, Active: false},
	} {
		if _, err := Users.Create(ctx, user); err != nil {
			t.Fatal(err)
		}
	}

	qualified := Tables.Users.Qualified()
	base := Tables.Users
	rows, err := db.QueryContext(ctx, fmt.Sprint(`
		WITH ranked_users AS (
			SELECT
				`, qualified.Columns, `,
				ROW_NUMBER() OVER (
					ORDER BY `, qualified.Col.Age, ` DESC, `, qualified.Col.ID, ` ASC
				) AS age_rank
			FROM `, qualified.Name, `
			WHERE `, qualified.Col.Active, ` = ?
		)
		SELECT `, base.Columns, `
		FROM ranked_users
		WHERE age_rank <= ?
		ORDER BY age_rank
	`), qualified.Col.Active.Val(true), 2)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	got, err := base.ScanRows(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "Grace" || got[1].Name != "Ada" {
		t.Fatalf("oldest active users = %#v", got)
	}
}

func TestStaticTableMetadata(t *testing.T) {
	base := Tables.Users
	qualified := base.Qualified()
	if base.Col.ID != "id" || base.Columns[:2] != "id" {
		t.Fatalf("base table metadata = %#v", base)
	}
	if qualified.Col.ID != "users.id" || qualified.Col.Email != "users.email" {
		t.Fatalf("qualified table metadata = %#v", qualified)
	}
	if Tables.Users.Col.ID != "id" {
		t.Fatal("Qualified mutated package-level metadata")
	}
}

func TestExtendedSQLTypes(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Agents := NewModels(db).Agents
	now := time.Now().UTC().Truncate(time.Second)
	agent := Agent{
		Status: AgentStatusReady, CreatedAt: now,
		Nickname: sql.NullString{String: "Ada", Valid: true},
		Payload:  []byte{1, 2, 3},
	}
	if err := Agents.Insert(ctx, &agent); err != nil {
		t.Fatal(err)
	}
	got, err := Agents.WhereStatus.Eq(AgentStatusReady).WhereCreatedAt.Gte(now).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != AgentStatusReady || got.Nickname.String != "Ada" || string(got.Payload) != string(agent.Payload) {
		t.Fatalf("agent = %#v", got)
	}
	if Tables.Agents.Col.CreatedAt != "created_at" || Tables.Agents.Col.Payload != "payload" {
		t.Fatalf("agent columns = %#v", Tables.Agents.Col)
	}
}

func TestPlainRecordPersistence(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	Users := NewModels(db).Users
	user := User{Name: "Ada", Email: "ada@example.com", Age: 36}

	if err := Users.Insert(ctx, &user); err != nil {
		t.Fatal(err)
	}
	if user.ID == 0 {
		t.Fatal("Insert did not assign ID")
	}
	user.Age = 37
	if err := Users.Update(ctx, &user); err != nil {
		t.Fatal(err)
	}
	user.Age = 0
	if err := Users.Reload(ctx, &user); err != nil {
		t.Fatal(err)
	}
	if user.Age != 37 {
		t.Fatalf("reloaded age = %d", user.Age)
	}
	if err := Users.DeleteRecord(ctx, &user); err != nil {
		t.Fatal(err)
	}
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
