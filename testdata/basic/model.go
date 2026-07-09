package fixture

//mgo:gen stringer Status
//mgo:gen enum Status

type Status string

const (
	Active   Status = "active"
	Disabled Status = "disabled"
)

//mgo:gen stringer Code

type Code int

//mgo:gen fields User
//mgo:gen summary User table=users

type User struct {
	ID     int `json:"id,omitempty"`
	Name   string
	Status Status
}

func (u User) Touch() {}
