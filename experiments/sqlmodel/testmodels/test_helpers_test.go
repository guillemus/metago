package models

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

const bootstrapSchema = `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	email TEXT NOT NULL UNIQUE,
	age INTEGER NOT NULL DEFAULT 0,
	active INTEGER NOT NULL DEFAULT 0,
	score REAL NOT NULL DEFAULT 0,
	bio TEXT,
	rank INTEGER
);

CREATE TABLE IF NOT EXISTS profiles (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL UNIQUE REFERENCES users(id),
	display_name TEXT NOT NULL,
	avatar_url TEXT
);

CREATE TABLE IF NOT EXISTS teams (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
		description TEXT
);

CREATE TABLE IF NOT EXISTS memberships (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	team_id INTEGER NOT NULL REFERENCES teams(id),
	user_id INTEGER NOT NULL REFERENCES users(id),
	role TEXT NOT NULL,
	active INTEGER NOT NULL DEFAULT 1,
	UNIQUE (team_id, user_id)
);

CREATE TABLE IF NOT EXISTS projects (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	team_id INTEGER NOT NULL REFERENCES teams(id),
	owner_id INTEGER NOT NULL REFERENCES users(id),
	name TEXT NOT NULL,
	archived INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS posts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL REFERENCES projects(id),
	user_id INTEGER NOT NULL REFERENCES users(id),
	title TEXT NOT NULL,
	body TEXT NOT NULL DEFAULT '',
	published INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS comments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	post_id INTEGER NOT NULL REFERENCES posts(id),
	user_id INTEGER NOT NULL REFERENCES users(id),
	parent_id INTEGER REFERENCES comments(id),
	body TEXT NOT NULL,
	resolved INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS tags (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS post_tags (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	post_id INTEGER NOT NULL REFERENCES posts(id),
	tag_id INTEGER NOT NULL REFERENCES tags(id),
	UNIQUE (post_id, tag_id)
);

CREATE TABLE IF NOT EXISTS activities (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL REFERENCES users(id),
	project_id INTEGER REFERENCES projects(id),
	kind TEXT NOT NULL,
	payload TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS widgets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	display_label TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	message TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS agents (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	status TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	seen_at TIMESTAMP,
	nickname TEXT,
	payload BLOB
);
`

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
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

func openTestDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000;`); err != nil {
		db.Close()
		return nil, err
	}
	return db, db.Ping()
}
