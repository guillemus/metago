package sqlquery

import (
	"database/sql"
	"time"
)

type SessionToken string

type Session struct {
	Token  SessionToken `db:"token"`
	Data   []byte       `db:"data"`
	Expiry time.Time    `db:"expiry"`
}

type Conn struct {
	db *sql.DB
}

func Open(path string) (*Conn, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &Conn{db: db}, nil
}

func (c *Conn) Close() error { return c.db.Close() }
