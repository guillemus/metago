package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"maps"
	"slices"
	"sort"
	"strings"
)

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
	propsSeen := map[int]bool{}
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
			if propsSeen[comment.group] {
				return nil, nil, fmt.Errorf("%s:%d: //mgo:%s must come before //mgo:props in a directive stack", filename, comment.line, verb)
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
		case "props":
			propsSeen[comment.group] = true
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

// anchor describes the symbol a doc-position directive is attached to.
type anchor struct {
	target string
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
		end := fset.Position(decl.End()).Line
		for _, comment := range doc.List {
			anchors[fset.Position(comment.Pos()).Line] = anchor{target: target, end: end}
		}
	}
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
