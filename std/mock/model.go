package mock

//go:generate go run ../.. .

type User struct {
	ID string
}

//mgo:gen std.mock
type Store interface {
	Get(id string) (User, error)
	Save(user User) error
}
