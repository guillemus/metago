package enum

//go:generate go run ../.. .

//mgo:gen std.enum
type ArticleStatus string

const (
	ArticleStatusDraft     ArticleStatus = "draft"
	ArticleStatusPublished ArticleStatus = "published"
	ArticleStatusArchived  ArticleStatus = "archived"
)

//mgo:gen std.enum
type Priority int16

const (
	PriorityLow    Priority = -10
	PriorityNormal Priority = 0
	PriorityHigh   Priority = 42
)

//mgo:gen std.enum
type Permission uint64

const (
	PermissionRead  Permission = 1
	PermissionWrite Permission = 2
	PermissionAdmin Permission = 1 << 63
)

//mgo:gen std.enum
type Ratio float32

const (
	RatioHalf Ratio = 0.5
	RatioFull Ratio = 1
)

//mgo:gen std.enum
type Scale float64

const (
	ScaleSmall Scale = 0.125
	ScaleLarge Scale = 100.5
)
