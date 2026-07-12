package mapstruct

//go:generate go run ../.. .

type Address struct {
	City    string `mapstructure:"city"`
	Country string `mapstructure:"country"`
}

// All fields are required by default.
//
//mgo:gen std.mapstruct
type User struct {
	ID      string  `mapstructure:"id"`
	Name    string  `mapstructure:"display_name"`
	Address Address `mapstructure:"address"`
}

// With allowmissing, decoding performs a partial update. Individual fields can
// opt back into required validation with the required tag option.
//
//mgo:gen std.mapstruct allowmissing
type Preferences struct {
	Theme  string `mapstructure:"theme"`
	Locale string `mapstructure:"locale,required"`
}
