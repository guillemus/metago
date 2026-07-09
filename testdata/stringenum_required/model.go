package fixture

//mgo:gen stringenum AgentStatus created,installed,running,offline,deleted,paused,resuming

type AgentStatus string

//mgo:gen required Agent

type Agent struct {
	ID     int
	Name   string
	Status AgentStatus
	Token  *string
	Labels []string
	Note   string `ignore:"true"`
}
