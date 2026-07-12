// Package serde demonstrates Serde, a reflection-free JSON coder-decoder
// generated entirely from standard metago templates. The lexer runtime and
// per-type codecs both live in generated meta.go — no runtime dependency.
package serde

//mgo:gen std.serderuntime
//mgo:gen std.serde
type User struct {
	ID       int64             `json:"id"`
	Name     string            `json:"name"`
	Email    string            `json:"email"`
	Age      int               `json:"age"`
	Active   bool              `json:"active"`
	Score    float64           `json:"score"`
	Tags     []string          `json:"tags"`
	Address  Address           `json:"address"`
	Items    []Item            `json:"items"`
	Metadata map[string]string `json:"metadata"`
}

//mgo:gen std.serde
type Address struct {
	Street string `json:"street"`
	City   string `json:"city"`
	Zip    string `json:"zip"`
}

//mgo:gen std.serde
type Item struct {
	SKU   string  `json:"sku"`
	Qty   int     `json:"qty"`
	Price float64 `json:"price"`
}

//mgo:gen std.serde
type Feed struct {
	Users []User `json:"users"`
}
