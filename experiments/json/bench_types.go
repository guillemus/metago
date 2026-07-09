package jsonexp

// Mirror types with identical shapes and tags, but no generated methods, so
// encoding/json and the drop-in competitors decode with their own machinery
// instead of calling our UnmarshalJSON.

type StdUser struct {
	ID       int64             `json:"id"`
	Name     string            `json:"name"`
	Email    string            `json:"email"`
	Age      int               `json:"age"`
	Active   bool              `json:"active"`
	Score    float64           `json:"score"`
	Tags     []string          `json:"tags"`
	Address  StdAddress        `json:"address"`
	Items    []StdItem         `json:"items"`
	Metadata map[string]string `json:"metadata"`
}

type StdAddress struct {
	Street string `json:"street"`
	City   string `json:"city"`
	Zip    string `json:"zip"`
}

type StdItem struct {
	SKU   string  `json:"sku"`
	Qty   int     `json:"qty"`
	Price float64 `json:"price"`
}

type StdFeed struct {
	Users []StdUser `json:"users"`
}
