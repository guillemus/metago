package enum

//go:generate go run ../.. .

//mgo:gen enum draft,published,archived
type ArticleStatus string
