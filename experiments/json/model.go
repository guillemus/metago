// Package jsonexp is a metago experiment: a reflection-free JSON codec
// generated entirely from templates. The lexer runtime and the per-type
// codecs both live in the generated meta.go — no runtime dependency.
package jsonexp

//mgo:gen jsonruntime
//mgo:gen json
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

//mgo:gen json
type Address struct {
	Street string `json:"street"`
	City   string `json:"city"`
	Zip    string `json:"zip"`
}

//mgo:gen json
type Item struct {
	SKU   string  `json:"sku"`
	Qty   int     `json:"qty"`
	Price float64 `json:"price"`
}

//mgo:gen json
type Feed struct {
	Users []User `json:"users"`
}
