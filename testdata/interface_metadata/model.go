package fixture

//mgo:gen interfaceinfo Store

type Store interface {
	Get(id string) (User, error)
	List(limit int, tags ...string) []User
}

type User struct{}
