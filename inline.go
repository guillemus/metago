package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
)

func generateInlineFile(templateFiles []string, pkg *Package, file string, metas []Meta, resolver *targetResolver) ([]byte, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(src), "\n")

	inlineImports := newImportSet()
	var diagnostics []error

	// Anchored directives stacked on the same symbol share one generated block after it, executed
	// in directive order; each standalone directive owns the block right after its own line.
	regionStart := func(meta Meta) int {
		if meta.Anchored {
			return meta.AnchorEnd
		}
		return meta.Line
	}
	regions := map[int][]Meta{}
	for _, meta := range metas {
		start := regionStart(meta)
		regions[start] = append(regions[start], meta)
	}
	starts := make([]int, 0, len(regions))
	for start := range regions {
		starts = append(starts, start)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(starts)))

	for _, start := range starts {
		regionMetas := regions[start]
		sort.Slice(regionMetas, func(i, j int) bool { return regionMetas[i].Line < regionMetas[j].Line })
		first := regionMetas[0]
		endLine := first.EndLine
		insertEnd := endLine == 0
		if insertEnd {
			endLine = start + 1
		}
		if endLine <= start {
			return nil, fmt.Errorf("%s:%d: inline meta comment has invalid //mgo:end", first.File, first.Line)
		}

		body, err := executeMetas(templateFiles, pkg, regionMetas, inlineImports, resolver)
		if err != nil {
			diagnostics = append(diagnostics, err)
			continue
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
		end := endLine - 1
		updated := make([]string, 0, len(lines)-max(0, end-start)+len(replacement))
		updated = append(updated, lines[:start]...)
		if len(replacement) != 1 || replacement[0] != "" {
			updated = append(updated, replacement...)
		}
		updated = append(updated, lines[end:]...)
		lines = updated
	}

	if len(diagnostics) > 0 {
		return nil, errors.Join(diagnostics...)
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
