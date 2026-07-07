package fixture

//#stringer Status
//#enum Status
type Status string

const (
	Active   Status = "active"
	Disabled Status = "disabled"
)

//#stringer Code
type Code int

//#fields User
//#summary User table=users
type User struct {
	ID     int `json:"id"`
	Name   string
	Status Status
}

func (u User) Touch() {}
