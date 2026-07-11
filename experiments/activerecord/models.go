// SQL generation experiment: annotated types + metago templates + SQLite.
package main

import (
	"database/sql"
	"time"
)

//mgo:gen queries

type UserID int64

//mgo:props model table=users
type User struct {
	ID     UserID //mgo:props sql pk auto filter sort
	Name   string //mgo:props sql filter sort
	Email  string //mgo:props sql unique filter
	Age    int    //mgo:props sql filter sort
	Active bool   //mgo:props sql filter
	Score  float64
	Bio    *string
}

type ProfileID int64

//mgo:props model table=profiles
type Profile struct {
	ID          ProfileID //mgo:props sql pk auto filter sort
	UserID      UserID    //mgo:props sql unique filter
	DisplayName string    //mgo:props sql filter sort
	AvatarURL   *string
}

type TeamID int64

//mgo:props model table=teams
type Team struct {
	ID          TeamID //mgo:props sql pk auto filter sort
	Name        string //mgo:props sql unique filter sort
	Description *string
}

type MembershipID int64

//mgo:props model table=memberships
type Membership struct {
	ID     MembershipID //mgo:props sql pk auto filter sort
	TeamID TeamID       //mgo:props sql filter sort
	UserID UserID       //mgo:props sql filter sort
	Role   string       //mgo:props sql filter
	Active bool         //mgo:props sql filter
}

type ProjectID int64

//mgo:props model table=projects
type Project struct {
	ID       ProjectID //mgo:props sql pk auto filter sort
	TeamID   TeamID    //mgo:props sql filter sort
	OwnerID  UserID    //mgo:props sql filter
	Name     string    //mgo:props sql filter sort
	Archived bool      //mgo:props sql filter
}

type PostID int64

//mgo:props model table=posts
type Post struct {
	ID        PostID    //mgo:props sql pk auto filter sort
	ProjectID ProjectID //mgo:props sql filter sort
	UserID    UserID    //mgo:props sql filter
	Title     string    //mgo:props sql filter sort
	Body      string
	Published bool //mgo:props sql filter
}

type CommentID int64

//mgo:props model table=comments
type Comment struct {
	ID       CommentID  //mgo:props sql pk auto filter sort
	PostID   PostID     //mgo:props sql filter sort
	UserID   UserID     //mgo:props sql filter
	ParentID *CommentID //mgo:props sql filter
	Body     string
	Resolved bool //mgo:props sql filter
}

type TagID int64

//mgo:props model table=tags
type Tag struct {
	ID   TagID  //mgo:props sql pk auto filter sort
	Name string //mgo:props sql unique filter sort
}

type PostTagID int64

//mgo:props model table=post_tags
type PostTag struct {
	ID     PostTagID //mgo:props sql pk auto filter sort
	PostID PostID    //mgo:props sql filter sort
	TagID  TagID     //mgo:props sql filter sort
}

type ActivityID int64

//mgo:props model table=activities
type Activity struct {
	ID        ActivityID //mgo:props sql pk auto filter sort
	UserID    UserID     //mgo:props sql filter sort
	ProjectID *ProjectID //mgo:props sql filter
	Kind      string     //mgo:props sql filter sort
	Payload   *string
	CreatedAt string //mgo:props sql sort
}

type AgentID int64
type AgentStatus string

const AgentStatusReady AgentStatus = "ready"

// Agent exercises named scalar, time, nullable, and byte-slice SQL types.
//
//mgo:props model table=agents
type Agent struct {
	ID        AgentID     //mgo:props sql pk auto filter sort
	Status    AgentStatus //mgo:props sql filter
	CreatedAt time.Time   //mgo:props sql filter sort
	SeenAt    sql.NullTime
	Nickname  sql.NullString
	Payload   []byte
}
