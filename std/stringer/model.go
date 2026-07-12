package stringer

//go:generate go run ../.. .

//mgo:gen std.stringer trimprefix=Priority
type Priority int

const (
	PriorityLow Priority = iota + 1
	PriorityNormal
	PriorityHigh
)

//mgo:gen std.stringer trimprefix=Direction
type Direction int16

const (
	DirectionUnknown Direction = -1
	DirectionNorth   Direction = 10
	DirectionSouth   Direction = 42 // Sparse and out of declaration order.
)

//mgo:gen std.stringer trimprefix=Permission
type Permission uint64

const (
	PermissionRead  Permission = 1 << iota
	PermissionWrite
	PermissionAdmin Permission = 1 << 63
)

//mgo:gen std.stringer trimprefix=State
type State string

const (
	StateReady State = "ready"
	StateBusy  State = "busy"
)

//mgo:gen std.stringer trimprefix=Enabled
type Enabled bool

const (
	EnabledNo  Enabled = false
	EnabledYes Enabled = true
)

//mgo:gen std.stringer trimprefix=Ratio
type Ratio float64

const (
	RatioHalf Ratio = 0.5
	RatioFull Ratio = 1
)

//mgo:gen std.stringer trimprefix=Phase
type Phase complex128

const (
	PhaseReal      Phase = 1
	PhaseImaginary Phase = 1i
)
