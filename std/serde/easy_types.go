package serde

// Mirror types for the easyjson competitor. easyjson generates its codecs
// into easy_types_easyjson.go via:
//
//	go run github.com/mailru/easyjson/easyjson -all easy_types.go

//easyjson:json
type EasyUser struct {
	ID       int64             `json:"id"`
	Name     string            `json:"name"`
	Email    string            `json:"email"`
	Age      int               `json:"age"`
	Active   bool              `json:"active"`
	Score    float64           `json:"score"`
	Tags     []string          `json:"tags"`
	Address  EasyAddress       `json:"address"`
	Items    []EasyItem        `json:"items"`
	Metadata map[string]string `json:"metadata"`
}

//easyjson:json
type EasyAddress struct {
	Street string `json:"street"`
	City   string `json:"city"`
	Zip    string `json:"zip"`
}

//easyjson:json
type EasyItem struct {
	SKU   string  `json:"sku"`
	Qty   int     `json:"qty"`
	Price float64 `json:"price"`
}

//easyjson:json
type EasyFeed struct {
	Users []EasyUser `json:"users"`
}
