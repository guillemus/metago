package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"maps"
	"slices"
	"sort"
	"strings"
)

// attachProps binds property directives to the symbol whose documentation contains them.
// Package properties are intentionally unsupported: an unanchored property is an error.
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
		if !prop.Anchored {
			return fmt.Errorf("%s:%d: property %q has no symbol to attach to", prop.File, prop.Line, prop.Target)
		}
		var best *propTarget
		for i := range targets {
			target := &targets[i]
			if target.file != prop.File || target.line != prop.AnchorLine {
				continue
			}
			if best == nil || target.specificity > best.specificity {
				best = target
			}
		}
		if best == nil {
			return fmt.Errorf("%s:%d: property %q has no symbol to attach to", prop.File, prop.Line, prop.Target)
		}
		if *best.props == nil {
			*best.props = map[string]Prop{}
		}
		merged, ok := (*best.props)[prop.Target]
		if !ok {
			merged = Prop{Group: prop.Target, Args: map[string]string{}}
		}
		maps.Copy(merged.Args, prop.Args)
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

// directivePrefix follows Go's directive comment convention (like //go:generate), which gofmt
// never reformats and go doc strips from rendered documentation.
const directivePrefix = "//mgo:"

const endDirective = directivePrefix + "end"

var reservedDirectives = map[string]struct{}{
	"build": {}, "config": {}, "file": {}, "format": {}, "generate": {},
	"import": {}, "include": {}, "option": {}, "options": {}, "output": {},
	"package": {}, "plugin": {}, "profile": {}, "use": {},
}

var reservedMetaArgs = map[string]struct{}{
	"build": {}, "dir": {}, "file": {}, "format": {}, "group": {}, "mode": {},
	"order": {}, "output": {}, "package": {}, "scope": {}, "tags": {},
}

func isReservedMetaArg(key string) bool {
	if _, ok := reservedMetaArgs[key]; ok {
		return true
	}
	return key == "mgo" || strings.HasPrefix(key, "mgo.") || strings.HasPrefix(key, "mgo_") || strings.HasPrefix(key, "mgo-")
}

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
// (//mgo:gen and //mgo:inline) and property metas (every other namespace) separately;
// //mgo:end is consumed as the inline end marker.
func scanMetas(fset *token.FileSet, filename string, file *ast.File) ([]Meta, []Meta, error) {
	type metaComment struct {
		text  string
		line  int
		group int
	}

	var comments []metaComment
	for groupIndex, group := range file.Comments {
		for _, comment := range group.List {
			comments = append(comments, metaComment{text: comment.Text, line: fset.Position(comment.Pos()).Line, group: groupIndex})
		}
	}
	sort.Slice(comments, func(i, j int) bool { return comments[i].line < comments[j].line })

	anchors := scanAnchors(fset, file)

	var metas []Meta
	var props []Meta
	propertiesSeen := map[int]bool{}
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
			if propertiesSeen[comment.group] {
				return nil, nil, fmt.Errorf("%s:%d: //mgo:%s must come before properties in a directive stack", filename, comment.line, verb)
			}
			anchor, anchored := anchors[comment.line]
			meta, err := parseMeta(rest, filename, comment.line, anchored)
			if err != nil {
				return nil, nil, err
			}
			if anchored {
				meta.Target = anchor.target
				meta.Anchored = true
				meta.AnchorEnd = anchor.end
			}
			meta.Inline = verb == "inline"
			if meta.Inline {
				// Anchored inline blocks live after the annotated symbol, so end-binding starts
				// there; this also skips sibling directives stacked in the same doc comment and
				// any directive comments inside the symbol (e.g. field props).
				after := meta.Line
				if meta.Anchored {
					after = meta.AnchorEnd
				}
				for _, candidate := range comments[i+1:] {
					if candidate.line <= after {
						continue
					}
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
		default:
			if _, reserved := reservedDirectives[verb]; reserved {
				return nil, nil, fmt.Errorf("%s:%d: directive %q is reserved for future metago features", filename, comment.line, verb)
			}
			if verb == "" {
				return nil, nil, fmt.Errorf("%s:%d: property directive is missing a namespace", filename, comment.line)
			}
			if strings.Contains(verb, "=") {
				return nil, nil, fmt.Errorf("%s:%d: invalid property namespace %q", filename, comment.line, verb)
			}
			propertiesSeen[comment.group] = true
			prop := parseProperty(verb, rest, filename, comment.line)
			if anchor, ok := anchors[comment.line]; ok {
				prop.Anchored = true
				prop.AnchorLine = anchor.line
				prop.AnchorEnd = anchor.end
			}
			logger.Debug("found property comment", "namespace", prop.Target, "file", filename, "line", prop.Line, "args", prop.Args, "argv", prop.Argv)
			props = append(props, prop)
		}
	}
	return metas, props, nil
}

// anchor describes the symbol a doc-position directive is attached to.
type anchor struct {
	target string
	line   int
	end    int
}

// scanAnchors maps comment lines belonging to a type, function, or method doc comment to that
// symbol, so directives written there infer their target and inline insertion point from it.
// Directives above const/var declarations or separated from a symbol by a blank line are not
// anchored and keep standalone semantics.
func scanAnchors(fset *token.FileSet, file *ast.File) map[int]anchor {
	anchors := map[int]anchor{}
	for _, decl := range file.Decls {
		var doc *ast.CommentGroup
		var target string
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			doc = decl.Doc
			target = decl.Name.Name
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				target = receiverName(decl.Recv.List[0].Type) + "." + decl.Name.Name
			}
		case *ast.GenDecl:
			if decl.Tok != token.TYPE {
				continue
			}
			doc = decl.Doc
			for _, spec := range decl.Specs {
				if spec, ok := spec.(*ast.TypeSpec); ok {
					target = spec.Name.Name
					break
				}
			}
		}
		if doc == nil || target == "" {
			continue
		}
		line := fset.Position(decl.Pos()).Line
		end := fset.Position(decl.End()).Line
		for _, comment := range doc.List {
			anchors[fset.Position(comment.Pos()).Line] = anchor{target: target, line: line, end: end}
		}
	}

	ast.Inspect(file, func(node ast.Node) bool {
		field, ok := node.(*ast.Field)
		if !ok {
			return true
		}
		target := ""
		if len(field.Names) > 0 {
			target = field.Names[0].Name
		}
		line := fset.Position(field.Pos()).Line
		end := fset.Position(field.End()).Line
		for _, group := range []*ast.CommentGroup{field.Doc, field.Comment} {
			if group == nil {
				continue
			}
			for _, comment := range group.List {
				anchors[fset.Position(comment.Pos()).Line] = anchor{target: target, line: line, end: end}
			}
		}
		return true
	})
	return anchors
}

func parseMeta(text, file string, line int, anchored bool) (Meta, error) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return Meta{}, fmt.Errorf("%s:%d: //mgo: directive is missing a template name", file, line)
	}
	meta := Meta{Template: parts[0], Args: map[string]string{}, File: file, Line: line}
	if !anchored && len(parts) > 1 && !strings.Contains(parts[1], "=") && !isPathToken(parts[1]) {
		meta.Target = parts[1]
		parts = parts[2:]
	} else {
		parts = parts[1:]
	}
	var diagnostics []error
	for _, part := range parts {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			if isReservedMetaArg(key) {
				diagnostics = append(diagnostics, fmt.Errorf("%s:%d: argument %q is reserved for future metago features", file, line, key))
				continue
			}
			meta.Args[key] = value
			continue
		}
		meta.Argv = append(meta.Argv, part)
	}
	return meta, errors.Join(diagnostics...)
}

// isPathToken reports whether an annotation token is a URL-path-like positional argument rather
// than a target name. Import-path targets like net/http.Client contain a slash but never start
// with one and never contain braces.
func isPathToken(s string) bool {
	return strings.HasPrefix(s, "/") || strings.Contains(s, "{")
}

// parseProperty parses a property namespace's flags and key=value arguments.
// The namespace is stored in Meta.Target.
func parseProperty(namespace, text, file string, line int) Meta {
	meta := Meta{Template: "props", Target: namespace, Args: map[string]string{}, File: file, Line: line}
	for _, part := range strings.Fields(text) {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			meta.Args[key] = value
			continue
		}
		meta.Argv = append(meta.Argv, part)
	}
	return meta
}
