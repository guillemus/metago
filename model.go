package main

type Package struct {
	Name       string
	Dir        string
	ImportPath string
	Types      []*Type
	Functions  []Function
	Metas      []Meta
}

type Type struct {
	Name       string
	Kind       string
	Underlying string
	Fields     []Field
	Methods    []Method
	Values     []Value
	Props      map[string]Prop
	File       string
	Line       int
}

type Field struct {
	Name       string
	Type       string
	Underlying string
	TypeKind   string
	Tag        string
	Embedded   bool
	Props      map[string]Prop
	Line       int
}

type Method struct {
	Name         string
	Receiver     string
	ReceiverType string
	Params       []Param
	Results      []Param
	Body         string
	Props        map[string]Prop
	File         string
	Line         int
}

type Function struct {
	Name    string
	Params  []Param
	Results []Param
	Body    string
	Props   map[string]Prop
	File    string
	Line    int
}

// Prop is one //mgo:group attached to a symbol. Argv holds bare flags, Args holds key=value
// pairs. Multiple //mgo:lines for the same group on the same symbol merge: flags union,
// later keys win.
type Prop struct {
	Group string
	Args  map[string]string
	Argv  []string
}

type Param struct {
	Name     string
	Type     string
	Variadic bool
}

type Value struct {
	Name  string
	Type  string
	Value string
}

type Meta struct {
	Template string
	Target   string
	Args     map[string]string
	Argv     []string
	File     string
	Line     int
	Inline   bool
	EndLine  int
	// Anchored marks a directive written in the doc comment of a type, function, or method. The
	// target is that symbol, every token after the template name is an argument, and inline output
	// is inserted after the symbol (line AnchorEnd) instead of after the directive.
	Anchored  bool
	AnchorEnd int
}

type Invocation struct {
	Package    *Package
	Meta       Meta
	Type       *Type
	Method     *Method
	Function   *Function
	Name       string
	Kind       string
	TypeName   string
	Args       map[string]string
	Argv       []string
	Fields     []Field
	Methods    []Method
	Functions  []Function
	Params     []Param
	Results    []Param
	Body       string
	Values     []Value
	IsType     bool
	IsMethod   bool
	IsFunction bool
}
