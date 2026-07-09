package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"text/template"
)

func TestUtilityName(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"invocation", Invocation{Name: "User"}, "User"},
		{"invocation pointer", &Invocation{Name: "User"}, "User"},
		{"type", Type{Name: "User"}, "User"},
		{"type pointer", &Type{Name: "User"}, "User"},
		{"field", Field{Name: "ID"}, "ID"},
		{"field pointer", &Field{Name: "ID"}, "ID"},
		{"method", Method{Name: "Touch"}, "Touch"},
		{"method pointer", &Method{Name: "Touch"}, "Touch"},
		{"value", Value{Name: "Active"}, "Active"},
		{"value pointer", &Value{Name: "Active"}, "Active"},
		{"string", "Raw", "Raw"},
		{"nil", nil, ""},
		{"nil type pointer", (*Type)(nil), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nameOf(tc.in); got != tc.want {
				t.Fatalf("nameOf(%#v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestUtilityTypeof(t *testing.T) {
	typ := &Type{Name: "Status", Underlying: "string"}
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"invocation", Invocation{Type: typ, TypeName: "Status"}, "string"},
		{"invocation fallback", Invocation{TypeName: "Status"}, "Status"},
		{"invocation pointer", &Invocation{Type: typ}, "string"},
		{"type", Type{Underlying: "int"}, "int"},
		{"type pointer", &Type{Underlying: "bool"}, "bool"},
		{"field", Field{Type: "[]string"}, "[]string"},
		{"field pointer", &Field{Type: "map[string]int"}, "map[string]int"},
		{"value", Value{Type: "Status"}, "Status"},
		{"value pointer", &Value{Type: "Status"}, "Status"},
		{"string", "*User", "*User"},
		{"nil", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := typeOf(tc.in); got != tc.want {
				t.Fatalf("typeOf(%#v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTemplateFuncRegistry(t *testing.T) {
	funcs := templateFuncs(func(string, ...string) string { return "" }, nil)
	want := []string{
		"name", "typeof", "imports", "keys", "fieldNames", "methodNames", "join", "lower", "upper", "contains", "hasPrefix", "hasSuffix", "trimPrefix", "trimSuffix", "replace", "split", "exported", "unexported", "quote", "snake", "kebab", "camel", "pascal", "initial", "receiver", "tag", "tagName", "tagOpts", "tagHas", "tagExists", "prop", "props", "propHas", "propExists", "fieldsWithTag", "fieldsWithoutTag", "exportedFields", "unexportedFields", "embeddedFields", "nonEmbeddedFields", "isString", "isInt", "isBool", "isFloat", "isSlice", "isMap", "isPointer", "elem", "zero", "dict", "list", "get", "arg", "default",
	}
	for _, name := range want {
		if funcs[name] == nil {
			t.Fatalf("template func %q is not registered", name)
		}
	}
	if funcs["typeOf"] != nil || funcs["taghas"] != nil || funcs["tagexists"] != nil {
		t.Fatal("old template helper aliases should not be registered")
	}
}

func TestUtilityTypeofIsOnlyLowercaseInTemplates(t *testing.T) {
	if _, err := template.New("ok").Funcs(templateFuncs(func(string, ...string) string { return "" }, nil)).Parse(`{{ typeof . }}`); err != nil {
		t.Fatal(err)
	}
	if _, err := template.New("bad").Funcs(templateFuncs(func(string, ...string) string { return "" }, nil)).Parse(`{{ typeOf . }}`); err == nil || !strings.Contains(err.Error(), `function "typeOf" not defined`) {
		t.Fatalf("typeOf alias should not exist, err=%v", err)
	}
}

func TestTemplateStringUtilities(t *testing.T) {
	cases := []struct {
		name string
		tmpl string
		want string
	}{
		{"join", `{{ join (split "a,b" ",") ":" }}`, "a:b"},
		{"lower", `{{ lower "Name" }}`, "name"},
		{"upper", `{{ upper "name" }}`, "NAME"},
		{"contains", `{{ contains "UserID" "ID" }}`, "true"},
		{"hasPrefix", `{{ hasPrefix "UserID" "User" }}`, "true"},
		{"hasSuffix", `{{ hasSuffix "UserID" "ID" }}`, "true"},
		{"trimPrefix", `{{ trimPrefix "SigName" "Sig" }}`, "Name"},
		{"trimSuffix", `{{ trimSuffix "NameID" "ID" }}`, "Name"},
		{"replace", `{{ replace "UserID" "ID" "Id" }}`, "UserId"},
		{"quote", `{{ quote "hello" }}`, `"hello"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := executeUtilityTemplate(t, tc.tmpl, nil)
			if got != tc.want {
				t.Fatalf("template = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUtilityKeys(t *testing.T) {
	fields := []Field{{Name: "B"}, {Name: "A"}}
	assertStrings(t, keysOf(Invocation{Fields: fields}), []string{"B", "A"})
	assertStrings(t, keysOf(&Type{Fields: fields}), []string{"B", "A"})
	assertStrings(t, keysOf(map[string]string{"b": "2", "a": "1"}), []string{"a", "b"})
	assertStrings(t, keysOf(map[string]any{"b": 2, "a": 1}), []string{"a", "b"})
	assertStrings(t, keysOf(map[int]string{1: "a"}), nil)
	assertStrings(t, keysOf(nil), nil)
}

func TestUtilityFieldNames(t *testing.T) {
	fields := []Field{{Name: "ID"}, {Name: "Name"}}
	assertStrings(t, fieldNamesOf(Invocation{Fields: fields}), []string{"ID", "Name"})
	assertStrings(t, fieldNamesOf(&Type{Fields: fields}), []string{"ID", "Name"})
	assertStrings(t, fieldNamesOf(fields), []string{"ID", "Name"})
	assertStrings(t, fieldNamesOf(nil), nil)
}

func TestUtilityMethodNames(t *testing.T) {
	methods := []Method{{Name: "Touch"}, {Name: "Save"}}
	if got := methodNamesOf(Invocation{Methods: methods}); got != "Touch,Save" {
		t.Fatalf("methodNamesOf invocation = %q", got)
	}
	if got := methodNamesOf(&Type{Methods: methods}); got != "Touch,Save" {
		t.Fatalf("methodNamesOf type = %q", got)
	}
	if got := methodNamesOf(methods); got != "Touch,Save" {
		t.Fatalf("methodNamesOf slice = %q", got)
	}
	if got := methodNamesOf(nil); got != "" {
		t.Fatalf("methodNamesOf nil = %q", got)
	}
}

func TestUtilityTag(t *testing.T) {
	field := Field{Tag: `json:"id,omitempty" db:"user_id" empty:""`}
	if got := tagOf(field, "json"); got != "id,omitempty" {
		t.Fatalf("tagOf field = %q", got)
	}
	if got := tagOf(&field, "db"); got != "user_id" {
		t.Fatalf("tagOf pointer = %q", got)
	}
	if got := tagOf(field.Tag, "empty"); got != "" {
		t.Fatalf("tagOf empty value = %q", got)
	}
	if got := tagOf(nil, "json"); got != "" {
		t.Fatalf("tagOf nil = %q", got)
	}
}

func TestUtilityTagName(t *testing.T) {
	field := Field{Tag: `json:"id,omitempty" db:"user_id" empty:"" skip:"-"`}
	cases := map[string]string{"json": "id", "db": "user_id", "empty": "", "skip": "-", "missing": ""}
	for key, want := range cases {
		if got := tagName(field, key); got != want {
			t.Fatalf("tagName %s = %q, want %q", key, got, want)
		}
	}
}

func TestUtilityTagOpts(t *testing.T) {
	field := Field{Tag: `json:"id,omitempty,string" db:"user_id" bare:",omitempty"`}
	assertStrings(t, tagOpts(field, "json"), []string{"omitempty", "string"})
	assertStrings(t, tagOpts(field, "db"), nil)
	assertStrings(t, tagOpts(field, "bare"), []string{"omitempty"})
	assertStrings(t, tagOpts(field, "missing"), nil)
}

func TestUtilityTagHas(t *testing.T) {
	field := Field{Tag: `json:"id,omitempty,string" bare:",omitempty"`}
	if !tagHas(field, "json", "omitempty") || !tagHas(field, "json", "string") {
		t.Fatal("tagHas did not find existing option")
	}
	if !tagHas(field, "bare", "omitempty") {
		t.Fatal("tagHas did not find option without tag name")
	}
	if !tagHas(field, "json", "id") {
		t.Fatal("tagHas should check all comma-separated parts, including the tag name")
	}
	if tagHas(field, "json", "missing") || tagHas(field, "missing", "omitempty") {
		t.Fatal("tagHas found missing option")
	}
}

func TestUtilityTagExists(t *testing.T) {
	field := Field{Tag: `json:"id" empty:""`}
	if !tagExists(field, "json") || !tagExists(field, "empty") {
		t.Fatal("tagExists should find present tags, including empty values")
	}
	if tagExists(field, "missing") || tagExists((*Field)(nil), "json") || tagExists(nil, "json") {
		t.Fatal("tagExists should be false for missing/nil values")
	}
}

func TestUtilityFieldsWithTag(t *testing.T) {
	fields := []Field{{Name: "ID", Tag: `json:"id"`}, {Name: "Name"}, {Name: "Email", Tag: `json:"email"`}}
	got := fieldsWithTag(Invocation{Fields: fields}, "json")
	assertFieldNames(t, got, []string{"ID", "Email"})
}

func TestUtilityFieldsWithoutTag(t *testing.T) {
	fields := []Field{{Name: "ID", Tag: `json:"id"`}, {Name: "Name"}, {Name: "Email", Tag: `json:"email"`}}
	got := fieldsWithoutTag(&Type{Fields: fields}, "json")
	assertFieldNames(t, got, []string{"Name"})
}

func TestUtilityExportedFields(t *testing.T) {
	fields := []Field{{Name: "ID"}, {Name: "name"}, {Name: "URL"}}
	assertFieldNames(t, exportedFields(fields), []string{"ID", "URL"})
}

func TestUtilityUnexportedFields(t *testing.T) {
	fields := []Field{{Name: "ID"}, {Name: "name"}, {Name: ""}}
	assertFieldNames(t, unexportedFields(fields), []string{"name", ""})
}

func TestUtilityEmbeddedFields(t *testing.T) {
	fields := []Field{{Name: "Base", Embedded: true}, {Name: "Name"}, {Name: "*Thing", Embedded: true}}
	assertFieldNames(t, embeddedFields(fields), []string{"Base", "*Thing"})
}

func TestUtilityNonEmbeddedFields(t *testing.T) {
	fields := []Field{{Name: "Base", Embedded: true}, {Name: "Name"}, {Name: "Email"}}
	assertFieldNames(t, nonEmbeddedFields(fields), []string{"Name", "Email"})
}

func TestUtilitySnake(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{"UserID", "user_id"},
		{"HTTPServer", "http_server"},
		{"user_id", "user_id"},
		{"user-id", "user_id"},
		{"User2FA", "user2_fa"},
		{Field{Name: "DBID"}, "dbid"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := snakeCase(tc.in); got != tc.want {
			t.Fatalf("snakeCase(%#v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestUtilityKebab(t *testing.T) {
	if got := kebabCase("UserID"); got != "user-id" {
		t.Fatalf("kebabCase = %q", got)
	}
}

func TestUtilityCamel(t *testing.T) {
	cases := map[any]string{"user_id": "userID", "UserID": "userID", "HTTPServer": "httpServer", "json_api": "jsonAPI", "": ""}
	for in, want := range cases {
		if got := camelCase(in); got != want {
			t.Fatalf("camelCase(%#v) = %q, want %q", in, got, want)
		}
	}
}

func TestUtilityPascal(t *testing.T) {
	cases := map[any]string{"user_id": "UserID", "UserID": "UserID", "HTTPServer": "HTTPServer", "json_api": "JSONAPI", "": ""}
	for in, want := range cases {
		if got := pascalCase(in); got != want {
			t.Fatalf("pascalCase(%#v) = %q, want %q", in, got, want)
		}
	}
}

func TestUtilityInitial(t *testing.T) {
	if got := initialOf("User"); got != "u" {
		t.Fatalf("initialOf User = %q", got)
	}
	if got := initialOf("Éclair"); got != "é" {
		t.Fatalf("initialOf unicode = %q", got)
	}
	if got := initialOf(""); got != "" {
		t.Fatalf("initialOf empty = %q", got)
	}
}

func TestUtilityReceiver(t *testing.T) {
	cases := map[any]string{"User": "u", "UserProfile": "up", "HTTPServer": "hs", "": "x"}
	for in, want := range cases {
		if got := receiverNameFor(in); got != want {
			t.Fatalf("receiverNameFor(%#v) = %q, want %q", in, got, want)
		}
	}
}

func TestUtilityExported(t *testing.T) {
	if !isExported("User") || !isExported("Éclair") {
		t.Fatal("isExported should accept uppercase names")
	}
	if isExported("user") || isExported("") || isExported("1User") {
		t.Fatal("isExported should reject non-uppercase starts")
	}
}

func TestUtilityUnexported(t *testing.T) {
	cases := map[string]string{"User": "user", "URL": "uRL", "Éclair": "éclair", "": ""}
	for in, want := range cases {
		if got := unexported(in); got != want {
			t.Fatalf("unexported(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUtilityTypeChecks(t *testing.T) {
	if !isStringType(Field{Type: "string"}) || isStringType(Field{Type: "String"}) {
		t.Fatal("isStringType failed")
	}
	if !isBoolType(Field{Type: "bool"}) || isBoolType(Field{Type: "*bool"}) {
		t.Fatal("isBoolType failed")
	}
	for _, typ := range []string{"int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr"} {
		if !isIntType(Field{Type: typ}) {
			t.Fatalf("isIntType(%s) = false", typ)
		}
	}
	if isIntType(Field{Type: "float64"}) {
		t.Fatal("isIntType float64 = true")
	}
	if !isFloatType(Field{Type: "float32"}) || !isFloatType(Field{Type: "float64"}) || isFloatType(Field{Type: "int"}) {
		t.Fatal("isFloatType failed")
	}
	if !isSliceType(Field{Type: "[]string"}) || !isSliceType(Field{Type: "UserIDs", Underlying: "[]UserID"}) || isSliceType(Field{Type: "[3]string"}) {
		t.Fatal("isSliceType failed")
	}
	if !isMapType(Field{Type: "map[string]int"}) || !isMapType(Field{Type: "UserMap", Underlying: "map[string]User"}) || isMapType(Field{Type: "sync.Map"}) {
		t.Fatal("isMapType failed")
	}
	if !isPointerType(Field{Type: "*User"}) || isPointerType(Field{Type: "User"}) {
		t.Fatal("isPointerType failed")
	}
}

func TestUtilityElem(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{Field{Type: "[]string"}, "string"},
		{Field{Type: "*User"}, "User"},
		{Field{Type: "[]*User"}, "*User"},
		{Field{Type: "UserIDs", Underlying: "[]UserID"}, "UserID"},
		{Field{Type: "User"}, ""},
	}
	for _, tc := range cases {
		if got := elemType(tc.in); got != tc.want {
			t.Fatalf("elemType(%#v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestUtilityZero(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{Field{Type: "string"}, `""`},
		{Field{Type: "bool"}, "false"},
		{Field{Type: "int"}, "0"},
		{Field{Type: "float64"}, "0"},
		{Field{Type: "[]string"}, "nil"},
		{Field{Type: "UserIDs", Underlying: "[]UserID"}, "nil"},
		{Field{Type: "map[string]int"}, "nil"},
		{Field{Type: "*User"}, "nil"},
		{Field{Type: "chan int"}, "nil"},
		{Field{Type: "func()"}, "nil"},
		{Field{Type: "User", Underlying: "struct {\n\tID int\n}", TypeKind: "struct"}, "User{}"},
		{Type{Name: "User", Kind: "struct", Underlying: "struct {\n\tID int\n}"}, "User{}"},
		{Invocation{Name: "User", Kind: "struct", Type: &Type{Name: "User", Kind: "struct", Underlying: "struct {\n\tID int\n}"}}, "User{}"},
		{Type{Kind: "interface", Underlying: "Reader"}, "nil"},
		{Type{Underlying: "interface{ Read() }"}, "nil"},
		{Field{Type: ""}, "nil"},
		{Field{Type: "any"}, "nil"},
		{Field{Type: "error"}, "nil"},
		{Field{Type: "interface{}"}, "nil"},
		{Field{Type: "complex64"}, "0"},
		{Field{Type: "complex128"}, "0"},
		{Field{Type: "<-chan int"}, "nil"},
		{Field{Type: "chan<- int"}, "nil"},
		// Named non-struct types and arrays fall back to composite literals.
		{Field{Type: "[3]int"}, "[3]int{}"},
		{Field{Type: "time.Time"}, "time.Time{}"},
		{Field{Type: "Status", Underlying: "string"}, `""`},
	}
	for _, tc := range cases {
		if got := zeroValue(tc.in); got != tc.want {
			t.Fatalf("zeroValue(%#v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestUtilityDict(t *testing.T) {
	got, err := dict("name", "User", "count", 2)
	if err != nil {
		t.Fatal(err)
	}
	if got["name"] != "User" || got["count"] != 2 {
		t.Fatalf("dict = %#v", got)
	}
	if _, err := dict("name"); err == nil {
		t.Fatal("dict should reject odd argument count")
	}
	if _, err := dict(1, "value"); err == nil {
		t.Fatal("dict should reject non-string keys")
	}
}

func TestUtilityList(t *testing.T) {
	got := list("a", 1, true)
	want := []any{"a", 1, true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("list = %#v, want %#v", got, want)
	}
}

func TestUtilityGet(t *testing.T) {
	type sample struct {
		Name string
		hide string
	}
	if got := getValue(map[string]string{"name": "User"}, "name"); got != "User" {
		t.Fatalf("get map string = %#v", got)
	}
	if got := getValue(map[string]any{"count": 2}, "count"); got != 2 {
		t.Fatalf("get map any = %#v", got)
	}
	if got := getValue(sample{Name: "User", hide: "secret"}, "Name"); got != "User" {
		t.Fatalf("get struct = %#v", got)
	}
	if got := getValue(&sample{Name: "User"}, "Name"); got != "User" {
		t.Fatalf("get struct pointer = %#v", got)
	}
	if got := getValue(sample{Name: "User", hide: "secret"}, "hide"); got != nil {
		t.Fatalf("get unexported = %#v", got)
	}
	if got := getValue(map[string]string{"name": "User"}, 1); got != nil {
		t.Fatalf("get incompatible key = %#v", got)
	}
	if got := getValue(nil, "name"); got != nil {
		t.Fatalf("get nil = %#v", got)
	}
}

func TestUtilityArg(t *testing.T) {
	meta := Meta{Args: map[string]string{"mode": "fast"}, Argv: []string{"users", "public"}}
	cases := []struct {
		key  any
		want string
	}{
		{0, "users"},
		{1, "public"},
		{2, ""},
		{-1, ""},
		{"mode", "fast"},
		{"missing", ""},
	}
	for _, tc := range cases {
		if got := argValue(meta, tc.key); got != tc.want {
			t.Fatalf("argValue(%#v, %#v) = %q, want %q", meta, tc.key, got, tc.want)
		}
	}
}

func TestUtilityDefault(t *testing.T) {
	cases := []struct {
		name     string
		fallback any
		value    any
		want     any
	}{
		{"empty string", "fallback", "", "fallback"},
		{"non-empty string", "fallback", "value", "value"},
		{"zero int", 7, 0, 7},
		{"non-zero int", 7, 3, 3},
		{"false", true, false, true},
		{"true", false, true, true},
		{"nil", "fallback", nil, "fallback"},
		{"empty slice", []string{"fallback"}, []string{}, []string{"fallback"}},
		{"non-empty slice", []string{"fallback"}, []string{"value"}, []string{"value"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := defaultValue(tc.fallback, tc.value); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("defaultValue(%#v, %#v) = %#v, want %#v", tc.fallback, tc.value, got, tc.want)
			}
		})
	}
}

func executeUtilityTemplate(t *testing.T, source string, data any) string {
	t.Helper()
	tmpl, err := template.New("test").Funcs(templateFuncs(func(string, ...string) string { return "" }, nil)).Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func assertStrings(t *testing.T, got []string, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func assertFieldNames(t *testing.T, fields []Field, want []string) {
	t.Helper()
	got := make([]string, 0, len(fields))
	for _, field := range fields {
		got = append(got, field.Name)
	}
	assertStrings(t, got, want)
}
