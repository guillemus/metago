package enum

//go:generate go run ../.. .

//mgo:gen std.enum draft,published,archived
type ArticleStatus string
