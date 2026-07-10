package main

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"unicode"
)

type importSet struct {
	paths map[string]string
}

func newImportSet() *importSet {
	return &importSet{paths: map[string]string{}}
}

// add powers {{ imports "strconv" }} and {{ imports "encoding/json" "stdjson" }}.
// It registers a sidecar-file import and returns an empty string, so templates can call it inline:
//
//	{{ imports "strconv" }}
//	return strconv.Itoa(x)
func (s *importSet) add(path string, name ...string) string {
	alias := ""
	if len(name) > 0 {
		alias = name[0]
	}
	s.paths[path] = alias
	logger.Debug("registered import", "path", path, "alias", alias)
	return ""
}

func (s *importSet) write(out *bytes.Buffer) {
	if len(s.paths) == 0 {
		return
	}
	paths := make([]string, 0, len(s.paths))
	for path := range s.paths {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	out.WriteString("import (\n")
	for _, path := range paths {
		if alias := s.paths[path]; alias != "" {
			fmt.Fprintf(out, "\t%s %q\n", alias, path)
			continue
		}
		fmt.Fprintf(out, "\t%q\n", path)
	}
	out.WriteString(")\n\n")
}

func loadTemplates(files []string, imports func(string, ...string) string, arg func(any) string) (*template.Template, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("no .metago files found")
	}

	tmpl := template.New("metago").Funcs(templateFuncs(imports, arg))
	logger.Debug("found template files", "count", len(files), "files", files)
	for _, file := range files {
		logger.Debug("parsing template file", "file", file)
		if _, err := tmpl.ParseFiles(file); err != nil {
			return nil, fmt.Errorf("parse %s: %w", file, err)
		}
	}
	for _, template := range tmpl.Templates() {
		logger.Debug("registered template", "name", template.Name())
	}
	return tmpl, nil
}

// templateFuncs registers every Metago template helper.
func templateFuncs(imports func(string, ...string) string, arg func(any) string) template.FuncMap {
	if arg == nil {
		arg = func(any) string { return "" }
	}
	return template.FuncMap{
		"name":              nameOf,
		"typeof":            typeOf,
		"imports":           imports,
		"keys":              keysOf,
		"fieldNames":        fieldNamesOf,
		"methodNames":       methodNamesOf,
		"join":              joinStrings,
		"lower":             lowerString,
		"upper":             upperString,
		"contains":          containsString,
		"hasPrefix":         hasStringPrefix,
		"hasSuffix":         hasStringSuffix,
		"trimPrefix":        trimStringPrefix,
		"trimSuffix":        trimStringSuffix,
		"replace":           replaceString,
		"split":             splitString,
		"exported":          isExported,
		"unexported":        unexported,
		"quote":             quoteString,
		"snake":             snakeCase,
		"kebab":             kebabCase,
		"camel":             camelCase,
		"pascal":            pascalCase,
		"initial":           initialOf,
		"receiver":          receiverNameFor,
		"tag":               tagOf,
		"tagName":           tagName,
		"tagOpts":           tagOpts,
		"tagHas":            tagHas,
		"tagExists":         tagExists,
		"prop":              propValue,
		"props":             propGroup,
		"propHas":           propHasFlag,
		"propExists":        propGroupExists,
		"fieldsWithTag":     fieldsWithTag,
		"fieldsWithoutTag":  fieldsWithoutTag,
		"exportedFields":    exportedFields,
		"unexportedFields":  unexportedFields,
		"embeddedFields":    embeddedFields,
		"nonEmbeddedFields": nonEmbeddedFields,
		"isString":          isStringType,
		"isInt":             isIntType,
		"isBool":            isBoolType,
		"isFloat":           isFloatType,
		"isSlice":           isSliceType,
		"isMap":             isMapType,
		"isPointer":         isPointerType,
		"elem":              elemType,
		"zero":              zeroValue,
		"dict":              dict,
		"list":              list,
		"get":               getValue,
		"arg":               arg,
		"default":           defaultValue,
	}
}

// joinStrings powers {{ join .Fields "," }}.
// It joins a []string with a separator; use it to emit comma/newline separated lists.
//
//	{{ join (fieldNames .) ", " }} -> "ID, Name"
func joinStrings(values []string, sep string) string { return strings.Join(values, sep) }

// lowerString powers {{ lower "Name" }}.
// It lowercases a string; use it for simple text/name normalization.
//
//	{{ lower "UserID" }} -> "userid"
func lowerString(s string) string { return strings.ToLower(s) }

// upperString powers {{ upper "name" }}.
// It uppercases a string; use it for constants or generated text.
//
//	{{ upper "status" }} -> "STATUS"
func upperString(s string) string { return strings.ToUpper(s) }

// containsString powers {{ contains "UserID" "ID" }}.
// It reports whether a string contains a substring; use it for conditional generation based on names.
//
//	{{ if contains .Name "ID" }}...{{ end }}
func containsString(s string, substr string) bool { return strings.Contains(s, substr) }

// hasStringPrefix powers {{ hasPrefix "SigName" "Sig" }}.
// It reports whether a string starts with a prefix; use it for naming conventions.
//
//	{{ hasPrefix .Name "Sig" }} -> true
func hasStringPrefix(s string, prefix string) bool { return strings.HasPrefix(s, prefix) }

// hasStringSuffix powers {{ hasSuffix "NameID" "ID" }}.
// It reports whether a string ends with a suffix; use it for naming conventions.
//
//	{{ hasSuffix .Name "ID" }} -> true
func hasStringSuffix(s string, suffix string) bool { return strings.HasSuffix(s, suffix) }

// trimStringPrefix powers {{ trimPrefix "SigName" "Sig" }}.
// It removes a prefix when present; use it to derive names.
//
//	{{ trimPrefix "SigName" "Sig" }} -> "Name"
func trimStringPrefix(s string, prefix string) string { return strings.TrimPrefix(s, prefix) }

// trimStringSuffix powers {{ trimSuffix "NameID" "ID" }}.
// It removes a suffix when present; use it to derive names.
//
//	{{ trimSuffix "NameID" "ID" }} -> "Name"
func trimStringSuffix(s string, suffix string) string { return strings.TrimSuffix(s, suffix) }

// replaceString powers {{ replace "UserID" "ID" "Id" }}.
// It replaces all occurrences of old with new; use it for small name cleanup.
//
//	{{ replace "UserID" "ID" "Id" }} -> "UserId"
func replaceString(s string, old string, new string) string { return strings.ReplaceAll(s, old, new) }

// splitString powers {{ split "a,b" "," }}.
// It splits a string into []string; use it with range or join for tiny inline lists.
//
//	{{ range split "a,b" "," }}{{ . }}{{ end }}
func splitString(s string, sep string) []string { return strings.Split(s, sep) }

// quoteString powers {{ quote "hello" }}.
// It returns a Go string literal; use it whenever generated code emits string data.
//
//	return {{ quote (tagName . "json") }}
func quoteString(s string) string { return strconv.Quote(s) }

// nameOf powers {{ name . }}.
// It returns the identifier/name for an invocation, type, field, method, or value; use it when emitting Go identifiers.
//
// Given:
//
//	type User struct{}
//
// Template:
//
//	type {{ name . }}Meta struct{}
func nameOf(v any) string {
	switch v := v.(type) {
	case Invocation:
		return v.Name
	case *Invocation:
		if v != nil {
			return v.Name
		}
	case Type:
		return v.Name
	case *Type:
		if v != nil {
			return v.Name
		}
	case Field:
		return v.Name
	case *Field:
		if v != nil {
			return v.Name
		}
	case Method:
		return v.Name
	case *Method:
		if v != nil {
			return v.Name
		}
	case Function:
		return v.Name
	case *Function:
		if v != nil {
			return v.Name
		}
	case Param:
		return v.Name
	case *Param:
		if v != nil {
			return v.Name
		}
	case Value:
		return v.Name
	case *Value:
		if v != nil {
			return v.Name
		}
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
	return ""
}

// typeOf powers {{ typeof . }}.
// It returns the declared type text for fields/values, or the underlying type text for target types; use it for type-specific templates.
//
// Given `type Status string`: {{ typeof . }} -> "string".
// Given field `IDs UserIDs`: {{ typeof . }} -> "UserIDs".
func typeOf(v any) string {
	switch v := v.(type) {
	case Invocation:
		if v.Type != nil {
			return v.Type.Underlying
		}
		return v.TypeName
	case *Invocation:
		if v != nil && v.Type != nil {
			return v.Type.Underlying
		}
	case Type:
		return v.Underlying
	case *Type:
		if v != nil {
			return v.Underlying
		}
	case Field:
		return v.Type
	case *Field:
		if v != nil {
			return v.Type
		}
	case Param:
		return v.Type
	case *Param:
		if v != nil {
			return v.Type
		}
	case Value:
		return v.Type
	case *Value:
		if v != nil {
			return v.Type
		}
	case string:
		return v
	}
	return ""
}

// keysOf powers {{ keys . }}.
// It returns field names for types/invocations or sorted string keys for maps; use it for stable loops.
//
// Given `type User struct { ID int; Name string }`: {{ range keys . }}{{ . }}{{ end }} -> ID, Name.
// Given {{ dict "b" 2 "a" 1 }}: keys returns a, b.
func keysOf(v any) []string {
	switch v := v.(type) {
	case Invocation:
		return fieldNames(v.Fields)
	case *Invocation:
		if v == nil {
			return nil
		}
		return fieldNames(v.Fields)
	case Type:
		return fieldNames(v.Fields)
	case *Type:
		if v == nil {
			return nil
		}
		return fieldNames(v.Fields)
	case []Field:
		return fieldNames(v)
	case map[string]string:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return keys
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Map && rv.Type().Key().Kind() == reflect.String {
		keys := make([]string, 0, rv.Len())
		for _, key := range rv.MapKeys() {
			keys = append(keys, key.String())
		}
		sort.Strings(keys)
		return keys
	}
	return nil
}

// fieldNamesOf powers {{ fieldNames . }}.
// It returns field names for a type, invocation, or []Field; use it when you need names as a list.
//
// Given `type User struct { ID int; Name string }`: {{ join (fieldNames .) "," }} -> "ID,Name".
func fieldNamesOf(v any) []string {
	switch v := v.(type) {
	case Invocation:
		return fieldNames(v.Fields)
	case *Invocation:
		if v == nil {
			return nil
		}
		return fieldNames(v.Fields)
	case Type:
		return fieldNames(v.Fields)
	case *Type:
		if v == nil {
			return nil
		}
		return fieldNames(v.Fields)
	case []Field:
		return fieldNames(v)
	}
	return nil
}

func fieldNames(fields []Field) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	return names
}

// methodNamesOf powers {{ methodNames . }}.
// It returns comma-joined method names; use it for summaries or debug generated code.
//
// Given methods `Touch` and `Save`: {{ methodNames . }} -> "Touch,Save".
func methodNamesOf(v any) string {
	var methods []Method
	switch v := v.(type) {
	case Invocation:
		methods = v.Methods
	case *Invocation:
		if v != nil {
			methods = v.Methods
		}
	case Type:
		methods = v.Methods
	case *Type:
		if v != nil {
			methods = v.Methods
		}
	case []Method:
		methods = v
	}

	names := make([]string, 0, len(methods))
	for _, method := range methods {
		names = append(names, method.Name)
	}
	return strings.Join(names, ",")
}

// tagOf powers {{ tag . "json" }}.
// It returns the raw struct tag value for a key; use it when you need the full tag.
//
// Given field: ID int `json:"id,omitempty"`
//
//	{{ tag . "json" }} -> "id,omitempty"
func tagOf(v any, key string) string {
	switch v := v.(type) {
	case Field:
		return reflect.StructTag(v.Tag).Get(key)
	case *Field:
		if v != nil {
			return reflect.StructTag(v.Tag).Get(key)
		}
	case string:
		return reflect.StructTag(v).Get(key)
	}
	return ""
}

// tagHas powers {{ tagHas . "json" "omitempty" }}.
// It checks whether a comma-separated tag value contains a part; use it for tag-name or option checks.
//
// Given field: ID int `json:"id,omitempty"`
//
//	{{ tagHas . "json" "id" }} -> true
//	{{ tagHas . "json" "omitempty" }} -> true
func tagHas(v any, key string, value string) bool {
	for part := range strings.SplitSeq(tagOf(v, key), ",") {
		if part == value {
			return true
		}
	}
	return false
}

// tagExists powers {{ tagExists . "json" }}.
// It reports whether a struct tag key exists, even if its value is empty; use it to distinguish absent from present-empty tags.
//
// Given field: ID int `json:""`
//
//	{{ tagExists . "json" }} -> true
func tagExists(v any, key string) bool {
	switch v := v.(type) {
	case Field:
		_, ok := reflect.StructTag(v.Tag).Lookup(key)
		return ok
	case *Field:
		if v != nil {
			_, ok := reflect.StructTag(v.Tag).Lookup(key)
			return ok
		}
	case string:
		_, ok := reflect.StructTag(v).Lookup(key)
		return ok
	}
	return false
}

// tagName powers {{ tagName . "json" }}.
// It returns the first comma-separated tag part; use it for JSON/db/form names.
//
// Given field: ID int `json:"id,omitempty"`
//
//	{{ tagName . "json" }} -> "id"
func tagName(v any, key string) string {
	value := tagOf(v, key)
	name, _, _ := strings.Cut(value, ",")
	return name
}

// tagOpts powers {{ tagOpts . "json" }}.
// It returns tag parts after the first comma; use it when options alter generation.
//
// Given field: ID int `json:"id,omitempty,string"`
//
//	{{ range tagOpts . "json" }}{{ . }}{{ end }} -> "omitempty", "string"
func tagOpts(v any, key string) []string {
	value := tagOf(v, key)
	_, opts, ok := strings.Cut(value, ",")
	if !ok || opts == "" {
		return nil
	}
	return strings.Split(opts, ",")
}

// propsOf normalizes a field, type, method, function, or invocation into its //mgo:props groups.
// Prop helpers build on it.
func propsOf(v any) map[string]Prop {
	switch v := v.(type) {
	case Field:
		return v.Props
	case *Field:
		if v != nil {
			return v.Props
		}
	case Type:
		return v.Props
	case *Type:
		if v != nil {
			return v.Props
		}
	case Method:
		return v.Props
	case *Method:
		if v != nil {
			return v.Props
		}
	case Function:
		return v.Props
	case *Function:
		if v != nil {
			return v.Props
		}
	case Invocation:
		return propsOfInvocation(v)
	case *Invocation:
		if v != nil {
			return propsOfInvocation(*v)
		}
	}
	return nil
}

func propsOfInvocation(inv Invocation) map[string]Prop {
	if inv.Method != nil {
		return inv.Method.Props
	}
	if inv.Function != nil {
		return inv.Function.Props
	}
	if inv.Type != nil {
		return inv.Type.Props
	}
	return nil
}

// propValue powers {{ prop . "validate" "max" }}.
// It returns a key=value from a //mgo:props group on a symbol, or "" when the group or key is
// missing; use it to read generation metadata without overloading struct tags.
//
// Given field:
//
//	//mgo:props validate required max=2000
//	Text string
//
//	{{ prop . "validate" "max" }} -> "2000"
func propValue(v any, group string, key string) string {
	return propsOf(v)[group].Args[key]
}

// propGroup powers {{ props . "validate" }}.
// It returns the whole Prop group (with .Args and .Argv) for a symbol, or a zero Prop when
// missing; use it to range over a group's data.
//
//	{{ range (props . "validate").Argv }}...{{ end }}
func propGroup(v any, group string) Prop {
	return propsOf(v)[group]
}

// propHasFlag powers {{ propHas . "validate" "required" }}.
// It reports whether a //mgo:props group contains a bare flag; use it for boolean markers.
//
// Given field:
//
//	//mgo:props validate required
//	Text string
//
//	{{ propHas . "validate" "required" }} -> true
func propHasFlag(v any, group string, flag string) bool {
	return slices.Contains(propsOf(v)[group].Argv, flag)
}

// propGroupExists powers {{ propExists . "validate" }}.
// It reports whether a symbol has a //mgo:props group at all; use it to distinguish absent groups
// from empty ones.
//
//	{{ if propExists . "pii" }}...{{ end }}
func propGroupExists(v any, group string) bool {
	_, ok := propsOf(v)[group]
	return ok
}

// fieldsOf normalizes an invocation, type, pointer, or []Field into []Field. It supports field filter helpers; templates normally call fieldsWithTag/exportedFields/etc instead.
func fieldsOf(v any) []Field {
	switch v := v.(type) {
	case Invocation:
		return v.Fields
	case *Invocation:
		if v != nil {
			return v.Fields
		}
	case Type:
		return v.Fields
	case *Type:
		if v != nil {
			return v.Fields
		}
	case []Field:
		return v
	}
	return nil
}

// fieldsWithTag powers {{ fieldsWithTag . "json" }}.
// It returns fields that have a tag key; use it for serializers, mappers, and form/db generators.
//
// Given `type User struct { ID int `json:"id"`; Password string }`, ranging fieldsWithTag returns only ID.
func fieldsWithTag(v any, key string) []Field {
	var fields []Field
	for _, field := range fieldsOf(v) {
		if tagExists(field, key) {
			fields = append(fields, field)
		}
	}
	return fields
}

// fieldsWithoutTag powers {{ fieldsWithoutTag . "json" }}.
// It returns fields missing a tag key; use it to apply generated defaults.
//
// Given `type User struct { ID int `json:"id"`; Password string }`, ranging fieldsWithoutTag returns Password.
func fieldsWithoutTag(v any, key string) []Field {
	var fields []Field
	for _, field := range fieldsOf(v) {
		if !tagExists(field, key) {
			fields = append(fields, field)
		}
	}
	return fields
}

// exportedFields powers {{ exportedFields . }}.
// It returns fields whose names start uppercase; use it for generated code that must work across packages.
//
// Given fields `ID` and `name`, ranging exportedFields returns ID.
func exportedFields(v any) []Field {
	var fields []Field
	for _, field := range fieldsOf(v) {
		if isExported(field.Name) {
			fields = append(fields, field)
		}
	}
	return fields
}

// unexportedFields powers {{ unexportedFields . }}.
// It returns fields whose names are not exported; use it for same-package helpers or diagnostics.
//
// Given fields `ID` and `name`, ranging unexportedFields returns name.
func unexportedFields(v any) []Field {
	var fields []Field
	for _, field := range fieldsOf(v) {
		if !isExported(field.Name) {
			fields = append(fields, field)
		}
	}
	return fields
}

// embeddedFields powers {{ embeddedFields . }}.
// It returns embedded fields; use it for flattening or forwarding generated code.
//
// Given `type User struct { Base; Name string }`, ranging embeddedFields returns Base.
func embeddedFields(v any) []Field {
	var fields []Field
	for _, field := range fieldsOf(v) {
		if field.Embedded {
			fields = append(fields, field)
		}
	}
	return fields
}

// nonEmbeddedFields powers {{ nonEmbeddedFields . }}.
// It returns ordinary non-embedded fields; use it for normal struct field loops.
//
// Given `type User struct { Base; Name string }`, ranging nonEmbeddedFields returns Name.
func nonEmbeddedFields(v any) []Field {
	var fields []Field
	for _, field := range fieldsOf(v) {
		if !field.Embedded {
			fields = append(fields, field)
		}
	}
	return fields
}

// snakeCase powers {{ snake .Name }}.
// It converts names to snake_case; use it for JSON defaults, db columns, metrics, and event names.
//
//	{{ snake "UserID" }} -> "user_id"
func snakeCase(v any) string {
	return strings.Join(wordsOf(nameOf(v)), "_")
}

// kebabCase powers {{ kebab .Name }}.
// It converts names to kebab-case; use it for HTML attributes, CSS classes, and CLI flags.
//
//	{{ kebab "UserID" }} -> "user-id"
func kebabCase(v any) string {
	return strings.Join(wordsOf(nameOf(v)), "-")
}

// camelCase powers {{ camel .Name }}.
// It converts names to camelCase while preserving common Go initialisms; use it for JS-facing or private names.
//
//	{{ camel "user_id" }} -> "userID"
func camelCase(v any) string {
	words := wordsOf(nameOf(v))
	if len(words) == 0 {
		return ""
	}
	for i := 1; i < len(words); i++ {
		words[i] = goNameWord(words[i])
	}
	return strings.Join(words, "")
}

// pascalCase powers {{ pascal .Name }}.
// It converts names to PascalCase while preserving common Go initialisms; use it for exported Go names.
//
//	{{ pascal "user_id" }} -> "UserID"
func pascalCase(v any) string {
	words := wordsOf(nameOf(v))
	for i := range words {
		words[i] = goNameWord(words[i])
	}
	return strings.Join(words, "")
}

// initialOf powers {{ initial .Name }}.
// It returns the lowercase first rune; use it for short receivers.
//
//	{{ initial "User" }} -> "u"
func initialOf(v any) string {
	name := nameOf(v)
	if name == "" {
		return ""
	}
	r, _ := utf8Rune(name)
	return string(unicode.ToLower(r))
}

// receiverNameFor powers {{ receiver . }}.
// It returns lowercase initials for a type name; use it for generated method receivers.
//
// Given `type UserProfile struct{}`:
//
//	func ({{ receiver . }} {{ name . }}) Validate() error { ... } -> func (up UserProfile) ...
func receiverNameFor(v any) string {
	words := wordsOf(nameOf(v))
	if len(words) == 0 {
		return "x"
	}
	var out strings.Builder
	for _, word := range words {
		r, _ := utf8Rune(word)
		out.WriteRune(unicode.ToLower(r))
	}
	return out.String()
}

// wordsOf splits identifiers into lowercase words while handling separators, camel case, and common acronym boundaries. Naming helpers build on it.
func wordsOf(s string) []string {
	runes := []rune(s)
	var words []string
	var current []rune
	for i, r := range runes {
		if r == '_' || r == '-' || unicode.IsSpace(r) {
			if len(current) > 0 {
				words = append(words, strings.ToLower(string(current)))
				current = nil
			}
			continue
		}

		if len(current) > 0 && unicode.IsUpper(r) {
			previous := current[len(current)-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(previous) || unicode.IsDigit(previous) || unicode.IsUpper(previous) && nextLower {
				words = append(words, strings.ToLower(string(current)))
				current = nil
			}
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		words = append(words, strings.ToLower(string(current)))
	}
	return words
}

// goNameWord formats one word for Go identifiers, preserving common initialisms. Example: id -> ID.
func goNameWord(s string) string {
	if initialism, ok := commonInitialisms[s]; ok {
		return initialism
	}
	return capitalize(s)
}

var commonInitialisms = map[string]string{
	"api":   "API",
	"ascii": "ASCII",
	"cpu":   "CPU",
	"css":   "CSS",
	"db":    "DB",
	"dns":   "DNS",
	"eof":   "EOF",
	"guid":  "GUID",
	"html":  "HTML",
	"http":  "HTTP",
	"https": "HTTPS",
	"id":    "ID",
	"ip":    "IP",
	"json":  "JSON",
	"qps":   "QPS",
	"ram":   "RAM",
	"rpc":   "RPC",
	"sla":   "SLA",
	"smtp":  "SMTP",
	"sql":   "SQL",
	"ssh":   "SSH",
	"tcp":   "TCP",
	"tls":   "TLS",
	"ttl":   "TTL",
	"udp":   "UDP",
	"ui":    "UI",
	"uid":   "UID",
	"uuid":  "UUID",
	"uri":   "URI",
	"url":   "URL",
	"utf8":  "UTF8",
	"vm":    "VM",
	"xml":   "XML",
	"xmpp":  "XMPP",
	"xsrf":  "XSRF",
	"xss":   "XSS",
}

// capitalize uppercases the first rune of a word. It is used by naming helpers after initialism handling.
func capitalize(s string) string {
	if s == "" {
		return ""
	}
	r, size := utf8Rune(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// resolvedTypeOf returns a field's resolved underlying type when known, otherwise typeOf. Type predicate helpers use it so aliases like type UserIDs []UserID behave like slices.
func resolvedTypeOf(v any) string {
	switch v := v.(type) {
	case Field:
		if v.Underlying != "" {
			return v.Underlying
		}
	case *Field:
		if v != nil && v.Underlying != "" {
			return v.Underlying
		}
	}
	return typeOf(v)
}

// isStringType powers {{ isString . }}.
// It reports whether the resolved type is string, including local aliases; use it for string-specific code.
//
// Given `type Email string`, field `Email Email`: {{ isString . }} -> true.
func isStringType(v any) bool { return resolvedTypeOf(v) == "string" }

// isBoolType powers {{ isBool . }}.
// It reports whether the resolved type is bool, including local aliases; use it for boolean code.
//
// Given `type Enabled bool`, field `Enabled Enabled`: {{ isBool . }} -> true.
func isBoolType(v any) bool { return resolvedTypeOf(v) == "bool" }

// isIntType powers {{ isInt . }}.
// It reports whether the resolved type is a Go int/uint type, including local aliases; use it for numeric code.
//
// Given `type Count int`, field `Count Count`: {{ isInt . }} -> true.
func isIntType(v any) bool {
	s := resolvedTypeOf(v)
	return s == "int" || s == "int8" || s == "int16" || s == "int32" || s == "int64" ||
		s == "uint" || s == "uint8" || s == "uint16" || s == "uint32" || s == "uint64" || s == "uintptr"
}

// isFloatType powers {{ isFloat . }}.
// It reports whether the resolved type is float32 or float64, including local aliases; use it for numeric code.
//
// Given `type Ratio float64`, field `Ratio Ratio`: {{ isFloat . }} -> true.
func isFloatType(v any) bool {
	s := resolvedTypeOf(v)
	return s == "float32" || s == "float64"
}

// isSliceType powers {{ isSlice . }}.
// It reports whether the resolved type is []T, including local aliases; use it for collection code.
//
// Given `type UserIDs []UserID`, field `IDs UserIDs`: {{ isSlice . }} -> true.
func isSliceType(v any) bool {
	return strings.HasPrefix(resolvedTypeOf(v), "[]")
}

// isMapType powers {{ isMap . }}.
// It reports whether the resolved type is map[K]V, including local aliases; use it for map code.
//
// Given `type Users map[string]User`, field `Users Users`: {{ isMap . }} -> true.
func isMapType(v any) bool {
	return strings.HasPrefix(resolvedTypeOf(v), "map[")
}

// isPointerType powers {{ isPointer . }}.
// It reports whether the resolved type is *T, including local aliases; use it for nil checks or dereferencing code.
//
// Given field `Owner *User`: {{ isPointer . }} -> true.
func isPointerType(v any) bool {
	return strings.HasPrefix(resolvedTypeOf(v), "*")
}

// elemType powers {{ elem . }}.
// It returns T for []T or *T, including local aliases; use it when generating item or dereferenced code.
//
// Given `type UserIDs []UserID`, field `IDs UserIDs`: {{ elem . }} -> "UserID".
func elemType(v any) string {
	s := resolvedTypeOf(v)
	if after, ok := strings.CutPrefix(s, "[]"); ok {
		return after
	}
	if after, ok := strings.CutPrefix(s, "*"); ok {
		return after
	}
	return ""
}

// zeroValue powers {{ zero . }}.
// It returns a Go zero-value expression for a Metago type, field, invocation, or raw type string; use it for defaults and initializers.
//
// Given `type User struct { ID int }`, {{ zero . }} on User -> "User{}".
// Given field `ID int`, {{ zero . }} -> "0".
// Given field `Owner *User`, {{ zero . }} -> "nil".
// Raw type strings also work: {{ zero "string" }} -> "\"\"" and {{ zero "[]User" }} -> "nil".
func zeroValue(v any) string {
	if isInterface(v) {
		return "nil"
	}
	if name, ok := zeroStructLiteral(v); ok {
		return name + "{}"
	}
	s := resolvedTypeOf(v)
	switch {
	case s == "string":
		return "\"\""
	case s == "bool":
		return "false"
	case s == "any" || s == "error":
		return "nil"
	case isIntType(v) || isFloatType(v) || s == "complex64" || s == "complex128":
		return "0"
	case strings.HasPrefix(s, "[]") || strings.HasPrefix(s, "map[") || strings.HasPrefix(s, "*") ||
		strings.HasPrefix(s, "chan ") || strings.HasPrefix(s, "chan<-") || strings.HasPrefix(s, "<-chan ") ||
		strings.HasPrefix(s, "func(") || strings.HasPrefix(s, "interface{"):
		return "nil"
	case s == "":
		return "nil"
	default:
		return s + "{}"
	}
}

// zeroStructLiteral returns the declared struct type name when zero should be a composite literal.
// It keeps {{ zero . }} on `type User struct{}` as User{} instead of invalid `struct{}{}`, and field `Owner User` as User{}.
func zeroStructLiteral(v any) (string, bool) {
	switch v := v.(type) {
	case Invocation:
		if v.Kind == "struct" {
			return v.Name, true
		}
		if v.Type != nil && v.Type.Kind == "struct" {
			return v.Type.Name, true
		}
	case *Invocation:
		if v != nil {
			return zeroStructLiteral(*v)
		}
	case Type:
		if v.Kind == "struct" {
			return v.Name, true
		}
	case *Type:
		if v != nil {
			return zeroStructLiteral(*v)
		}
	case Field:
		if v.TypeKind == "struct" {
			if v.Type != "" {
				return v.Type, true
			}
			return v.Underlying, true
		}
	case *Field:
		if v != nil {
			return zeroStructLiteral(*v)
		}
	}
	return "", false
}

// isInterface reports whether a type/invocation is an interface. zeroValue uses it so interface targets return nil.
func isInterface(v any) bool {
	switch v := v.(type) {
	case Invocation:
		return v.Kind == "interface" || v.Type != nil && v.Type.Kind == "interface"
	case *Invocation:
		return v != nil && (v.Kind == "interface" || v.Type != nil && v.Type.Kind == "interface")
	case Type:
		return v.Kind == "interface"
	case *Type:
		return v != nil && v.Kind == "interface"
	}
	return false
}

// dict powers {{ dict "name" .Name }}.
// It builds a map from key/value pairs; use it to pass structured data to nested templates.
//
//	{{ template "field" (dict "Field" . "JSON" (tagName . "json")) }}
func dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict requires an even number of arguments")
	}
	m := map[string]any{}
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict keys must be strings")
		}
		m[key] = values[i+1]
	}
	return m, nil
}

// list powers {{ list "a" "b" }}.
// It returns its arguments as []any; use it for small inline enumerations.
//
//	{{ range list "created" "updated" "deleted" }}...{{ end }}
func list(values ...any) []any {
	return values
}

// getValue powers {{ get .Args "table" }}.
// It reads map keys or exported struct fields and returns nil when missing; use it for optional/dynamic lookups.
//
//	{{ get .Args "table" }}
func getValue(v any, key any) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Map:
		kv := reflect.ValueOf(key)
		if !kv.Type().AssignableTo(rv.Type().Key()) {
			return nil
		}
		value := rv.MapIndex(kv)
		if value.IsValid() {
			return value.Interface()
		}
	case reflect.Struct:
		name, ok := key.(string)
		if !ok {
			return nil
		}
		value := rv.FieldByName(name)
		if value.IsValid() && value.CanInterface() {
			return value.Interface()
		}
	}
	return nil
}

// argValue powers {{ arg 0 }} and {{ arg "name" }}.
// It returns positional annotation args by zero-based index, or named key=value args by key.
//
// Given `//mgo:gen table User users public mode=fast`, {{ arg 0 }} -> "users", {{ arg 1 }} -> "public", and {{ arg "mode" }} -> "fast".
func argValue(meta Meta, key any) string {
	switch key := key.(type) {
	case int:
		return positionalArg(meta.Argv, key)
	case int64:
		return positionalArg(meta.Argv, int(key))
	case string:
		return meta.Args[key]
	}
	return ""
}

func positionalArg(args []string, index int) string {
	if index < 0 || index >= len(args) {
		return ""
	}
	return args[index]
}

// defaultValue powers {{ default "users" (get .Args "table") }}.
// It returns fallback when value is zero/empty; use it for optional annotation args.
//
//	{{ get .Args "table" | default "users" }}
func defaultValue(fallback any, value any) any {
	if isZero(value) {
		return fallback
	}
	return value
}

// isZero reports whether a value is nil, false, numeric zero, an empty string, or an empty array/slice/map/chan. defaultValue uses it.
func isZero(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return rv.Len() == 0
	}
	return rv.IsZero()
}

// isExported powers {{ exported .Name }}.
// It reports whether a name starts with an uppercase rune; use it for visibility checks.
//
//	{{ if exported .Name }}...{{ end }}
func isExported(name string) bool {
	if name == "" {
		return false
	}
	r, _ := utf8Rune(name)
	return unicode.IsUpper(r)
}

// unexported powers {{ unexported .Name }}.
// It lowercases the first rune; use it for private helper names.
//
//	{{ unexported "User" }} -> "user"
func unexported(name string) string {
	if name == "" {
		return ""
	}
	r, size := utf8Rune(name)
	return string(unicode.ToLower(r)) + name[size:]
}

// utf8Rune returns the first rune and its byte size. Naming helpers use it for Unicode-safe first-rune edits.
func utf8Rune(s string) (rune, int) {
	for _, r := range s {
		return r, len(string(r))
	}
	return 0, 0
}
