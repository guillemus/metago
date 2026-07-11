package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func seedUsers(t *testing.T, Users *UserQuery) []*User {
	t.Helper()
	return []*User{
		insertTestUser(t, Users, "Ada", "ada@example.com", 37, true),
		insertTestUser(t, Users, "Bob", "bob@example.com", 15, false),
		insertTestUser(t, Users, "Grace", "grace@example.com", 45, true),
	}
}

func queryNames(t *testing.T, query *UserQuery) []string {
	t.Helper()
	rows, err := query.All(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(rows))
	for i := range rows {
		names[i] = rows[i].Name
	}
	return names
}

func TestGeneratedFilterOperators(t *testing.T) {
	db := testDB(t)
	Users := NewModels(db).Users
	seedUsers(t, Users)

	tests := []struct {
		name  string
		query *UserQuery
		want  int64
	}{
		{"Eq", Users.WhereName.Eq("Ada"), 1},
		{"Neq", Users.WhereName.Neq("Ada"), 2},
		{"In one", Users.WhereAge.In(37), 1},
		{"In many", Users.WhereAge.In(15, 45), 2},
		{"In empty", Users.WhereAge.In(), 0},
		{"Gt", Users.WhereAge.Gt(37), 1},
		{"Gte", Users.WhereAge.Gte(37), 2},
		{"Lt", Users.WhereAge.Lt(37), 1},
		{"Lte", Users.WhereAge.Lte(37), 2},
		{"Like", Users.WhereName.Like("A%"), 1},
		{"bool Neq", Users.WhereActive.Neq(true), 1},
		{"named ID In", Users.WhereID.In(UserID(1), UserID(3)), 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { assertCount(t, tt.query, tt.want) })
	}
}

func TestNullableFilterOperators(t *testing.T) {
	db := testDB(t)
	Models := NewModels(db)
	ctx := context.Background()
	user := insertTestUser(t, Models.Users, "Ada", "ada@example.com", 37, true)
	team, err := Models.Teams.Create(ctx, Team{Name: "Core"})
	if err != nil {
		t.Fatal(err)
	}
	project, err := Models.Projects.Create(ctx, Project{TeamID: team.ID, OwnerID: user.ID, Name: "Metago"})
	if err != nil {
		t.Fatal(err)
	}
	post, err := Models.Posts.Create(ctx, Post{ProjectID: project.ID, UserID: user.ID, Title: "One"})
	if err != nil {
		t.Fatal(err)
	}
	parent, err := Models.Comments.Create(ctx, Comment{PostID: post.ID, UserID: user.ID, Body: "parent"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Models.Comments.Create(ctx, Comment{PostID: post.ID, UserID: user.ID, ParentID: &parent.ID, Body: "child"})
	if err != nil {
		t.Fatal(err)
	}

	count := func(query *CommentQuery) int64 {
		t.Helper()
		got, err := query.Count(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := count(Models.Comments.WhereParentID.IsNull()); got != 1 {
		t.Fatalf("IsNull count = %d", got)
	}
	if got := count(Models.Comments.WhereParentID.IsNotNull()); got != 1 {
		t.Fatalf("IsNotNull count = %d", got)
	}
	if got := count(Models.Comments.WhereParentID.Eq(parent.ID)); got != 1 {
		t.Fatalf("nullable Eq count = %d", got)
	}
	if got := count(Models.Comments.WhereParentID.Neq(parent.ID)); got != 0 {
		t.Fatalf("nullable Neq count = %d", got)
	}
	bio := "engineer"
	rank := 7
	if err := Models.Users.Update(ctx, &User{ID: user.ID, Name: user.Name, Email: user.Email, Age: user.Age, Active: user.Active, Bio: &bio, Rank: &rank}); err != nil {
		t.Fatal(err)
	}
	if got, err := Models.Users.WhereBio.Like("eng%").Count(ctx); err != nil || got != 1 {
		t.Fatalf("nullable Like count = %d, %v", got, err)
	}
	if got, err := Models.Users.WhereBio.IsNotNull().Count(ctx); err != nil || got != 1 {
		t.Fatalf("nullable text IsNotNull count = %d, %v", got, err)
	}
	if got, err := Models.Users.WhereRank.Gte(7).Count(ctx); err != nil || got != 1 {
		t.Fatalf("nullable ordered Gte count = %d, %v", got, err)
	}
}

func TestNestedPredicateTrees(t *testing.T) {
	db := testDB(t)
	Users := NewModels(db).Users
	seedUsers(t, Users)

	a := Users.WhereName.Eq("Ada")
	b := Users.WhereAge.Eq(15)
	c := Users.WhereName.Eq("Grace")
	assertCount(t, a.Or(b.Or(c)), 3)
	assertCount(t, a.Or(b).And(Users.WhereActive.Eq(true)), 1)
	assertCount(t, a.Or(b).And(a.Or(c)), 1)
	assertCount(t, a.Or(b).Or(c), 3)
	assertCount(t, a.And(Users.WhereActive.Eq(true)).And(Users.WhereAge.Gte(18)), 1)
	assertCount(t, a.Or(nil), 1)
	assertCount(t, a.And(nil), 1)

	left := Users.OrderByName.Desc().Limit(2).Offset(1).WhereActive.Eq(true)
	right := Users.OrderByAge.Asc().Limit(0).Offset(99).WhereAge.Eq(15)
	if got := queryNames(t, left.Or(right)); !reflect.DeepEqual(got, []string{"Bob", "Ada"}) {
		t.Fatalf("receiver execution settings not retained: %v", got)
	}
}

func TestFindUniqueLookupsAndExecution(t *testing.T) {
	db := testDB(t)
	Models := NewModels(db)
	Users := Models.Users
	seeded := seedUsers(t, Users)
	ctx := context.Background()

	found, err := Users.Find(ctx, seeded[1].ID)
	if err != nil || found.Name != "Bob" {
		t.Fatalf("Find = %#v, %v", found, err)
	}
	if _, err := Users.Find(ctx, UserID(999)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing Find error = %v", err)
	}
	found, err = Users.FindByEmail(ctx, "grace@example.com")
	if err != nil || found.Name != "Grace" {
		t.Fatalf("FindByEmail = %#v, %v", found, err)
	}
	if _, err := Users.FindByEmail(ctx, "missing@example.com"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing FindByEmail error = %v", err)
	}

	first, err := Users.First(ctx)
	if err != nil || first.ID != seeded[0].ID {
		t.Fatalf("default First = %#v, %v", first, err)
	}
	first, err = Users.OrderByAge.Desc().First(ctx)
	if err != nil || first.Name != "Grace" {
		t.Fatalf("ordered First = %#v, %v", first, err)
	}
	if _, err := Users.WhereName.Eq("Nobody").First(ctx); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("empty First error = %v", err)
	}

	if exists, err := Users.WhereName.Eq("Ada").Exists(ctx); err != nil || !exists {
		t.Fatalf("Exists = %v, %v", exists, err)
	}
	if exists, err := Users.WhereName.Eq("Nobody").Exists(ctx); err != nil || exists {
		t.Fatalf("missing Exists = %v, %v", exists, err)
	}
	if exists, err := Users.Limit(0).Exists(ctx); err != nil || exists {
		t.Fatalf("Limit(0).Exists = %v, %v", exists, err)
	}
	if exists, err := Users.OrderByID.Asc().Offset(3).Exists(ctx); err != nil || exists {
		t.Fatalf("Offset.Exists = %v, %v", exists, err)
	}

	empty, err := Users.WhereName.Eq("Nobody").All(ctx)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty All = %#v, %v", empty, err)
	}
}

func TestOrderingAndPaginationMatrix(t *testing.T) {
	db := testDB(t)
	Users := NewModels(db).Users
	seedUsers(t, Users)

	cases := []struct {
		name  string
		query *UserQuery
		want  []string
	}{
		{"ascending", Users.OrderByAge.Asc(), []string{"Bob", "Ada", "Grace"}},
		{"descending", Users.OrderByAge.Desc(), []string{"Grace", "Ada", "Bob"}},
		{"limit zero", Users.OrderByID.Asc().Limit(0), []string{}},
		{"limit", Users.OrderByID.Asc().Limit(2), []string{"Ada", "Bob"}},
		{"offset zero", Users.OrderByID.Asc().Offset(0), []string{"Ada", "Bob", "Grace"}},
		{"offset at size", Users.OrderByID.Asc().Offset(3), []string{}},
		{"offset past size", Users.OrderByID.Asc().Offset(10), []string{}},
		{"limit offset", Users.OrderByID.Asc().Limit(1).Offset(1), []string{"Bob"}},
		{"latest limit", Users.OrderByID.Asc().Limit(1).Limit(2), []string{"Ada", "Bob"}},
		{"latest offset", Users.OrderByID.Asc().Offset(2).Offset(1), []string{"Bob", "Grace"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := queryNames(t, tt.query); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("names = %v, want %v", got, tt.want)
			}
		})
	}

	for name, fn := range map[string]func(){
		"negative limit":  func() { Users.Limit(-1) },
		"negative offset": func() { Users.Offset(-1) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic")
				}
			}()
			fn()
		})
	}
}

func TestScopedDeleteBehavior(t *testing.T) {
	db := testDB(t)
	Users := NewModels(db).Users
	seedUsers(t, Users)
	ctx := context.Background()

	affected, err := Users.WhereActive.Eq(false).Delete(ctx)
	if err != nil || affected != 1 {
		t.Fatalf("filtered Delete = %d, %v", affected, err)
	}
	assertCount(t, Users, 2)

	affected, err = Users.WhereAge.In().Delete(ctx)
	if err != nil || affected != 0 {
		t.Fatalf("empty In Delete = %d, %v", affected, err)
	}
	for name, query := range map[string]*UserQuery{
		"order":  Users.WhereActive.Eq(true).OrderByID.Asc(),
		"limit":  Users.WhereActive.Eq(true).Limit(1),
		"offset": Users.WhereActive.Eq(true).Offset(1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := query.Delete(ctx); err == nil || !strings.Contains(err.Error(), "does not support") {
				t.Fatalf("unsafe Delete error = %v", err)
			}
		})
	}
	assertCount(t, Users, 2)
}

func TestPersistenceAllFieldsAndMissingRecords(t *testing.T) {
	db := testDB(t)
	Users := NewModels(db).Users
	ctx := context.Background()
	bio := "first"
	user, err := Users.Create(ctx, User{Name: "Ada", Email: "ada@example.com", Age: 36, Active: true, Score: 98.5, Bio: &bio})
	if err != nil || user.ID == 0 {
		t.Fatalf("Create = %#v, %v", user, err)
	}
	loaded, err := Users.Find(ctx, user.ID)
	if err != nil || !reflect.DeepEqual(loaded, user) {
		t.Fatalf("inserted fields = %#v, want %#v, err %v", loaded, user, err)
	}

	user.Name, user.Age, user.Active, user.Score, user.Bio = "Augusta", 37, false, 100, nil
	if err := Users.Update(ctx, user); err != nil {
		t.Fatal(err)
	}
	loaded, err = Users.Find(ctx, user.ID)
	if err != nil || !reflect.DeepEqual(loaded, user) {
		t.Fatalf("updated fields = %#v, want %#v, err %v", loaded, user, err)
	}

	missing := &User{ID: UserID(999), Name: "Missing"}
	if err := Users.Update(ctx, missing); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing Update error = %v", err)
	}
	before := *missing
	if err := Users.Reload(ctx, missing); !errors.Is(err, sql.ErrNoRows) || !reflect.DeepEqual(*missing, before) {
		t.Fatalf("failed Reload = %#v, %v", missing, err)
	}
	if err := Users.DeleteRecord(ctx, missing); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing DeleteRecord error = %v", err)
	}

	if err := Users.Insert(ctx, nil); err == nil {
		t.Fatal("nil Insert succeeded")
	}
	if err := Users.Update(ctx, nil); err == nil {
		t.Fatal("nil Update succeeded")
	}
	if err := Users.Reload(ctx, nil); err == nil {
		t.Fatal("nil Reload succeeded")
	}
	if err := Users.DeleteRecord(ctx, nil); err == nil {
		t.Fatal("nil DeleteRecord succeeded")
	}
}

func TestConstraintsContextsAndClosedDatabaseErrors(t *testing.T) {
	db := testDB(t)
	Users := NewModels(db).Users
	ctx := context.Background()
	insertTestUser(t, Users, "Ada", "ada@example.com", 37, true)
	if _, err := Users.Create(ctx, User{Name: "Other", Email: "ada@example.com"}); err == nil {
		t.Fatal("unique constraint violation succeeded")
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Users.All(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled All error = %v", err)
	}
	if _, err := Users.Count(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Count error = %v", err)
	}
	if _, err := Users.Exists(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Exists error = %v", err)
	}

	closed, err := openTestDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	ClosedUsers := NewModels(closed).Users
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ClosedUsers.All(ctx); err == nil {
		t.Fatal("query on closed database succeeded")
	}
}

func TestTransactionCommitRollbackAndNamespaces(t *testing.T) {
	db := testDB(t)
	Models := NewModels(db)
	ctx := context.Background()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	TxModels := Models.With(tx)
	if _, err := TxModels.Users.Create(ctx, User{Name: "Rolled", Email: "rolled@example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	assertCount(t, Models.Users, 0)
	if _, err := TxModels.Users.All(ctx); err == nil {
		t.Fatal("query after rollback succeeded")
	}

	tx, err = db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	TxModels = Models.With(tx)
	if _, err := TxModels.Users.Create(ctx, User{Name: "Committed", Email: "committed@example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	assertCount(t, Models.Users, 1)
}

func TestMetadataAndScannerMatrix(t *testing.T) {
	db := testDB(t)
	Users := NewModels(db).Users
	ctx := context.Background()
	bio := "notes"
	created, err := Users.Create(ctx, User{Name: "Ada", Email: "ada@example.com", Age: 37, Active: true, Score: 9.5, Bio: &bio})
	if err != nil {
		t.Fatal(err)
	}

	table := Tables.Users
	if table.Name != "users" || table.Columns != "id, name, email, age, active, score, bio, rank" {
		t.Fatalf("table metadata = %#v", table)
	}
	if table.InsertColumns != "name, email, age, active, score, bio, rank" || table.InsertPlaceholders != "?, ?, ?, ?, ?, ?, ?" {
		t.Fatalf("insert metadata = %#v", table)
	}
	if table.UpdateSet != "name = ?, email = ?, age = ?, active = ?, score = ?, bio = ?, rank = ?" {
		t.Fatalf("update metadata = %q", table.UpdateSet)
	}
	if twice := table.Qualified().Qualified(); twice.Col.ID != "users.id" {
		t.Fatalf("Qualified not idempotent: %q", twice.Col.ID)
	}
	if Tables.PostTags.Name != "post_tags" || Tables.PostTags.Col.PostID != "post_id" {
		t.Fatalf("underscore metadata = %#v", Tables.PostTags)
	}
	if got := Tables.Users.Col.ID.Val(UserID(7)); got != UserID(7) {
		t.Fatalf("typed Val = %#v", got)
	}

	dest := table.ScanDestinations(&User{})
	if len(dest) != 8 {
		t.Fatalf("ScanDestinations len = %d", len(dest))
	}
	row := db.QueryRowContext(ctx, "SELECT "+table.Columns+" FROM "+table.Name+" WHERE id = ?", created.ID)
	var loaded User
	if err := table.ScanRow(row, &loaded); err != nil || !reflect.DeepEqual(loaded, *created) {
		t.Fatalf("ScanRow = %#v, %v", loaded, err)
	}
	badRow := db.QueryRowContext(ctx, "SELECT name FROM users LIMIT 1")
	if err := table.ScanRow(badRow, &loaded); err == nil {
		t.Fatal("ScanRow accepted wrong projection")
	}

	rows, err := db.QueryContext(ctx, "SELECT "+table.Columns+" FROM users WHERE 1 = 0")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	empty, err := table.ScanRows(rows)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty ScanRows = %#v, %v", empty, err)
	}
}

func TestModelAndColumnNamingOverrides(t *testing.T) {
	db := testDB(t)
	Models := NewModels(db)
	ctx := context.Background()

	widget, err := Models.Widgets.Create(ctx, Widget{Label: "dial"})
	if err != nil {
		t.Fatal(err)
	}
	found, err := Models.Widgets.WhereLabel.Eq("dial").Find(ctx, widget.ID)
	if err != nil || found.Label != "dial" {
		t.Fatalf("conventional model = %#v, %v", found, err)
	}
	if Tables.Widgets.Name != "widgets" || Tables.Widgets.Col.Label != "display_label" {
		t.Fatalf("conventional/column metadata = %#v", Tables.Widgets)
	}

	entry, err := Models.AuditTrail.Create(ctx, AuditLog{Message: "created"})
	if err != nil || entry.ID == 0 {
		t.Fatalf("custom handle Create = %#v, %v", entry, err)
	}
	if Tables.AuditTrail.Name != "audit_logs" {
		t.Fatalf("custom handle metadata = %#v", Tables.AuditTrail)
	}
}

func TestNamedAndExtendedTypesRoundTrip(t *testing.T) {
	db := testDB(t)
	Agents := NewModels(db).Agents
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	seen := now.Add(-time.Hour)
	agent := Agent{
		Status:    AgentStatusReady,
		CreatedAt: now,
		SeenAt:    sql.NullTime{Time: seen, Valid: true},
		Nickname:  sql.NullString{String: "Ada", Valid: true},
		Payload:   []byte{1, 2, 3},
	}
	if err := Agents.Insert(ctx, &agent); err != nil {
		t.Fatal(err)
	}
	loaded, err := Agents.Find(ctx, agent.ID)
	if err != nil || loaded.Status != agent.Status || !loaded.CreatedAt.Equal(now) || !loaded.SeenAt.Time.Equal(seen) || !reflect.DeepEqual(loaded.Payload, agent.Payload) {
		t.Fatalf("extended round trip = %#v, %v", loaded, err)
	}
	if got, err := Agents.WhereCreatedAt.Gt(now.Add(-time.Minute)).Count(ctx); err != nil || got != 1 {
		t.Fatalf("time filter count = %d, %v", got, err)
	}
}

type stubResult struct {
	id, rows int64
	idErr    error
	rowsErr  error
}

func (r stubResult) LastInsertId() (int64, error) { return r.id, r.idErr }
func (r stubResult) RowsAffected() (int64, error) { return r.rows, r.rowsErr }

type stubDB struct {
	result sql.Result
	err    error
}

func (db stubDB) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return db.result, db.err
}
func (stubDB) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, fmt.Errorf("unexpected query")
}
func (stubDB) QueryRowContext(context.Context, string, ...any) *sql.Row { return nil }

func TestDriverResultErrorsPropagate(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("driver result failed")

	Users := NewModels(stubDB{result: stubResult{idErr: boom}}).Users
	if err := Users.Insert(ctx, &User{}); !errors.Is(err, boom) {
		t.Fatalf("LastInsertId error = %v", err)
	}
	Users = NewModels(stubDB{result: stubResult{rowsErr: boom}}).Users
	if err := Users.Update(ctx, &User{}); !errors.Is(err, boom) {
		t.Fatalf("RowsAffected update error = %v", err)
	}
	if err := Users.DeleteRecord(ctx, &User{}); !errors.Is(err, boom) {
		t.Fatalf("RowsAffected delete error = %v", err)
	}
	Users = NewModels(stubDB{err: boom}).Users
	if err := Users.Insert(ctx, &User{}); !errors.Is(err, boom) {
		t.Fatalf("Exec error = %v", err)
	}
}

func TestNilDatabasePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewModels(nil) did not panic")
		}
	}()
	NewModels(nil)
}

// These assignments are compile-time assertions for generated named-ID APIs.
func TestCompileTimeAPIShapes(t *testing.T) {
	db := testDB(t)
	Models := NewModels(db)
	var userQuery *UserQuery = Models.Users.WhereID.Eq(UserID(1))
	var value any = Tables.Users.Col.ID.Val(UserID(1))
	if userQuery == nil || value != UserID(1) {
		t.Fatal("generated typed API shape changed")
	}
}
