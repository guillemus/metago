package main

import (
	"bytes"
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

	"golang.org/x/tools/go/packages"
)

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
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || name == "meta.go" ||
			strings.HasSuffix(name, "_meta.go") || strings.HasSuffix(name, "_test.go") {
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
				logger.Debug("found method",
					"receiver", receiver,
					"method", decl.Name.Name,
				)
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
			logger.Debug("found type",
				"name", typ.Name,
				"kind", typ.Kind,
				"underlying", typ.Underlying,
				"fields", len(typ.Fields),
				"line", typ.Line,
			)
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
				logger.Debug("found typed const",
					"name", name.Name,
					"type", constType,
					"value", value)
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
			fields = append(fields, Field{
				Name:     embeddedName(field.Type),
				Type:     fieldType,
				Tag:      tag,
				Embedded: true,
				Line:     line,
			})
			continue
		}
		for _, name := range field.Names {
			fields = append(fields, Field{
				Name: name.Name,
				Type: fieldType,
				Tag:  tag,
				Line: line,
			})
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
