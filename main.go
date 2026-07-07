package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var logger = newLogger(false)

type Package struct {
	Name  string
	Dir   string
	Types []*Type
}

type Type struct {
	Name       string
	Kind       string
	Underlying string
	Fields     []Field
	Methods    []Method
	Values     []Value
	Line       int
}

type Field struct {
	Name       string
	Type       string
	Underlying string
	TypeKind   string
	Tag        string
	Embedded   bool
}

type Method struct {
	Name string
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
	File     string
	Line     int
	Inline   bool
	EndLine  int
}

type Invocation struct {
	Package  *Package
	Meta     Meta
	Type     *Type
	Name     string
	Kind     string
	TypeName string
	Args     map[string]string
	Fields   []Field
	Methods  []Method
	Values   []Value
}

func main() {
	verbose := flag.Bool("v", false, "log what metago is doing")
	flag.BoolVar(verbose, "verbose", false, "log what metago is doing")
	flag.Parse()

	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}

	logger = newLogger(*verbose)
	logger.Debug("starting metago", "dir", dir)
	if err := run(dir); err != nil {
		fatal(err)
	}
}

func run(root string) error {
	dirs, err := findPackageDirs(root)
	if err != nil {
		return err
	}
	if len(dirs) == 0 {
		return fmt.Errorf("no Go package found in %s", root)
	}
	logger.Debug("found package directories", "count", len(dirs), "dirs", dirs)
	templateFiles, err := findTemplateFiles(root)
	if err != nil {
		return err
	}
	logger.Debug("found template files", "count", len(templateFiles), "files", templateFiles)

	for _, dir := range dirs {
		logger.Debug("generating package", "dir", dir)
		files, err := generateFilesWithTemplates(dir, templateFiles)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			logger.Debug("no meta comments found", "dir", dir)
			continue
		}

		outputs := sortedMapKeys(files)
		for _, output := range outputs {
			src := files[output]
			logger.Debug("writing generated file", "file", output, "bytes", len(src))
			if err := os.WriteFile(output, src, 0644); err != nil {
				return err
			}
			logger.Debug("generation complete", "file", output)
		}
	}
	return nil
}

func findPackageDirs(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && shouldSkipDir(entry.Name()) {
			return filepath.SkipDir
		}
		hasGoFiles, err := hasPackageGoFiles(path)
		if err != nil {
			return err
		}
		if hasGoFiles {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(dirs)
	return dirs, nil
}

func shouldSkipDir(name string) bool {
	return name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".")
}

func findTemplateFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && shouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".metago") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func hasPackageGoFiles(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_meta.go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		return true, nil
	}
	return false, nil
}

func generate(dir string) ([]byte, error) {
	files, err := generateFiles(dir)
	if err != nil {
		return nil, err
	}
	outputs := sortedMapKeys(files)
	if len(outputs) == 0 {
		return nil, nil
	}
	return files[outputs[0]], nil
}

func generateFiles(dir string) (map[string][]byte, error) {
	templateFiles, err := findTemplateFiles(dir)
	if err != nil {
		return nil, err
	}
	return generateFilesWithTemplates(dir, templateFiles)
}

func generateFilesWithTemplates(dir string, templateFiles []string) (map[string][]byte, error) {
	pkg, metas, err := scanPackage(dir)
	if err != nil {
		return nil, err
	}
	logger.Debug("scanned package", "package", pkg.Name, "types", len(pkg.Types), "metas", len(metas))
	if len(metas) == 0 {
		return nil, nil
	}

	generatedGroups := map[string][]Meta{}
	inlineGroups := map[string][]Meta{}
	for _, meta := range metas {
		if meta.Inline {
			inlineGroups[meta.File] = append(inlineGroups[meta.File], meta)
			continue
		}
		output := metaOutputPath(meta.File)
		generatedGroups[output] = append(generatedGroups[output], meta)
	}

	files := map[string][]byte{}
	outputs := sortedMapKeys(generatedGroups)
	for _, output := range outputs {
		src, err := generateMetas(templateFiles, pkg, generatedGroups[output])
		if err != nil {
			return nil, err
		}
		files[output] = src
	}

	inlineFiles := sortedMapKeys(inlineGroups)
	for _, file := range inlineFiles {
		src, err := generateInlineFile(templateFiles, pkg, file, inlineGroups[file])
		if err != nil {
			return nil, err
		}
		files[file] = src
	}
	return files, nil
}

func generateMetas(templateFiles []string, pkg *Package, metas []Meta) ([]byte, error) {
	imports := newImportSet()
	body, err := executeMetas(templateFiles, pkg, metas, imports)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	fmt.Fprintf(&out, "// Code generated by metago; DO NOT EDIT.\n\npackage %s\n\n", pkg.Name)
	imports.write(&out)
	out.Write(body)

	src := out.Bytes()
	logger.Debug("formatting generated source", "bytes", len(src))
	formatted, err := format.Source(src)
	if err == nil {
		src = formatted
		logger.Debug("formatted generated source", "bytes", len(src))
	} else {
		logger.Warn("generated source could not be formatted; writing raw output", "error", err)
	}
	return src, nil
}

func executeMetas(templateFiles []string, pkg *Package, metas []Meta, imports *importSet) ([]byte, error) {
	tmpl, err := loadTemplates(templateFiles, imports.add)
	if err != nil {
		return nil, err
	}

	var body bytes.Buffer

	typesByName := map[string]*Type{}
	for _, typ := range pkg.Types {
		typesByName[typ.Name] = typ
	}

	for _, meta := range metas {
		typ := typesByName[meta.Target]
		if typ == nil && meta.Target == "" {
			typ = nearestType(pkg.Types, meta.Line)
		}
		if typ == nil {
			return nil, fmt.Errorf("%s:%d: unknown meta target %q", meta.File, meta.Line, meta.Target)
		}

		logger.Debug("executing template", "template", meta.Template, "target", typ.Name, "file", meta.File, "line", meta.Line, "inline", meta.Inline)
		data := Invocation{
			Package:  pkg,
			Meta:     meta,
			Type:     typ,
			Name:     typ.Name,
			Kind:     typ.Kind,
			TypeName: typ.Name,
			Args:     meta.Args,
			Fields:   typ.Fields,
			Methods:  typ.Methods,
			Values:   typ.Values,
		}
		if err := tmpl.ExecuteTemplate(&body, meta.Template, data); err != nil {
			return nil, fmt.Errorf("%s:%d: execute template %q: %w", meta.File, meta.Line, meta.Template, err)
		}
		body.WriteByte('\n')
	}
	return body.Bytes(), nil
}

func generateInlineFile(templateFiles []string, pkg *Package, file string, metas []Meta) ([]byte, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(src), "\n")

	sort.Slice(metas, func(i, j int) bool { return metas[i].Line > metas[j].Line })
	for _, meta := range metas {
		insertEnd := meta.EndLine == 0
		if insertEnd {
			meta.EndLine = meta.Line + 1
		}
		if meta.EndLine <= meta.Line {
			return nil, fmt.Errorf("%s:%d: inline meta comment has invalid //end", meta.File, meta.Line)
		}

		imports := newImportSet()
		body, err := executeMetas(templateFiles, pkg, []Meta{meta}, imports)
		if err != nil {
			return nil, err
		}
		if len(imports.paths) > 0 {
			return nil, fmt.Errorf("%s:%d: inline meta templates cannot use imports helper", meta.File, meta.Line)
		}
		body = formatInlineBody(pkg.Name, body)

		replacement := strings.Split(strings.Trim(string(body), "\n"), "\n")
		if len(replacement) == 1 && replacement[0] == "" {
			replacement = nil
		}
		replacement = append([]string{""}, replacement...)
		replacement = append(replacement, "")
		if insertEnd {
			replacement = append(replacement, "//end")
		}
		start := meta.Line
		end := meta.EndLine - 1
		updated := make([]string, 0, len(lines)-max(0, end-start)+len(replacement))
		updated = append(updated, lines[:start]...)
		if len(replacement) != 1 || replacement[0] != "" {
			updated = append(updated, replacement...)
		}
		updated = append(updated, lines[end:]...)
		lines = updated
	}

	out := []byte(strings.Join(lines, "\n"))
	return out, nil
}

func formatInlineBody(packageName string, body []byte) []byte {
	src := fmt.Appendf(nil, "package %s\n\n%s", packageName, body)
	formatted, err := format.Source(src)
	if err != nil {
		logger.Warn("inline body could not be formatted; writing raw output", "error", err)
		return body
	}
	_, trimmed, ok := bytes.Cut(formatted, []byte("\n\n"))
	if !ok {
		return body
	}
	return trimmed
}

func metaOutputPath(source string) string {
	return strings.TrimSuffix(source, ".go") + "_meta.go"
}

func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func scanPackage(dir string) (*Package, []Meta, error) {
	logger.Debug("scanning package", "dir", dir)
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}

	filenames := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_meta.go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		filenames = append(filenames, filepath.Join(dir, name))
	}
	if len(filenames) == 0 {
		return nil, nil, fmt.Errorf("no Go package found in %s", dir)
	}
	sort.Strings(filenames)
	logger.Debug("found go files", "count", len(filenames), "files", filenames)

	pkg := &Package{Dir: dir}
	var metas []Meta
	for _, filename := range filenames {
		logger.Debug("parsing go file", "file", filename)
		file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
		if err != nil {
			return nil, nil, err
		}
		if pkg.Name == "" {
			pkg.Name = file.Name.Name
			logger.Debug("detected package name", "package", pkg.Name)
		}
		if file.Name.Name != pkg.Name {
			return nil, nil, fmt.Errorf("multiple packages found: %s, %s", pkg.Name, file.Name.Name)
		}

		metas = append(metas, scanMetas(fset, filename, file)...)
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.GenDecl:
				scanGenDecl(fset, pkg, decl)
			case *ast.FuncDecl:
				if decl.Recv != nil && len(decl.Recv.List) > 0 {
					receiver := receiverName(decl.Recv.List[0].Type)
					for _, typ := range pkg.Types {
						if typ.Name == receiver {
							typ.Methods = append(typ.Methods, Method{Name: decl.Name.Name})
							logger.Debug("found method", "receiver", receiver, "method", decl.Name.Name)
						}
					}
				}
			}
		}
	}
	resolveFieldTypes(pkg)
	sort.Slice(pkg.Types, func(i, j int) bool { return pkg.Types[i].Line < pkg.Types[j].Line })
	sort.Slice(metas, func(i, j int) bool {
		if metas[i].File == metas[j].File {
			return metas[i].Line < metas[j].Line
		}
		return metas[i].File < metas[j].File
	})
	return pkg, metas, nil
}

func scanGenDecl(fset *token.FileSet, pkg *Package, decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			typ := &Type{
				Name:       spec.Name.Name,
				Kind:       typeKind(spec.Type),
				Underlying: nodeString(fset, spec.Type),
				Line:       fset.Position(spec.Pos()).Line,
			}
			if st, ok := spec.Type.(*ast.StructType); ok {
				typ.Fields = scanFields(fset, st)
			}
			logger.Debug("found type", "name", typ.Name, "kind", typ.Kind, "underlying", typ.Underlying, "fields", len(typ.Fields), "line", typ.Line)
			pkg.Types = append(pkg.Types, typ)
		case *ast.ValueSpec:
			if decl.Tok != token.CONST {
				continue
			}
			constType := nodeString(fset, spec.Type)
			for i, name := range spec.Names {
				value := ""
				if i < len(spec.Values) {
					value = nodeString(fset, spec.Values[i])
				}
				for _, typ := range pkg.Types {
					if typ.Name == constType {
						typ.Values = append(typ.Values, Value{Name: name.Name, Type: constType, Value: value})
						logger.Debug("found typed const", "name", name.Name, "type", constType, "value", value)
					}
				}
			}
		}
	}
}

func resolveFieldTypes(pkg *Package) {
	typesByName := map[string]*Type{}
	for _, typ := range pkg.Types {
		typesByName[typ.Name] = typ
	}
	for _, typ := range pkg.Types {
		for i := range typ.Fields {
			underlying, kind := resolveUnderlyingType(typesByName, typ.Fields[i].Type, map[string]bool{})
			if underlying != typ.Fields[i].Type {
				typ.Fields[i].Underlying = underlying
			}
			typ.Fields[i].TypeKind = kind
		}
	}
}

func resolveUnderlyingType(typesByName map[string]*Type, name string, seen map[string]bool) (string, string) {
	typ := typesByName[name]
	if typ == nil || seen[name] {
		return name, ""
	}
	if typ.Kind == "struct" || typ.Kind == "interface" {
		return typ.Underlying, typ.Kind
	}
	seen[name] = true
	return resolveUnderlyingType(typesByName, typ.Underlying, seen)
}

func scanFields(fset *token.FileSet, st *ast.StructType) []Field {
	var fields []Field
	for _, field := range st.Fields.List {
		fieldType := nodeString(fset, field.Type)
		tag := ""
		if field.Tag != nil {
			tag, _ = strconv.Unquote(field.Tag.Value)
		}
		if len(field.Names) == 0 {
			fields = append(fields, Field{Name: embeddedName(field.Type), Type: fieldType, Tag: tag, Embedded: true})
			continue
		}
		for _, name := range field.Names {
			fields = append(fields, Field{Name: name.Name, Type: fieldType, Tag: tag})
		}
	}
	return fields
}

func scanMetas(fset *token.FileSet, filename string, file *ast.File) []Meta {
	type metaComment struct {
		text string
		line int
	}

	var comments []metaComment
	for _, group := range file.Comments {
		for _, comment := range group.List {
			comments = append(comments, metaComment{text: comment.Text, line: fset.Position(comment.Pos()).Line})
		}
	}
	sort.Slice(comments, func(i, j int) bool { return comments[i].line < comments[j].line })

	var metas []Meta
	for i, comment := range comments {
		prefix := ""
		switch {
		case strings.HasPrefix(comment.text, "//#"):
			prefix = "//#"
		case strings.HasPrefix(comment.text, "//@"):
			prefix = "//@"
		default:
			continue
		}

		meta, ok := parseMeta(strings.TrimSpace(strings.TrimPrefix(comment.text, prefix)), filename, comment.line)
		if !ok {
			continue
		}
		meta.Inline = prefix == "//@"
		if meta.Inline {
			for _, candidate := range comments[i+1:] {
				if candidate.text == "//end" {
					meta.EndLine = candidate.line
					break
				}
			}
		}
		logger.Debug("found meta comment", "template", meta.Template, "target", meta.Target, "file", filename, "line", meta.Line, "inline", meta.Inline, "endLine", meta.EndLine, "args", meta.Args)
		metas = append(metas, meta)
	}
	return metas
}

func parseMeta(text, file string, line int) (Meta, bool) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return Meta{}, false
	}
	meta := Meta{Template: parts[0], Args: map[string]string{}, File: file, Line: line}
	if len(parts) > 1 && !strings.Contains(parts[1], "=") {
		meta.Target = parts[1]
		parts = parts[2:]
	} else {
		parts = parts[1:]
	}
	for _, part := range parts {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			meta.Args[key] = value
		}
	}
	return meta, true
}

func nearestType(types []*Type, line int) *Type {
	var best *Type
	bestDistance := int(^uint(0) >> 1)
	for _, typ := range types {
		distance := typ.Line - line
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			best = typ
			bestDistance = distance
		}
	}
	return best
}

func typeKind(expr ast.Expr) string {
	switch expr.(type) {
	case *ast.StructType:
		return "struct"
	case *ast.InterfaceType:
		return "interface"
	default:
		return "type"
	}
}

func nodeString(fset *token.FileSet, node any) string {
	if node == nil {
		return ""
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return ""
	}
	return buf.String()
}

func receiverName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

func embeddedName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.StarExpr:
		return embeddedName(expr.X)
	case *ast.SelectorExpr:
		return expr.Sel.Name
	default:
		return ""
	}
}
