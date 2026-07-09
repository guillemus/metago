package fixture

//mgo:gen wrap User

type User struct{}

func (u User) Save() error {
	if false {
		return nil
	}
	return nil
}

func Helper(name string) string {
	return "hello " + name
}
