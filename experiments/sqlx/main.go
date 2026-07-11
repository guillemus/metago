package main

import (
	"context"
	"fmt"
	"log"
)

const bootstrapSchema = `
CREATE TABLE users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	email TEXT NOT NULL UNIQUE,
	age INTEGER NOT NULL DEFAULT 0,
	active INTEGER NOT NULL DEFAULT 0,
	score REAL NOT NULL DEFAULT 0,
	bio TEXT
);
CREATE TABLE profiles (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL UNIQUE REFERENCES users(id),
	display_name TEXT NOT NULL,
	avatar_url TEXT
);
CREATE TABLE teams (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	description TEXT
);
`

func main() {
	db, err := Open(":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(bootstrapSchema); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	u := Tables.Users
	user := User{Name: "Ada", Email: "ada@example.com", Age: 37, Active: true}
	result, err := db.NamedExecContext(ctx, `
		INSERT INTO `+u.Name+` (`+u.InsertColumns+`)
		VALUES (`+u.InsertValues+`)
	`, &user)
	if err != nil {
		log.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		log.Fatal(err)
	}
	user.ID = UserID(id)
	fmt.Printf("created user %d: %s\n", user.ID, user.Name)
}
