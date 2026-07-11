package main

import (
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

func Open(path string) (*sqlx.DB, error) {
	db, err := sqlx.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
