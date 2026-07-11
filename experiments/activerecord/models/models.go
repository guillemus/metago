// SQL generation experiment: annotated types + metago templates + SQLite.
package models

import (
	"database/sql"
	"time"
)

//mgo:gen queries

type UserID int64

//mgo:model table=users
type User struct {
	ID     UserID  //mgo:sql pk auto filter sort
	Name   string  //mgo:sql filter sort
	Email  string  //mgo:sql unique filter
	Age    int     //mgo:sql filter sort
	Active bool    //mgo:sql filter
	Score  float64 //mgo:sql filter sort
	Bio    *string //mgo:sql filter
	Rank   *int    //mgo:sql filter sort
}

type ProfileID int64

//mgo:model table=profiles
type Profile struct {
	ID          ProfileID //mgo:sql pk auto filter sort
	UserID      UserID    //mgo:sql unique filter
	DisplayName string    //mgo:sql filter sort
	AvatarURL   *string
}

type TeamID int64

//mgo:model table=teams
type Team struct {
	ID          TeamID //mgo:sql pk auto filter sort
	Name        string //mgo:sql unique filter sort
	Description *string
}

type MembershipID int64

//mgo:model table=memberships
type Membership struct {
	ID     MembershipID //mgo:sql pk auto filter sort
	TeamID TeamID       //mgo:sql filter sort
	UserID UserID       //mgo:sql filter sort
	Role   string       //mgo:sql filter
	Active bool         //mgo:sql filter
}

type ProjectID int64

//mgo:model table=projects
type Project struct {
	ID       ProjectID //mgo:sql pk auto filter sort
	TeamID   TeamID    //mgo:sql filter sort
	OwnerID  UserID    //mgo:sql filter
	Name     string    //mgo:sql filter sort
	Archived bool      //mgo:sql filter
}

type PostID int64

//mgo:model table=posts
type Post struct {
	ID        PostID    //mgo:sql pk auto filter sort
	ProjectID ProjectID //mgo:sql filter sort
	UserID    UserID    //mgo:sql filter
	Title     string    //mgo:sql filter sort
	Body      string
	Published bool //mgo:sql filter
}

type CommentID int64

//mgo:model table=comments
type Comment struct {
	ID       CommentID  //mgo:sql pk auto filter sort
	PostID   PostID     //mgo:sql filter sort
	UserID   UserID     //mgo:sql filter
	ParentID *CommentID //mgo:sql filter
	Body     string
	Resolved bool //mgo:sql filter
}

type TagID int64

//mgo:model table=tags
type Tag struct {
	ID   TagID  //mgo:sql pk auto filter sort
	Name string //mgo:sql unique filter sort
}

type PostTagID int64

//mgo:model table=post_tags
type PostTag struct {
	ID     PostTagID //mgo:sql pk auto filter sort
	PostID PostID    //mgo:sql filter sort
	TagID  TagID     //mgo:sql filter sort
}

type ActivityID int64

//mgo:model table=activities
type Activity struct {
	ID        ActivityID //mgo:sql pk auto filter sort
	UserID    UserID     //mgo:sql filter sort
	ProjectID *ProjectID //mgo:sql filter
	Kind      string     //mgo:sql filter sort
	Payload   *string
	CreatedAt string //mgo:sql sort
}

type WidgetID int64

// Widget exercises conventional table naming and a column override.
//
//mgo:model
type Widget struct {
	ID    WidgetID //mgo:sql pk auto filter sort
	Label string   //mgo:sql column=display_label filter sort
}

type AuditLogID int64

// AuditLog exercises an overridden repository handle.
//
//mgo:model table=audit_logs handle=AuditTrail
type AuditLog struct {
	ID      AuditLogID //mgo:sql pk auto filter sort
	Message string     //mgo:sql filter
}

type AgentID int64
type AgentStatus string

const AgentStatusReady AgentStatus = "ready"

// Agent exercises named scalar, time, nullable, and byte-slice SQL types.
//
//mgo:model table=agents
type Agent struct {
	ID        AgentID     //mgo:sql pk auto filter sort
	Status    AgentStatus //mgo:sql filter
	CreatedAt time.Time   //mgo:sql filter sort
	SeenAt    sql.NullTime
	Nickname  sql.NullString
	Payload   []byte
}
