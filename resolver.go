package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

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
