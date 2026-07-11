// sqlx + Metago experiment: struct tags drive raw-SQL table metadata.
package main

//mgo:gen tables

type UserID int64

type User struct {
	ID     UserID  `db:"id"`
	Name   string  `db:"name"`
	Email  string  `db:"email"`
	Age    int     `db:"age"`
	Active bool    `db:"active"`
	Score  float64 `db:"score"`
	Bio    *string `db:"bio"`
}

type ProfileID int64

type Profile struct {
	ID          ProfileID `db:"id"`
	UserID      UserID    `db:"user_id"`
	DisplayName string    `db:"display_name"`
	AvatarURL   *string   `db:"avatar_url"`
}

type TeamID int64

type Team struct {
	ID          TeamID  `db:"id"`
	Name        string  `db:"name"`
	Description *string `db:"description"`
}
