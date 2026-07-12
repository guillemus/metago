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

type recordingDB struct {
	query  string
	args   []any
	result sql.Result
	err    error
}

func (db *recordingDB) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	db.query = query
	db.args = append([]any(nil), args...)
	if db.result == nil {
		db.result = fixedResult(1)
	}
	return db.result, db.err
}

func (*recordingDB) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, errors.New("unexpected QueryContext call")
}

func (*recordingDB) QueryRowContext(context.Context, string, ...any) *sql.Row { return &sql.Row{} }

type fixedResult int64

func (result fixedResult) LastInsertId() (int64, error) { return 0, nil }
func (result fixedResult) RowsAffected() (int64, error) { return int64(result), nil }

type failingResult struct{ err error }

func (result failingResult) LastInsertId() (int64, error) { return 0, result.err }
func (result failingResult) RowsAffected() (int64, error) { return 0, result.err }

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

func TestBelongsToJoins(t *testing.T) {
	db := testDB(t)
	Models := NewModels(db)
	ctx := context.Background()

	ada := insertTestUser(t, Models.Users, "Ada", "ada@example.com", 37, true)
	bob := insertTestUser(t, Models.Users, "Bob", "bob@example.com", 29, true)
	core, err := Models.Teams.Create(ctx, Team{Name: "Core"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := Models.Teams.Create(ctx, Team{Name: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	for _, membership := range []Membership{
		{TeamID: core.ID, UserID: ada.ID, Role: "owner", Active: true},
		{TeamID: core.ID, UserID: bob.ID, Role: "member", Active: true},
		{TeamID: other.ID, UserID: ada.ID, Role: "member", Active: true},
	} {
		if _, err := Models.Memberships.Create(ctx, membership); err != nil {
			t.Fatal(err)
		}
	}

	query := Models.Memberships.
		JoinUser().
		JoinTeam().
		WhereUserName.Eq("Ada").
		WhereTeamName.Eq("Core")
	rows, err := query.All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].UserID != ada.ID || rows[0].TeamID != core.ID {
		t.Fatalf("joined memberships = %#v", rows)
	}

	// Repeating a join is harmless, and the original repository remains unscoped.
	count, err := Models.Memberships.JoinUser().JoinUser().WhereUserEmail.Eq("ada@example.com").Count(ctx)
	if err != nil || count != 2 {
		t.Fatalf("deduplicated join count = %d, %v", count, err)
	}
	if count, err := Models.Memberships.Count(ctx); err != nil || count != 3 {
		t.Fatalf("base membership count = %d, %v", count, err)
	}
}

func TestBelongsToJoinQueryOperations(t *testing.T) {
	db := testDB(t)
	Models := NewModels(db)
	ctx := context.Background()
	ada := insertTestUser(t, Models.Users, "Ada", "ada@example.com", 37, true)
	bob := insertTestUser(t, Models.Users, "Bob", "bob@example.com", 29, true)
	core, err := Models.Teams.Create(ctx, Team{Name: "Core"})
	if err != nil {
		t.Fatal(err)
	}
	for _, membership := range []Membership{
		{TeamID: core.ID, UserID: ada.ID, Role: "owner", Active: true},
		{TeamID: core.ID, UserID: bob.ID, Role: "member", Active: false},
	} {
		if _, err := Models.Memberships.Create(ctx, membership); err != nil {
			t.Fatal(err)
		}
	}

	joined := Models.Memberships.JoinUser().JoinTeam()
	first, err := joined.WhereUserName.Eq("Ada").First(ctx)
	if err != nil || first.UserID != ada.ID {
		t.Fatalf("joined First = %#v, %v", first, err)
	}
	found, err := joined.WhereTeamName.Eq("Core").Find(ctx, first.ID)
	if err != nil || found.ID != first.ID {
		t.Fatalf("joined Find = %#v, %v", found, err)
	}
	if exists, err := joined.WhereUserEmail.Eq("bob@example.com").Exists(ctx); err != nil || !exists {
		t.Fatalf("joined Exists = %v, %v", exists, err)
	}
	if count, err := joined.WhereTeamName.Eq("Missing").Count(ctx); err != nil || count != 0 {
		t.Fatalf("joined Count = %d, %v", count, err)
	}
}

func TestBelongsToJoinsQualifyAmbiguousColumns(t *testing.T) {
	db := testDB(t)
	Models := NewModels(db)
	ctx := context.Background()
	ada := insertTestUser(t, Models.Users, "Ada", "ada@example.com", 37, true)
	team, err := Models.Teams.Create(ctx, Team{Name: "Core"})
	if err != nil {
		t.Fatal(err)
	}
	membership, err := Models.Memberships.Create(ctx, Membership{TeamID: team.ID, UserID: ada.ID, Role: "owner", Active: true})
	if err != nil {
		t.Fatal(err)
	}

	rows, err := Models.Memberships.
		JoinUser().
		JoinTeam().
		WhereID.Eq(membership.ID).
		WhereUserName.Eq("Ada").
		WhereTeamName.Eq("Core").
		OrderByID.Desc().
		All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != membership.ID {
		t.Fatalf("qualified joined rows = %#v", rows)
	}
}

func TestBelongsToJoinCompositionIsImmutable(t *testing.T) {
	db := testDB(t)
	Models := NewModels(db)
	ctx := context.Background()
	ada := insertTestUser(t, Models.Users, "Ada", "ada@example.com", 37, true)
	bob := insertTestUser(t, Models.Users, "Bob", "bob@example.com", 29, true)
	team, err := Models.Teams.Create(ctx, Team{Name: "Core"})
	if err != nil {
		t.Fatal(err)
	}
	for _, membership := range []Membership{
		{TeamID: team.ID, UserID: ada.ID, Role: "owner", Active: true},
		{TeamID: team.ID, UserID: bob.ID, Role: "member", Active: false},
	} {
		if _, err := Models.Memberships.Create(ctx, membership); err != nil {
			t.Fatal(err)
		}
	}

	base := Models.Memberships.JoinUser()
	adaOnly := base.WhereUserName.Eq("Ada")
	activeOrBob := base.WhereActive.Eq(true).Or(base.WhereUserName.Eq("Bob"))
	if count, err := base.Count(ctx); err != nil || count != 2 {
		t.Fatalf("base joined count = %d, %v", count, err)
	}
	if count, err := adaOnly.Count(ctx); err != nil || count != 1 {
		t.Fatalf("derived joined count = %d, %v", count, err)
	}
	if count, err := activeOrBob.Count(ctx); err != nil || count != 2 {
		t.Fatalf("composed joined count = %d, %v", count, err)
	}
}

func TestGeneratedBelongsToAPIMatrix(t *testing.T) {
	db := testDB(t)
	Models := NewModels(db)

	// Compile-time coverage for every declared belongsTo relation and its
	// associated typed filters.
	var profile *ProfileQuery = Models.Profiles.JoinUser().WhereUserEmail.Eq("ada@example.com")
	var membershipUser *MembershipQuery = Models.Memberships.JoinUser().WhereUserAge.Gte(18)
	var membershipTeam *MembershipQuery = Models.Memberships.JoinTeam().WhereTeamName.Like("Core%")
	var projectTeam *ProjectQuery = Models.Projects.JoinTeam().WhereTeamName.Eq("Core")
	var projectOwner *ProjectQuery = Models.Projects.JoinOwner().WhereOwnerActive.Eq(true)
	var postProject *PostQuery = Models.Posts.JoinProject().WhereProjectName.Eq("Metago")
	var postUser *PostQuery = Models.Posts.JoinUser().WhereUserEmail.Eq("ada@example.com")
	var commentPost *CommentQuery = Models.Comments.JoinPost().WherePostTitle.Like("Intro%")
	var commentUser *CommentQuery = Models.Comments.JoinUser().WhereUserName.Eq("Ada")
	if profile == nil || membershipUser == nil || membershipTeam == nil || projectTeam == nil || projectOwner == nil || postProject == nil || postUser == nil || commentPost == nil || commentUser == nil {
		t.Fatal("generated belongsTo API returned nil")
	}
}

func TestBelongsToJoinStateAndAssociationNames(t *testing.T) {
	db := testDB(t)
	Models := NewModels(db)

	membership := Models.Memberships.JoinUser().JoinTeam().JoinUser()
	if got, want := membership.state.joinsSQL(), " JOIN users ON users.id = memberships.user_id JOIN teams ON teams.id = memberships.team_id"; got != want {
		t.Fatalf("joins SQL = %q, want %q", got, want)
	}
	if len(membership.state.joins) != 2 {
		t.Fatalf("deduplicated joins = %#v", membership.state.joins)
	}

	left := Models.Memberships.LeftJoinUser()
	if got, want := left.state.joinsSQL(), " LEFT OUTER JOIN users ON users.id = memberships.user_id"; got != want {
		t.Fatalf("left join SQL = %q, want %q", got, want)
	}

	// Association APIs use the foreign-key name, as Active Record does, rather
	// than the target model name. OwnerID therefore produces JoinOwner.
	owner := Models.Projects.JoinOwner()
	if got, want := owner.state.joinsSQL(), " JOIN users ON users.id = projects.owner_id"; got != want {
		t.Fatalf("named association SQL = %q, want %q", got, want)
	}

	// If scopes request different join kinds for one association, the receiver's
	// join wins instead of emitting the association twice.
	combined := Models.Memberships.LeftJoinUser().Or(Models.Memberships.JoinUser())
	if got, want := combined.state.joinsSQL(), " LEFT OUTER JOIN users ON users.id = memberships.user_id"; got != want {
		t.Fatalf("merged join kind = %q, want %q", got, want)
	}
}

func TestBelongsToJoinPaginationOrderingAndOperators(t *testing.T) {
	db := testDB(t)
	Models := NewModels(db)
	ctx := context.Background()
	bio := "engineer"
	for _, user := range []User{
		{Name: "Grace", Email: "grace@example.com", Age: 45, Bio: &bio},
		{Name: "Ada", Email: "ada@example.com", Age: 37},
		{Name: "Bob", Email: "bob@example.com", Age: 15},
	} {
		if _, err := Models.Users.Create(ctx, user); err != nil {
			t.Fatal(err)
		}
	}
	team, err := Models.Teams.Create(ctx, Team{Name: "Core"})
	if err != nil {
		t.Fatal(err)
	}
	users, err := Models.Users.OrderByID.Asc().All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range users {
		if _, err := Models.Projects.Create(ctx, Project{TeamID: team.ID, OwnerID: user.ID, Name: user.Name + " project"}); err != nil {
			t.Fatal(err)
		}
	}

	projects, err := Models.Projects.
		JoinOwner().
		WhereOwnerAge.Gte(18).
		OrderByOwnerName.Asc().
		Limit(1).
		Offset(1).
		All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Name != "Grace project" {
		t.Fatalf("joined pagination/order = %#v", projects)
	}
	if count, err := Models.Projects.JoinOwner().WhereOwnerName.Like("A%").Count(ctx); err != nil || count != 1 {
		t.Fatalf("joined Like count = %d, %v", count, err)
	}
	if count, err := Models.Projects.JoinOwner().WhereOwnerBio.IsNotNull().Count(ctx); err != nil || count != 1 {
		t.Fatalf("joined nullable count = %d, %v", count, err)
	}
	if count, err := Models.Projects.JoinOwner().WhereRaw("users.email = ?", "ada@example.com").Count(ctx); err != nil || count != 1 {
		t.Fatalf("joined WhereRaw count = %d, %v", count, err)
	}
}

func TestBelongsToJoinMergesAcrossAndOr(t *testing.T) {
	db := testDB(t)
	Models := NewModels(db)
	ctx := context.Background()
	ada := insertTestUser(t, Models.Users, "Ada", "ada@example.com", 37, true)
	bob := insertTestUser(t, Models.Users, "Bob", "bob@example.com", 29, true)
	core, err := Models.Teams.Create(ctx, Team{Name: "Core"})
	if err != nil {
		t.Fatal(err)
	}
	for _, membership := range []Membership{
		{TeamID: core.ID, UserID: ada.ID, Role: "owner"},
		{TeamID: core.ID, UserID: bob.ID, Role: "member"},
	} {
		if _, err := Models.Memberships.Create(ctx, membership); err != nil {
			t.Fatal(err)
		}
	}

	// The receiver has no join. Or must merge the join required by the other scope.
	query := Models.Memberships.WhereRole.Eq("owner").Or(
		Models.Memberships.JoinUser().WhereUserName.Eq("Bob"),
	)
	if count, err := query.Count(ctx); err != nil || count != 2 {
		t.Fatalf("Or merged joins count = %d, %v", count, err)
	}
	query = Models.Memberships.WhereTeamID.Eq(core.ID).And(
		Models.Memberships.JoinUser().WhereUserName.Eq("Ada"),
	)
	if count, err := query.Count(ctx); err != nil || count != 1 {
		t.Fatalf("And merged joins count = %d, %v", count, err)
	}
}

func TestBelongsToLeftJoinFindByTransactionAndDelete(t *testing.T) {
	db := testDB(t)
	Models := NewModels(db)
	ctx := context.Background()
	user := insertTestUser(t, Models.Users, "Ada", "ada@example.com", 37, true)
	if _, err := Models.Profiles.Create(ctx, Profile{UserID: user.ID, DisplayName: "Ada Lovelace"}); err != nil {
		t.Fatal(err)
	}

	profile, err := Models.Profiles.LeftJoinUser().WhereUserName.Eq("Ada").FindByUserID(ctx, user.ID)
	if err != nil || profile.DisplayName != "Ada Lovelace" {
		t.Fatalf("left join FindBy = %#v, %v", profile, err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	TxModels := Models.With(tx)
	if count, err := TxModels.Profiles.JoinUser().WhereUserEmail.Eq("ada@example.com").Count(ctx); err != nil || count != 1 {
		t.Fatalf("transaction join count = %d, %v", count, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	if _, err := Models.Profiles.JoinUser().WhereUserName.Eq("Ada").Delete(ctx); err == nil || !strings.Contains(err.Error(), "does not support joins") {
		t.Fatalf("joined Delete error = %v", err)
	}
	if count, err := Models.Profiles.Count(ctx); err != nil || count != 1 {
		t.Fatalf("joined Delete changed rows: %d, %v", count, err)
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

func TestGeneratedPartialUpdateSetterMatrix(t *testing.T) {
	tests := []struct {
		name    string
		table   any
		setters []string
	}{
		{"Users", Tables.Users, []string{"SetName", "SetEmail", "SetAge", "SetActive", "SetScore", "SetBio", "SetRank"}},
		{"Agents", Tables.Agents, []string{"SetStatus", "SetCreatedAt", "SetSeenAt", "SetNickname", "SetPayload"}},
		{"Widgets", Tables.Widgets, []string{"SetLabel"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ := reflect.TypeOf(tt.table)
			if _, exists := typ.FieldByName("SetID"); exists {
				t.Fatal("generated a primary-key setter")
			}
			for _, setter := range tt.setters {
				if _, exists := typ.FieldByName(setter); !exists {
					t.Fatalf("missing generated setter %s", setter)
				}
			}
		})
	}
}

func TestScopedPartialUpdateSQLAndArgumentOrder(t *testing.T) {
	db := &recordingDB{result: fixedResult(3)}
	Users := NewModels(db).Users
	ctx := context.Background()

	affected, err := Users.
		WhereName.Eq("Ada").
		WhereAge.Gte(18).
		UpdateColumns(ctx,
			Tables.Users.SetName("Augusta"),
			Tables.Users.SetAge.Expr("age + ?", 2),
			Tables.Users.SetBio.Null(),
		)
	if err != nil || affected != 3 {
		t.Fatalf("UpdateColumns = %d, %v", affected, err)
	}
	wantSQL := "UPDATE users SET name = ?, age = age + ?, bio = NULL WHERE (users.name = ? AND users.age >= ?)"
	if db.query != wantSQL {
		t.Fatalf("SQL = %q, want %q", db.query, wantSQL)
	}
	wantArgs := []any{"Augusta", 2, "Ada", 18}
	if !reflect.DeepEqual(db.args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", db.args, wantArgs)
	}

	_, err = Users.
		WhereName.Eq("Ada").
		Or(Users.WhereAge.Lt(18)).
		UpdateColumns(ctx, Tables.Users.SetActive(false))
	if err != nil {
		t.Fatal(err)
	}
	wantSQL = "UPDATE users SET active = ? WHERE (users.name = ? OR users.age < ?)"
	if db.query != wantSQL {
		t.Fatalf("composed SQL = %q, want %q", db.query, wantSQL)
	}
	wantArgs = []any{false, "Ada", 18}
	if !reflect.DeepEqual(db.args, wantArgs) {
		t.Fatalf("composed args = %#v, want %#v", db.args, wantArgs)
	}
}

func TestScopedPartialUpdateExpressionsAndColumnOverrides(t *testing.T) {
	db := &recordingDB{}
	ctx := context.Background()

	_, err := NewModels(db).Agents.WhereID.Eq(AgentID(7)).UpdateColumns(ctx,
		Tables.Agents.SetCreatedAt.CurrentTimestamp(),
		Tables.Agents.SetSeenAt.Null(),
	)
	if err != nil {
		t.Fatal(err)
	}
	wantSQL := "UPDATE agents SET created_at = CURRENT_TIMESTAMP, seen_at = NULL WHERE agents.id = ?"
	if db.query != wantSQL || !reflect.DeepEqual(db.args, []any{AgentID(7)}) {
		t.Fatalf("expression update = %q %#v, want %q %#v", db.query, db.args, wantSQL, []any{AgentID(7)})
	}

	_, err = NewModels(db).Widgets.WhereID.Eq(WidgetID(9)).UpdateColumns(ctx, Tables.Widgets.SetLabel("renamed"))
	if err != nil {
		t.Fatal(err)
	}
	wantSQL = "UPDATE widgets SET display_label = ? WHERE widgets.id = ?"
	if db.query != wantSQL || !reflect.DeepEqual(db.args, []any{"renamed", WidgetID(9)}) {
		t.Fatalf("overridden-column update = %q %#v", db.query, db.args)
	}
}

func TestScopedPartialUpdateBehavior(t *testing.T) {
	db := testDB(t)
	Models := NewModels(db)
	Users := Models.Users
	seedUsers(t, Users)
	ctx := context.Background()

	affected, err := Users.WhereName.Eq("Ada").UpdateColumns(ctx,
		Tables.Users.SetName("Augusta"),
		Tables.Users.SetAge.Expr("age + ?", 1),
		Tables.Users.SetBio.Null(),
	)
	if err != nil || affected != 1 {
		t.Fatalf("UpdateColumns = %d, %v", affected, err)
	}
	updated, err := Users.WhereName.Eq("Augusta").First(ctx)
	if err != nil || updated.Age != 38 || updated.Bio != nil {
		t.Fatalf("updated user = %#v, %v", updated, err)
	}

	affected, err = Users.WhereName.Eq("missing").UpdateColumns(ctx, Tables.Users.SetActive(false))
	if err != nil || affected != 0 {
		t.Fatalf("zero-row UpdateColumns = %d, %v", affected, err)
	}

	if _, err := Users.UpdateColumns(ctx, Tables.Users.SetActive(false)); err == nil || !strings.Contains(err.Error(), "refused unrestricted UPDATE") {
		t.Fatalf("unrestricted UpdateColumns error = %v", err)
	}
	if _, err := Users.WhereID.Eq(updated.ID).UpdateColumns(ctx); err == nil || !strings.Contains(err.Error(), "at least one assignment") {
		t.Fatalf("empty UpdateColumns error = %v", err)
	}
	if _, err := Users.WhereID.Eq(updated.ID).UpdateColumns(ctx,
		Tables.Users.SetName("first"),
		Tables.Users.SetName("second"),
	); err == nil || !strings.Contains(err.Error(), "more than once") {
		t.Fatalf("duplicate UpdateColumns error = %v", err)
	}
	if _, err := Models.Profiles.JoinUser().WhereID.Eq(1).UpdateColumns(ctx, Tables.Profiles.SetDisplayName("joined")); err == nil || !strings.Contains(err.Error(), "does not support joins") {
		t.Fatalf("joined UpdateColumns error = %v", err)
	}
	for name, query := range map[string]*UserQuery{
		"order":  Users.WhereActive.Eq(true).OrderByID.Asc(),
		"limit":  Users.WhereActive.Eq(true).Limit(1),
		"offset": Users.WhereActive.Eq(true).Offset(1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := query.UpdateColumns(ctx, Tables.Users.SetActive(false)); err == nil || !strings.Contains(err.Error(), "does not support") {
				t.Fatalf("unsafe UpdateColumns error = %v", err)
			}
		})
	}
}

func TestScopedPartialUpdateErrorsAndTransactions(t *testing.T) {
	ctx := context.Background()
	execErr := errors.New("exec failed")
	db := &recordingDB{err: execErr}
	if _, err := NewModels(db).Users.WhereID.Eq(1).UpdateColumns(ctx, Tables.Users.SetName("Ada")); !errors.Is(err, execErr) {
		t.Fatalf("execution error = %v", err)
	}

	rowsErr := errors.New("rows affected failed")
	db = &recordingDB{result: failingResult{err: rowsErr}}
	if _, err := NewModels(db).Users.WhereID.Eq(1).UpdateColumns(ctx, Tables.Users.SetName("Ada")); !errors.Is(err, rowsErr) {
		t.Fatalf("RowsAffected error = %v", err)
	}

	db = &recordingDB{}
	if _, err := NewModels(db).Users.WhereID.Eq(1).UpdateColumns(ctx, Tables.Users.SetName.Expr("")); err == nil || !strings.Contains(err.Error(), "invalid assignment") {
		t.Fatalf("empty expression error = %v", err)
	}
	if db.query != "" {
		t.Fatalf("invalid update executed SQL %q", db.query)
	}

	sqlDB := testDB(t)
	Models := NewModels(sqlDB)
	user := insertTestUser(t, Models.Users, "Ada", "ada@example.com", 37, true)
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	TxModels := Models.With(tx)
	if _, err := TxModels.Users.WhereID.Eq(user.ID).UpdateColumns(ctx, Tables.Users.SetName("transactional")); err != nil {
		t.Fatal(err)
	}
	inside, err := TxModels.Users.Find(ctx, user.ID)
	if err != nil || inside.Name != "transactional" {
		t.Fatalf("transactional value = %#v, %v", inside, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	outside, err := Models.Users.Find(ctx, user.ID)
	if err != nil || outside.Name != "Ada" {
		t.Fatalf("rolled-back value = %#v, %v", outside, err)
	}
}

func TestScopedPartialUpdateCurrentTimestamp(t *testing.T) {
	db := testDB(t)
	Agents := NewModels(db).Agents
	ctx := context.Background()
	agent, err := Agents.Create(ctx, Agent{Status: AgentStatusReady, CreatedAt: time.Unix(1, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}

	affected, err := Agents.WhereID.Eq(agent.ID).UpdateColumns(ctx, Tables.Agents.SetCreatedAt.CurrentTimestamp())
	if err != nil || affected != 1 {
		t.Fatalf("CurrentTimestamp update = %d, %v", affected, err)
	}
	updated, err := Agents.Find(ctx, agent.ID)
	if err != nil || !updated.CreatedAt.After(agent.CreatedAt) {
		t.Fatalf("timestamp update = %#v, %v", updated, err)
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
