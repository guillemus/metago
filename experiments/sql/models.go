// SQL generation experiment: annotated types + metago templates + SQLite.
package main

//mgo:gen queries

//mgo:props model table=users
type User struct {
	ID     int64  //mgo:props sql pk auto filter sort
	Name   string //mgo:props sql filter sort
	Email  string //mgo:props sql unique filter
	Age    int    //mgo:props sql filter sort
	Active bool   //mgo:props sql filter
	Score  float64
	Bio    *string
	db     DBTX
}

//mgo:props model table=profiles
type Profile struct {
	ID          int64  //mgo:props sql pk auto filter sort
	UserID      int64  //mgo:props sql unique fk=users.id filter
	DisplayName string //mgo:props sql filter sort
	AvatarURL   *string
	db          DBTX
}

//mgo:props model table=teams
type Team struct {
	ID          int64  //mgo:props sql pk auto filter sort
	Name        string //mgo:props sql unique filter sort
	Description *string
	db          DBTX
}

//mgo:props model table=memberships
type Membership struct {
	ID     int64  //mgo:props sql pk auto filter sort
	TeamID int64  //mgo:props sql fk=teams.id filter sort
	UserID int64  //mgo:props sql fk=users.id filter sort
	Role   string //mgo:props sql filter
	Active bool   //mgo:props sql filter
	db     DBTX
}

//mgo:props model table=projects
type Project struct {
	ID       int64  //mgo:props sql pk auto filter sort
	TeamID   int64  //mgo:props sql fk=teams.id filter sort
	OwnerID  int64  //mgo:props sql fk=users.id filter
	Name     string //mgo:props sql filter sort
	Archived bool   //mgo:props sql filter
	db       DBTX
}

//mgo:props model table=posts
type Post struct {
	ID        int64  //mgo:props sql pk auto filter sort
	ProjectID int64  //mgo:props sql fk=projects.id filter sort
	UserID    int64  //mgo:props sql fk=users.id filter
	Title     string //mgo:props sql filter sort
	Body      string
	Published bool //mgo:props sql filter
	db        DBTX
}

//mgo:props model table=comments
type Comment struct {
	ID       int64  //mgo:props sql pk auto filter sort
	PostID   int64  //mgo:props sql fk=posts.id filter sort
	UserID   int64  //mgo:props sql fk=users.id filter
	ParentID *int64 //mgo:props sql fk=comments.id filter
	Body     string
	Resolved bool //mgo:props sql filter
	db       DBTX
}

//mgo:props model table=tags
type Tag struct {
	ID   int64  //mgo:props sql pk auto filter sort
	Name string //mgo:props sql unique filter sort
	db   DBTX
}

//mgo:props model table=post_tags
type PostTag struct {
	ID     int64 //mgo:props sql pk auto filter sort
	PostID int64 //mgo:props sql fk=posts.id filter sort
	TagID  int64 //mgo:props sql fk=tags.id filter sort
	db     DBTX
}

//mgo:props model table=activities
type Activity struct {
	ID        int64  //mgo:props sql pk auto filter sort
	UserID    int64  //mgo:props sql fk=users.id filter sort
	ProjectID *int64 //mgo:props sql fk=projects.id filter
	Kind      string //mgo:props sql filter sort
	Payload   *string
	CreatedAt string //mgo:props sql sort
	db        DBTX
}
