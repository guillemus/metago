package server

type Server struct {
	Addr string
}

func NewServer(addr string) Server {
	return Server{Addr: addr}
}

func (s Server) Serve(path string) error {
	return nil
}
