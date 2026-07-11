package models

import (
	"context"
	"testing"
)

// TestApplicationModelFlow is intentionally a small consumer-level smoke test.
// Generator behavior belongs to x/activerecord/testmodels.
func TestApplicationModelFlow(t *testing.T) {
	db := testDB(t)
	models := NewModels(db)
	ctx := context.Background()

	user, err := models.Users.Create(ctx, User{
		Name: "Ada", Email: "ada@example.com", Age: 37, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	team, err := models.Teams.Create(ctx, Team{Name: "Core"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := models.Memberships.Create(ctx, Membership{
		TeamID: team.ID, UserID: user.ID, Role: "owner", Active: true,
	}); err != nil {
		t.Fatal(err)
	}

	memberships, err := models.Memberships.
		JoinUser().
		JoinTeam().
		WhereUserName.Eq("Ada").
		WhereTeamName.Eq("Core").
		All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(memberships) != 1 || memberships[0].Role != "owner" {
		t.Fatalf("memberships = %#v", memberships)
	}
}
