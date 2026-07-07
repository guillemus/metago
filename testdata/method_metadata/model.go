package fixture

//#methods Agent

type Agent struct{}

func (a Agent) Start(ctx string, force bool) error      { return nil }
func (a *Agent) Rename(names ...string) (string, error) { return "", nil }
