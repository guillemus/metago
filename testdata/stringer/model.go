package fixture

// Consecutive iota values, prefix trimming, and an alias.
//
//mgo:gen stringer trimprefix=Pill
type Pill int

const (
	PillPlacebo Pill = iota
	PillAspirin
	PillIbuprofen
	PillParacetamol
	PillAcetaminophen Pill = PillParacetamol
)

// Negative, computed, out-of-order, and sparse values.
//
//mgo:gen stringer trimprefix=Direction
type Direction int

const (
	DirectionSouth  Direction = 10
	DirectionNorth  Direction = -1
	DirectionCenter Direction = 2 + 3
	DirectionFar    Direction = 1 << 10
)

// Unsigned values and bit patterns.
//
//mgo:gen stringer trimprefix=Permission
type Permission uint64

const (
	PermissionRead Permission = 1 << iota
	PermissionWrite
	PermissionAdmin Permission = 1 << 63
)

// Multiple names in one declaration.
//
//mgo:gen stringer trimprefix=Pair
type Pair int

const PairLeft, PairRight Pair = -1, 1

// With no trimprefix argument, the full constant name is retained.
//
//mgo:gen stringer
type Mode int

const (
	ModeOff Mode = iota
	ModeOn
)
