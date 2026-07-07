package fixture

//#stringenum AgentStatus created,installed,running,offline,deleted,paused,resuming

type AgentStatus string

//#required Agent

type Agent struct {
	ID     int
	Name   string
	Status AgentStatus
	Token  *string
	Labels []string
	Note   string `ignore:"true"`
}
