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
	"slices"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

var logger = newLogger(false)

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

// Prop is one //mgo:props group attached to a symbol. Argv holds bare flags, Args holds key=value
// pairs. Multiple //mgo:props lines for the same group on the same symbol merge: flags union,
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
	resolver, dirs, err := newResolver(root)
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
		files, err := generateFilesWithTemplates(dir, templateFiles, resolver)
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
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || name == "meta.go" || strings.HasSuffix(name, "_meta.go") || strings.HasSuffix(name, "_test.go") {
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
	resolver, _, err := newResolver(dir)
	if err != nil {
		return nil, err
	}
	return generateFilesWithTemplates(dir, templateFiles, resolver)
}

func generateFilesWithTemplates(dir string, templateFiles []string, resolver *targetResolver) (map[string][]byte, error) {
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
		output := metaOutputPath(dir)
		generatedGroups[output] = append(generatedGroups[output], meta)
	}

	files := map[string][]byte{}
	outputs := sortedMapKeys(generatedGroups)
	for _, output := range outputs {
		src, err := generateMetas(templateFiles, pkg, generatedGroups[output], resolver)
		if err != nil {
			return nil, err
		}
		files[output] = src
	}

	inlineFiles := sortedMapKeys(inlineGroups)
	for _, file := range inlineFiles {
		src, err := generateInlineFile(templateFiles, pkg, file, inlineGroups[file], resolver)
		if err != nil {
			return nil, err
		}
		files[file] = src
	}
	return files, nil
}

func generateMetas(templateFiles []string, pkg *Package, metas []Meta, resolver *targetResolver) ([]byte, error) {
	imports := newImportSet()
	body, err := executeMetas(templateFiles, pkg, metas, imports, resolver)
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

func executeMetas(templateFiles []string, pkg *Package, metas []Meta, imports *importSet, resolver *targetResolver) ([]byte, error) {
	var body bytes.Buffer

	for _, meta := range metas {
		data, err := resolver.resolveInvocation(pkg, meta)
		if err != nil {
			return nil, err
		}

		logger.Debug("executing template", "template", meta.Template, "target", data.Name, "file", meta.File, "line", meta.Line, "inline", meta.Inline, "kind", data.Kind)
		tmpl, err := loadTemplates(templateFiles, imports.add, func(key any) string {
			return argValue(meta, key)
		})
		if err != nil {
			return nil, err
		}
		if err := tmpl.ExecuteTemplate(&body, meta.Template, data); err != nil {
			return nil, fmt.Errorf("%s:%d: execute template %q: %w", meta.File, meta.Line, meta.Template, err)
		}
		body.WriteByte('\n')
	}
	return body.Bytes(), nil
}

type targetResolver struct {
	root           string
	packagesByDir  map[string]*Package
	packagesByName map[string][]*Package
	packagesByPath map[string]*Package
	external       map[string]*Package
}

func newResolver(root string) (*targetResolver, []string, error) {
	dirs, err := findPackageDirs(root)
	if err != nil {
		return nil, nil, err
	}
	resolver := &targetResolver{
		root:           root,
		packagesByDir:  map[string]*Package{},
		packagesByName: map[string][]*Package{},
		packagesByPath: map[string]*Package{},
		external:       map[string]*Package{},
	}
	modulePath := modulePath(root)
	for _, dir := range dirs {
		pkg, _, err := scanPackage(dir)
		if err != nil {
			return nil, nil, err
		}
		if modulePath != "" {
			if rel, err := filepath.Rel(root, dir); err == nil && rel != "." {
				pkg.ImportPath = modulePath + "/" + filepath.ToSlash(rel)
			} else {
				pkg.ImportPath = modulePath
			}
		}
		resolver.packagesByDir[dir] = pkg
		resolver.packagesByName[pkg.Name] = append(resolver.packagesByName[pkg.Name], pkg)
		if pkg.ImportPath != "" {
			resolver.packagesByPath[pkg.ImportPath] = pkg
		}
	}
	return resolver, dirs, nil
}

func modulePath(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(after)
		}
	}
	return ""
}

func (r *targetResolver) resolveInvocation(pkg *Package, meta Meta) (Invocation, error) {
	data := Invocation{Package: pkg, Meta: meta, Args: meta.Args, Argv: meta.Argv, Functions: pkg.Functions}
	if meta.Target == "" {
		typ, fn := nearestTarget(pkg, meta.Line)
		if typ != nil {
			return typeInvocation(data, typ), nil
		}
		if fn != nil {
			return functionInvocation(data, fn), nil
		}
		return Invocation{}, fmt.Errorf("%s:%d: unknown meta target %q", meta.File, meta.Line, meta.Target)
	}

	if targetPkg, rest, ok, err := r.resolvePackagePrefix(meta.Target); err != nil {
		return Invocation{}, fmt.Errorf("%s:%d: %w", meta.File, meta.Line, err)
	} else if ok {
		return resolveInPackage(data, targetPkg, rest, meta)
	}

	if typeName, methodName, ok := strings.Cut(meta.Target, "."); ok {
		typ := findType(pkg, typeName)
		if typ != nil {
			for i := range typ.Methods {
				if typ.Methods[i].Name == methodName {
					return methodInvocation(data, typ, &typ.Methods[i]), nil
				}
			}
		}
	}

	if targetPkg, rest, ok, err := r.resolveImportPathTarget(meta.Target); err != nil {
		return Invocation{}, fmt.Errorf("%s:%d: %w", meta.File, meta.Line, err)
	} else if ok {
		return resolveInPackage(data, targetPkg, rest, meta)
	}

	if typ := findType(pkg, meta.Target); typ != nil {
		return typeInvocation(data, typ), nil
	}
	for i := range pkg.Functions {
		if pkg.Functions[i].Name == meta.Target {
			return functionInvocation(data, &pkg.Functions[i]), nil
		}
	}
	return Invocation{}, fmt.Errorf("%s:%d: unknown meta target %q", meta.File, meta.Line, meta.Target)
}

func (r *targetResolver) resolvePackagePrefix(target string) (*Package, string, bool, error) {
	prefix, rest, ok := strings.Cut(target, ".")
	if !ok {
		return nil, "", false, nil
	}
	pkgs := r.packagesByName[prefix]
	if len(pkgs) == 0 {
		return nil, "", false, nil
	}
	if len(pkgs) > 1 {
		candidates := make([]string, 0, len(pkgs))
		for _, pkg := range pkgs {
			if pkg.ImportPath != "" {
				candidates = append(candidates, pkg.ImportPath)
				continue
			}
			candidates = append(candidates, pkg.Dir)
		}
		return nil, "", false, fmt.Errorf("ambiguous package %q in target %q: %s", prefix, target, strings.Join(candidates, ", "))
	}
	return pkgs[0], rest, true, nil
}

func (r *targetResolver) resolveImportPathTarget(target string) (*Package, string, bool, error) {
	lastSlash := strings.LastIndex(target, "/")
	if lastSlash == -1 {
		return nil, "", false, nil
	}
	var loadErr error
	for i := lastSlash + 1; i < len(target); i++ {
		if target[i] != '.' {
			continue
		}
		pkgPath := target[:i]
		rest := target[i+1:]
		pkg, err := r.loadPackage(pkgPath)
		if err == nil {
			return pkg, rest, true, nil
		}
		loadErr = fmt.Errorf("load package %q: %w", pkgPath, err)
	}
	if loadErr != nil {
		return nil, "", false, loadErr
	}
	return nil, "", false, nil
}

func (r *targetResolver) loadPackage(importPath string) (*Package, error) {
	if pkg := r.packagesByPath[importPath]; pkg != nil {
		return pkg, nil
	}
	if pkg := r.external[importPath]; pkg != nil {
		return pkg, nil
	}
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax, Dir: r.root}
	pkgs, err := packages.Load(cfg, importPath)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 || len(pkgs[0].Errors) > 0 {
		if len(pkgs) > 0 && len(pkgs[0].Errors) > 0 {
			return nil, pkgs[0].Errors[0]
		}
		return nil, fmt.Errorf("package not found: %s", importPath)
	}
	pkg, err := packageFromLoaded(pkgs[0])
	if err != nil {
		return nil, err
	}
	r.external[importPath] = pkg
	return pkg, nil
}

func resolveInPackage(data Invocation, pkg *Package, target string, meta Meta) (Invocation, error) {
	if typeName, methodName, ok := strings.Cut(target, "."); ok {
		typ := findType(pkg, typeName)
		if typ != nil {
			for i := range typ.Methods {
				if typ.Methods[i].Name == methodName {
					return methodInvocation(data, typ, &typ.Methods[i]), nil
				}
			}
		}
		return Invocation{}, fmt.Errorf("%s:%d: unknown method target %q", meta.File, meta.Line, meta.Target)
	}
	if typ := findType(pkg, target); typ != nil {
		return typeInvocation(data, typ), nil
	}
	for i := range pkg.Functions {
		if pkg.Functions[i].Name == target {
			return functionInvocation(data, &pkg.Functions[i]), nil
		}
	}
	return Invocation{}, fmt.Errorf("%s:%d: unknown package target %q", meta.File, meta.Line, meta.Target)
}

func typeInvocation(data Invocation, typ *Type) Invocation {
	data.Type = typ
	data.Name = typ.Name
	data.Kind = typ.Kind
	data.TypeName = typ.Name
	data.Fields = typ.Fields
	data.Methods = typ.Methods
	data.Values = typ.Values
	data.IsType = true
	return data
}

func methodInvocation(data Invocation, typ *Type, method *Method) Invocation {
	data.Type = typ
	data.Method = method
	data.Name = method.Name
	data.Kind = "method"
	data.TypeName = typ.Name
	data.Fields = typ.Fields
	data.Methods = typ.Methods
	data.Params = method.Params
	data.Results = method.Results
	data.Body = method.Body
	data.Values = typ.Values
	data.IsMethod = true
	return data
}

func functionInvocation(data Invocation, fn *Function) Invocation {
	data.Function = fn
	data.Name = fn.Name
	data.Kind = "function"
	data.Params = fn.Params
	data.Results = fn.Results
	data.Body = fn.Body
	data.IsFunction = true
	return data
}

func findType(pkg *Package, name string) *Type {
	for _, typ := range pkg.Types {
		if typ.Name == name {
			return typ
		}
	}
	return nil
}

func nearestTarget(pkg *Package, line int) (*Type, *Function) {
	typ := nearestType(pkg.Types, line)
	fn := nearestFunction(pkg.Functions, line)
	if typ == nil || fn == nil {
		return typ, fn
	}
	if lineDistance(fn.Line, line) < lineDistance(typ.Line, line) {
		return nil, fn
	}
	return typ, nil
}

func nearestFunction(functions []Function, line int) *Function {
	bestIndex := -1
	bestDistance := int(^uint(0) >> 1)
	for i := range functions {
		distance := lineDistance(functions[i].Line, line)
		if distance < bestDistance {
			bestIndex = i
			bestDistance = distance
		}
	}
	if bestIndex == -1 {
		return nil
	}
	return &functions[bestIndex]
}

func lineDistance(a int, b int) int {
	distance := a - b
	if distance < 0 {
		distance = -distance
	}
	return distance
}

func generateInlineFile(templateFiles []string, pkg *Package, file string, metas []Meta, resolver *targetResolver) ([]byte, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(src), "\n")

	inlineImports := newImportSet()
	sort.Slice(metas, func(i, j int) bool { return metas[i].Line > metas[j].Line })
	for _, meta := range metas {
		insertEnd := meta.EndLine == 0
		if insertEnd {
			meta.EndLine = meta.Line + 1
		}
		if meta.EndLine <= meta.Line {
			return nil, fmt.Errorf("%s:%d: inline meta comment has invalid //mgo:end", meta.File, meta.Line)
		}

		body, err := executeMetas(templateFiles, pkg, []Meta{meta}, inlineImports, resolver)
		if err != nil {
			return nil, err
		}
		body = formatInlineBody(pkg.Name, body)

		replacement := strings.Split(strings.Trim(string(body), "\n"), "\n")
		if len(replacement) == 1 && replacement[0] == "" {
			replacement = nil
		}
		replacement = append([]string{""}, replacement...)
		replacement = append(replacement, "")
		if insertEnd {
			replacement = append(replacement, endDirective)
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
	if len(inlineImports.paths) == 0 {
		return out, nil
	}
	out, err = addImportsToSource(out, inlineImports)
	if err != nil {
		return nil, err
	}
	if formatted, err := format.Source(out); err == nil {
		out = formatted
	} else {
		logger.Warn("inline file could not be formatted after imports; writing raw output", "error", err)
	}
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

func addImportsToSource(src []byte, imports *importSet) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "inline.go", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	existing := map[string]bool{}
	for _, spec := range file.Imports {
		existing[strings.Trim(spec.Path.Value, "\"")] = true
	}
	newSpecs := importSpecs(imports, existing)
	if len(newSpecs) == 0 {
		return src, nil
	}

	for _, decl := range file.Decls {
		decl, ok := decl.(*ast.GenDecl)
		if !ok || decl.Tok != token.IMPORT {
			continue
		}
		if decl.Lparen.IsValid() {
			offset := fset.Position(decl.Rparen).Offset
			insert := "\t" + strings.Join(newSpecs, "\n\t") + "\n"
			out := slicesInsert(src, offset, []byte(insert))
			return out, nil
		}

		start := fset.Position(decl.Pos()).Offset
		end := fset.Position(decl.End()).Offset
		current := strings.TrimSpace(strings.TrimPrefix(string(src[start:end]), "import"))
		block := "import (\n\t" + current + "\n\t" + strings.Join(newSpecs, "\n\t") + "\n)"
		out := slicesReplace(src, start, end, []byte(block))
		return out, nil
	}

	packageEnd := fset.Position(file.Name.End()).Offset
	for packageEnd < len(src) && src[packageEnd] != '\n' {
		packageEnd++
	}
	insert := "\n\nimport (\n\t" + strings.Join(newSpecs, "\n\t") + "\n)"
	out := slicesInsert(src, packageEnd, []byte(insert))
	return out, nil
}

func importSpecs(imports *importSet, existing map[string]bool) []string {
	paths := make([]string, 0, len(imports.paths))
	for path := range imports.paths {
		if !existing[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)

	specs := make([]string, 0, len(paths))
	for _, path := range paths {
		if alias := imports.paths[path]; alias != "" {
			specs = append(specs, alias+" "+strconv.Quote(path))
			continue
		}
		specs = append(specs, strconv.Quote(path))
	}
	return specs
}

func slicesInsert(src []byte, offset int, insert []byte) []byte {
	out := make([]byte, 0, len(src)+len(insert))
	out = append(out, src[:offset]...)
	out = append(out, insert...)
	out = append(out, src[offset:]...)
	return out
}

func slicesReplace(src []byte, start int, end int, replacement []byte) []byte {
	out := make([]byte, 0, len(src)-(end-start)+len(replacement))
	out = append(out, src[:start]...)
	out = append(out, replacement...)
	out = append(out, src[end:]...)
	return out
}

func metaOutputPath(dir string) string {
	return filepath.Join(dir, "meta.go")
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
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || name == "meta.go" || strings.HasSuffix(name, "_meta.go") || strings.HasSuffix(name, "_test.go") {
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
	pendingMethods := map[string][]Method{}
	pendingValues := map[string][]Value{}
	var metas []Meta
	var props []Meta
	for _, filename := range filenames {
		logger.Debug("parsing go file", "file", filename)
		source, err := os.ReadFile(filename)
		if err != nil {
			return nil, nil, err
		}
		file, err := parser.ParseFile(fset, filename, source, parser.ParseComments)
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

		fileMetas, fileProps, err := scanMetas(fset, filename, file)
		if err != nil {
			return nil, nil, err
		}
		metas = append(metas, fileMetas...)
		props = append(props, fileProps...)
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.GenDecl:
				scanGenDecl(fset, filename, pkg, decl, pendingValues)
			case *ast.FuncDecl:
				if decl.Recv == nil || len(decl.Recv.List) == 0 {
					pkg.Functions = append(pkg.Functions, Function{
						Name:    decl.Name.Name,
						Params:  scanParams(fset, decl.Type.Params),
						Results: scanParams(fset, decl.Type.Results),
						Body:    bodyText(fset, source, decl.Body),
						File:    filename,
						Line:    fset.Position(decl.Pos()).Line,
					})
					logger.Debug("found function", "function", decl.Name.Name)
					continue
				}
				receiverField := decl.Recv.List[0]
				receiver := receiverName(receiverField.Type)
				method := Method{
					Name:         decl.Name.Name,
					Receiver:     receiverParamName(receiverField),
					ReceiverType: nodeString(fset, receiverField.Type),
					Params:       scanParams(fset, decl.Type.Params),
					Results:      scanParams(fset, decl.Type.Results),
					Body:         bodyText(fset, source, decl.Body),
					File:         filename,
					Line:         fset.Position(decl.Pos()).Line,
				}
				pendingMethods[receiver] = append(pendingMethods[receiver], method)
				logger.Debug("found method", "receiver", receiver, "method", decl.Name.Name)
			}
		}
	}
	attachPendingMethods(pkg, pendingMethods)
	attachPendingValues(pkg, pendingValues)
	resolveFieldTypes(pkg)
	if err := attachProps(pkg, props); err != nil {
		return nil, nil, err
	}
	sort.Slice(pkg.Types, func(i, j int) bool { return pkg.Types[i].Line < pkg.Types[j].Line })
	sort.Slice(metas, func(i, j int) bool {
		if metas[i].File == metas[j].File {
			return metas[i].Line < metas[j].Line
		}
		return metas[i].File < metas[j].File
	})
	pkg.Metas = metas
	return pkg, metas, nil
}

// attachProps binds //mgo:props metas to the nearest symbol declared in the same file: a struct
// field, method, function, or type. Nearest means smallest line distance; ties prefer the symbol
// below the comment (doc-comment convention), then fields/methods over their enclosing type.
func attachProps(pkg *Package, props []Meta) error {
	if len(props) == 0 {
		return nil
	}

	type propTarget struct {
		file        string
		line        int
		specificity int
		props       *map[string]Prop
	}
	var targets []propTarget
	for _, typ := range pkg.Types {
		targets = append(targets, propTarget{file: typ.File, line: typ.Line, specificity: 1, props: &typ.Props})
		for i := range typ.Fields {
			targets = append(targets, propTarget{file: typ.File, line: typ.Fields[i].Line, specificity: 2, props: &typ.Fields[i].Props})
		}
		for i := range typ.Methods {
			targets = append(targets, propTarget{file: typ.Methods[i].File, line: typ.Methods[i].Line, specificity: 2, props: &typ.Methods[i].Props})
		}
	}
	for i := range pkg.Functions {
		targets = append(targets, propTarget{file: pkg.Functions[i].File, line: pkg.Functions[i].Line, specificity: 1, props: &pkg.Functions[i].Props})
	}

	for _, prop := range props {
		var best *propTarget
		for i := range targets {
			target := &targets[i]
			if target.file != prop.File {
				continue
			}
			if best == nil || closerPropTarget(prop.Line, target.line, target.specificity, best.line, best.specificity) {
				best = target
			}
		}
		if best == nil {
			return fmt.Errorf("%s:%d: //mgo:props %s has no symbol to attach to", prop.File, prop.Line, prop.Target)
		}
		if *best.props == nil {
			*best.props = map[string]Prop{}
		}
		merged, ok := (*best.props)[prop.Target]
		if !ok {
			merged = Prop{Group: prop.Target, Args: map[string]string{}}
		}
		for key, value := range prop.Args {
			merged.Args[key] = value
		}
		for _, flag := range prop.Argv {
			if !slices.Contains(merged.Argv, flag) {
				merged.Argv = append(merged.Argv, flag)
			}
		}
		(*best.props)[prop.Target] = merged
		logger.Debug("attached props", "group", prop.Target, "file", prop.File, "line", prop.Line, "symbolLine", best.line)
	}
	return nil
}

// closerPropTarget reports whether candidate beats best for a props comment at metaLine.
func closerPropTarget(metaLine int, candidateLine int, candidateSpecificity int, bestLine int, bestSpecificity int) bool {
	candidateDistance := lineDistance(candidateLine, metaLine)
	bestDistance := lineDistance(bestLine, metaLine)
	if candidateDistance != bestDistance {
		return candidateDistance < bestDistance
	}
	candidateBelow := candidateLine >= metaLine
	bestBelow := bestLine >= metaLine
	if candidateBelow != bestBelow {
		return candidateBelow
	}
	return candidateSpecificity > bestSpecificity
}

func packageFromLoaded(loaded *packages.Package) (*Package, error) {
	pkg := &Package{Name: loaded.Name, ImportPath: loaded.PkgPath}
	pendingMethods := map[string][]Method{}
	pendingValues := map[string][]Value{}
	fset := loaded.Fset
	if fset == nil {
		fset = token.NewFileSet()
	}
	sources := map[string][]byte{}
	for _, filename := range loaded.GoFiles {
		source, err := os.ReadFile(filename)
		if err == nil {
			sources[filename] = source
		}
	}
	for _, file := range loaded.Syntax {
		filename := fset.Position(file.Pos()).Filename
		source := sources[filename]
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.GenDecl:
				scanGenDecl(fset, filename, pkg, decl, pendingValues)
			case *ast.FuncDecl:
				if decl.Recv == nil || len(decl.Recv.List) == 0 {
					pkg.Functions = append(pkg.Functions, Function{
						Name:    decl.Name.Name,
						Params:  scanParams(fset, decl.Type.Params),
						Results: scanParams(fset, decl.Type.Results),
						Body:    bodyText(fset, source, decl.Body),
						File:    filename,
						Line:    fset.Position(decl.Pos()).Line,
					})
					continue
				}
				receiverField := decl.Recv.List[0]
				receiver := receiverName(receiverField.Type)
				method := Method{
					Name:         decl.Name.Name,
					Receiver:     receiverParamName(receiverField),
					ReceiverType: nodeString(fset, receiverField.Type),
					Params:       scanParams(fset, decl.Type.Params),
					Results:      scanParams(fset, decl.Type.Results),
					Body:         bodyText(fset, source, decl.Body),
					File:         filename,
					Line:         fset.Position(decl.Pos()).Line,
				}
				pendingMethods[receiver] = append(pendingMethods[receiver], method)
			}
		}
	}
	attachPendingMethods(pkg, pendingMethods)
	attachPendingValues(pkg, pendingValues)
	resolveFieldTypes(pkg)
	sort.Slice(pkg.Types, func(i, j int) bool { return pkg.Types[i].Line < pkg.Types[j].Line })
	return pkg, nil
}

func scanGenDecl(fset *token.FileSet, filename string, pkg *Package, decl *ast.GenDecl, values map[string][]Value) {
	// Specs without a type or values inherit both from the previous spec, so iota blocks work.
	constType := ""
	var constValues []ast.Expr
	for _, spec := range decl.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			typ := &Type{
				Name:       spec.Name.Name,
				Kind:       typeKind(spec.Type),
				Underlying: nodeString(fset, spec.Type),
				File:       filename,
				Line:       fset.Position(spec.Pos()).Line,
			}
			if st, ok := spec.Type.(*ast.StructType); ok {
				typ.Fields = scanFields(fset, st)
			}
			if iface, ok := spec.Type.(*ast.InterfaceType); ok {
				typ.Methods = scanInterfaceMethods(fset, iface)
				for i := range typ.Methods {
					typ.Methods[i].File = filename
				}
			}
			logger.Debug("found type", "name", typ.Name, "kind", typ.Kind, "underlying", typ.Underlying, "fields", len(typ.Fields), "line", typ.Line)
			pkg.Types = append(pkg.Types, typ)
		case *ast.ValueSpec:
			if decl.Tok != token.CONST {
				continue
			}
			if spec.Type != nil {
				constType = nodeString(fset, spec.Type)
				constValues = spec.Values
			} else if len(spec.Values) > 0 {
				constType = ""
				constValues = nil
			}
			if constType == "" {
				continue
			}
			specValues := spec.Values
			if len(specValues) == 0 {
				specValues = constValues
			}
			for i, name := range spec.Names {
				if name.Name == "_" {
					continue
				}
				value := ""
				if i < len(specValues) {
					value = nodeString(fset, specValues[i])
				}
				values[constType] = append(values[constType], Value{Name: name.Name, Type: constType, Value: value})
				logger.Debug("found typed const", "name", name.Name, "type", constType, "value", value)
			}
		}
	}
}

func attachPendingMethods(pkg *Package, methods map[string][]Method) {
	for _, typ := range pkg.Types {
		typ.Methods = append(typ.Methods, methods[typ.Name]...)
	}
}

// attachPendingValues attaches typed consts after all files are scanned, so consts may be declared
// before their type or in a different file.
func attachPendingValues(pkg *Package, values map[string][]Value) {
	for _, typ := range pkg.Types {
		typ.Values = append(typ.Values, values[typ.Name]...)
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
		line := fset.Position(field.Pos()).Line
		tag := ""
		if field.Tag != nil {
			tag, _ = strconv.Unquote(field.Tag.Value)
		}
		if len(field.Names) == 0 {
			fields = append(fields, Field{Name: embeddedName(field.Type), Type: fieldType, Tag: tag, Embedded: true, Line: line})
			continue
		}
		for _, name := range field.Names {
			fields = append(fields, Field{Name: name.Name, Type: fieldType, Tag: tag, Line: line})
		}
	}
	return fields
}

func scanInterfaceMethods(fset *token.FileSet, iface *ast.InterfaceType) []Method {
	if iface == nil || iface.Methods == nil {
		return nil
	}
	var methods []Method
	for _, field := range iface.Methods.List {
		fn, ok := field.Type.(*ast.FuncType)
		if !ok {
			continue
		}
		for _, name := range field.Names {
			methods = append(methods, Method{
				Name:    name.Name,
				Params:  scanParams(fset, fn.Params),
				Results: scanParams(fset, fn.Results),
				Line:    fset.Position(field.Pos()).Line,
			})
		}
	}
	return methods
}

func scanParams(fset *token.FileSet, params *ast.FieldList) []Param {
	if params == nil {
		return nil
	}
	var out []Param
	for _, field := range params.List {
		param := Param{Type: nodeString(fset, field.Type)}
		if _, ok := field.Type.(*ast.Ellipsis); ok {
			param.Variadic = true
		}
		if len(field.Names) == 0 {
			out = append(out, param)
			continue
		}
		for _, name := range field.Names {
			param.Name = name.Name
			out = append(out, param)
		}
	}
	return out
}

// directivePrefix follows Go's directive comment convention (like //go:generate), which gofmt
// never reformats and go doc strips from rendered documentation.
const directivePrefix = "//mgo:"

const endDirective = directivePrefix + "end"

// isDirectiveComment reports whether a comment is any //mgo: directive, including malformed ones.
func isDirectiveComment(text string) bool {
	return strings.HasPrefix(text, directivePrefix)
}

// isEndDirective reports whether a comment is //mgo:end, tolerating trailing whitespace so an
// editor-added space never orphans an inline block's terminator.
func isEndDirective(text string) bool {
	return strings.TrimRight(strings.TrimPrefix(text, directivePrefix), " \t") == "end"
}

// scanMetas scans one file's comments for //mgo: directives. It returns generation metas
// (//mgo:gen and //mgo:inline) and props metas (//mgo:props) separately; //mgo:end is a marker
// consumed by inline end-binding. Any other //mgo: comment is an error so typos never silently
// no-op.
func scanMetas(fset *token.FileSet, filename string, file *ast.File) ([]Meta, []Meta, error) {
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
	var props []Meta
	for i, comment := range comments {
		if !isDirectiveComment(comment.text) {
			continue
		}
		trimmed := strings.TrimPrefix(comment.text, directivePrefix)
		words := strings.Fields(trimmed)
		verb := ""
		if len(words) > 0 {
			verb = words[0]
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, verb))
		switch verb {
		case "end":
			if rest != "" {
				return nil, nil, fmt.Errorf("%s:%d: //mgo:end takes no arguments", filename, comment.line)
			}
			continue
		case "gen", "inline":
			meta, err := parseMeta(rest, filename, comment.line)
			if err != nil {
				return nil, nil, err
			}
			meta.Inline = verb == "inline"
			if meta.Inline {
				for _, candidate := range comments[i+1:] {
					if isEndDirective(candidate.text) {
						meta.EndLine = candidate.line
						break
					}
					// A later meta comment means this //mgo:inline has no //mgo:end yet; without
					// this, a fresh //mgo:inline would steal the //mgo:end of the next annotation
					// and wipe everything between.
					if isDirectiveComment(candidate.text) {
						break
					}
				}
			}
			logger.Debug("found meta comment", "template", meta.Template, "target", meta.Target, "file", filename, "line", meta.Line, "inline", meta.Inline, "endLine", meta.EndLine, "args", meta.Args)
			metas = append(metas, meta)
		case "props":
			prop, err := parseProps(rest, filename, comment.line)
			if err != nil {
				return nil, nil, err
			}
			logger.Debug("found props comment", "group", prop.Target, "file", filename, "line", prop.Line, "args", prop.Args, "argv", prop.Argv)
			props = append(props, prop)
		default:
			return nil, nil, fmt.Errorf("%s:%d: invalid //mgo: directive %q: expected //mgo:gen, //mgo:inline, //mgo:props, or //mgo:end", filename, comment.line, comment.text)
		}
	}
	return metas, props, nil
}

func parseMeta(text, file string, line int) (Meta, error) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return Meta{}, fmt.Errorf("%s:%d: //mgo: directive is missing a template name", file, line)
	}
	meta := Meta{Template: parts[0], Args: map[string]string{}, File: file, Line: line}
	if len(parts) > 1 && !strings.Contains(parts[1], "=") && !isPathToken(parts[1]) {
		meta.Target = parts[1]
		parts = parts[2:]
	} else {
		parts = parts[1:]
	}
	for _, part := range parts {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			meta.Args[key] = value
			continue
		}
		meta.Argv = append(meta.Argv, part)
	}
	return meta, nil
}

// isPathToken reports whether an annotation token is a URL-path-like positional argument rather
// than a target name. Import-path targets like net/http.Client contain a slash but never start
// with one and never contain braces.
func isPathToken(s string) bool {
	return strings.HasPrefix(s, "/") || strings.Contains(s, "{")
}

// parseProps parses the text after //mgo:props. The group name is mandatory; the rest follows the
// usual positional and key=value grammar. Group is stored in Meta.Target.
func parseProps(text, file string, line int) (Meta, error) {
	parts := strings.Fields(text)
	if len(parts) == 0 || strings.Contains(parts[0], "=") {
		return Meta{}, fmt.Errorf("%s:%d: //mgo:props requires a group name, e.g. //mgo:props validate max=10", file, line)
	}
	meta := Meta{Template: "props", Target: parts[0], Args: map[string]string{}, File: file, Line: line}
	for _, part := range parts[1:] {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			meta.Args[key] = value
			continue
		}
		meta.Argv = append(meta.Argv, part)
	}
	return meta, nil
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

func bodyText(fset *token.FileSet, source []byte, body *ast.BlockStmt) string {
	if body == nil {
		return ""
	}
	start := fset.Position(body.Lbrace).Offset + 1
	end := fset.Position(body.Rbrace).Offset
	if start < 0 || end < start || end > len(source) {
		return ""
	}
	return string(source[start:end])
}

func receiverName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.StarExpr:
		return receiverName(expr.X)
	case *ast.IndexExpr:
		return receiverName(expr.X)
	case *ast.IndexListExpr:
		return receiverName(expr.X)
	case *ast.Ident:
		return expr.Name
	}
	return ""
}

func receiverParamName(field *ast.Field) string {
	if field == nil || len(field.Names) == 0 {
		return ""
	}
	return field.Names[0].Name
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
