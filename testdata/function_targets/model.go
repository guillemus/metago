package fixture

//#wrap BuildUser
//#wrap Server.Serve

type Server struct{}

type User struct {
	Name string
}

func BuildUser(name string) (User, error) {
	return User{Name: name}, nil
}

func (s Server) Serve(path string) error {
	return nil
}
