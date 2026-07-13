package sqlquery

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE sessions (
	token TEXT PRIMARY KEY,
	data BLOB NOT NULL,
	expiry TIMESTAMP NOT NULL
)`

func testConn(t *testing.T) *Conn {
	t.Helper()
	conn, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	conn.db.SetMaxOpenConns(1)
	if _, err := conn.db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	return conn
}

func TestExecAndOneScalar(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	result, err := conn.InsertSession(ctx, "active", []byte("payload"), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		t.Fatalf("RowsAffected() = %d, %v", affected, err)
	}

	data, err := conn.FindSession(ctx, "active", now)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, []byte("payload")) {
		t.Fatalf("FindSession() = %q", data)
	}

	if _, err := conn.FindSession(ctx, "active", now.Add(2*time.Hour)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("FindSession() error = %v, want sql.ErrNoRows", err)
	}
}

func TestOneStructuredRow(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	expiry := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	if _, err := conn.InsertSession(ctx, "one", []byte("first"), expiry); err != nil {
		t.Fatal(err)
	}

	session, err := conn.GetSession(ctx, "one")
	if err != nil {
		t.Fatal(err)
	}
	if session.Token != "one" || !bytes.Equal(session.Data, []byte("first")) || !session.Expiry.Equal(expiry) {
		t.Fatalf("GetSession() = %#v", session)
	}
}

func TestManyScalarAndStructuredRows(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	fixtures := []Session{
		{Token: "b", Data: []byte("second"), Expiry: now.Add(2 * time.Hour)},
		{Token: "a", Data: []byte("first"), Expiry: now.Add(time.Hour)},
		{Token: "expired", Data: []byte("old"), Expiry: now.Add(-time.Hour)},
	}
	for _, session := range fixtures {
		if _, err := conn.InsertSession(ctx, session.Token, session.Data, session.Expiry); err != nil {
			t.Fatal(err)
		}
	}

	tokens, err := conn.ListActiveSessionTokens(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 2 || tokens[0] != "a" || tokens[1] != "b" {
		t.Fatalf("ListActiveSessionTokens() = %#v", tokens)
	}

	sessions, err := conn.ListActiveSessions(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 || sessions[0].Token != "a" || sessions[1].Token != "b" {
		t.Fatalf("ListActiveSessions() = %#v", sessions)
	}
}

func TestExecDelete(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()
	if _, err := conn.InsertSession(ctx, "delete-me", []byte("data"), time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	result, err := conn.DeleteSession(ctx, "delete-me")
	if err != nil {
		t.Fatal(err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		t.Fatalf("RowsAffected() = %d, %v", affected, err)
	}
	if _, err := conn.GetSession(ctx, "delete-me"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetSession() error = %v, want sql.ErrNoRows", err)
	}
}
