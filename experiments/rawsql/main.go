package main

import (
	"context"
	"fmt"
	"log"
)

const bootstrapSchema = `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	email TEXT NOT NULL UNIQUE,
	age INTEGER NOT NULL DEFAULT 0,
	active INTEGER NOT NULL DEFAULT 0,
	score REAL NOT NULL DEFAULT 0,
	bio TEXT
);
`

func main() {
	path := "test.db"

	db, err := Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(bootstrapSchema); err != nil {
		log.Fatalf("schema: %v", err)
	}

	ctx := context.Background()

	u := &User{
		Name:   "Ada",
		Email:  "ada@example.com",
		Age:    36,
		Active: true,
	}

	result, err := db.ExecContext(ctx, `
		INSERT INTO `+Users.Table+` (`+Users.InsertColumns+`)
		VALUES (`+Users.InsertPlaceholders+`)
	`, UserInsertArgs(u)...)
	if err != nil {
		log.Fatalf("insert: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		log.Fatalf("last insert id: %v", err)
	}
	u.ID = id
	fmt.Printf("created user id=%d name=%s\n", u.ID, u.Name)

	u.Age = 37
	if _, err := db.ExecContext(ctx, `
		UPDATE `+Users.Table+`
		SET `+Users.UpdateSet+`
		WHERE `+Users.ID+` = ?
	`, UserUpdateArgs(u)...); err != nil {
		log.Fatalf("update: %v", err)
	}

	rows, err := db.QueryContext(ctx, `
		SELECT `+Users.Columns+`
		FROM `+Users.Table+`
		WHERE `+Users.Age+` >= ?
		  AND `+Users.Active+` = ?
		ORDER by `+Users.Name+` ASC
	`, 18, true)
	if err != nil {
		log.Fatalf("list: %v", err)
	}
	defer rows.Close()

	list, err := ScanUsers(rows)
	if err != nil {
		log.Fatalf("scan: %v", err)
	}
	fmt.Printf("adults: %d\n", len(list))

	if _, err := db.ExecContext(ctx, `
		DELETE FROM `+Users.Table+`
		WHERE `+Users.ID+` = ?
	`, u.ID); err != nil {
		log.Fatalf("delete: %v", err)
	}
}
