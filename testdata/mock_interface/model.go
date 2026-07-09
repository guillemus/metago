package fixture

//mgo:gen mock Store

type Store interface {
	Get(id string) (User, error)
	Save(user User) error
	List(limit int, tags ...string) ([]User, error)
}

type User struct {
	ID   string
	Name string
}
