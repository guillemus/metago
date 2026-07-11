// SQL generation experiment: annotated types + metago templates + SQLite.
package main

import (
	"database/sql"
	"time"
)

//mgo:gen queries

//mgo:props model table=users
type User struct {
	ID        int64   //mgo:props sql pk auto filter sort
	Name      string  //mgo:props sql filter sort
	Email     string  //mgo:props sql unique filter
	Age       int     //mgo:props sql filter sort
	Active    bool    //mgo:props sql filter
	Score     float64 //mgo:props sql
	Bio       *string //mgo:props sql
	Transient string  // deliberately not persisted
}

//mgo:props model table=profiles
type Profile struct {
	ID          int64   //mgo:props sql pk auto filter sort
	UserID      int64   //mgo:props sql unique fk=users.id filter
	DisplayName string  //mgo:props sql filter sort
	AvatarURL   *string //mgo:props sql
}

//mgo:props model table=teams
type Team struct {
	ID          int64   //mgo:props sql pk auto filter sort
	Name        string  //mgo:props sql unique filter sort
	Description *string //mgo:props sql
}

//mgo:props model table=memberships
type Membership struct {
	ID     int64  //mgo:props sql pk auto filter sort
	TeamID int64  //mgo:props sql fk=teams.id filter sort
	UserID int64  //mgo:props sql fk=users.id filter sort
	Role   string //mgo:props sql filter
	Active bool   //mgo:props sql filter
}

//mgo:props model table=projects
type Project struct {
	ID       int64  //mgo:props sql pk auto filter sort
	TeamID   int64  //mgo:props sql fk=teams.id filter sort
	OwnerID  int64  //mgo:props sql fk=users.id filter
	Name     string //mgo:props sql filter sort
	Archived bool   //mgo:props sql filter
}

//mgo:props model table=posts
type Post struct {
	ID        int64  //mgo:props sql pk auto filter sort
	ProjectID int64  //mgo:props sql fk=projects.id filter sort
	UserID    int64  //mgo:props sql fk=users.id filter
	Title     string //mgo:props sql filter sort
	Body      string //mgo:props sql
	Published bool   //mgo:props sql filter
}

//mgo:props model table=comments
type Comment struct {
	ID       int64  //mgo:props sql pk auto filter sort
	PostID   int64  //mgo:props sql fk=posts.id filter sort
	UserID   int64  //mgo:props sql fk=users.id filter
	ParentID *int64 //mgo:props sql fk=comments.id filter
	Body     string //mgo:props sql
	Resolved bool   //mgo:props sql filter
}

//mgo:props model table=tags
type Tag struct {
	ID   int64  //mgo:props sql pk auto filter sort
	Name string //mgo:props sql unique filter sort
}

//mgo:props model table=post_tags
type PostTag struct {
	ID     int64 //mgo:props sql pk auto filter sort
	PostID int64 //mgo:props sql fk=posts.id filter sort
	TagID  int64 //mgo:props sql fk=tags.id filter sort
}

//mgo:props model table=activities
type Activity struct {
	ID        int64   //mgo:props sql pk auto filter sort
	UserID    int64   //mgo:props sql fk=users.id filter sort
	ProjectID *int64  //mgo:props sql fk=projects.id filter
	Kind      string  //mgo:props sql filter sort
	Payload   *string //mgo:props sql
	CreatedAt string  //mgo:props sql sort
}

type AgentStatus string

const AgentStatusReady AgentStatus = "ready"

// Agent exercises named scalar, time, nullable, and byte-slice SQL types.
//
//mgo:props model table=agents
type Agent struct {
	ID        int64          //mgo:props sql pk auto filter sort
	Status    AgentStatus    //mgo:props sql filter
	CreatedAt time.Time      //mgo:props sql filter sort
	SeenAt    sql.NullTime   //mgo:props sql
	Nickname  sql.NullString //mgo:props sql
	Payload   []byte         //mgo:props sql
}
