package framework

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Data is a small convenience alias for view data.
type Data map[string]any

// Renderer renders a named template into an HTTP response.
type Renderer interface {
	Render(w http.ResponseWriter, status int, name string, data any) error
}

// ViewEngine renders native Go html/templates and Blade-like .ordin.html views.
//
// Blade-like views are compiled to html/template before execution, so escaped
// output remains safe by default.
type ViewEngine struct {
	Dir   string
	Funcs template.FuncMap
}

// NewViewEngine creates a renderer rooted at dir.
func NewViewEngine(dir string, funcs ...template.FuncMap) (*ViewEngine, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("views directory is empty")
	}
	if stat, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("views directory %q is not available: %w", dir, err)
	} else if !stat.IsDir() {
		return nil, fmt.Errorf("views path %q is not a directory", dir)
	}

	merged := defaultViewFuncs()
	for _, set := range funcs {
		for name, fn := range set {
			merged[name] = fn
		}
	}

	return &ViewEngine{Dir: dir, Funcs: merged}, nil
}

// MustViewEngine creates a view engine or panics with a readable message.
func MustViewEngine(dir string, funcs ...template.FuncMap) *ViewEngine {
	engine, err := NewViewEngine(dir, funcs...)
	if err != nil {
		panic(err)
	}
	return engine
}

// Render renders a view by name, for example "welcome" or "layouts.app".
func (v *ViewEngine) Render(w http.ResponseWriter, status int, name string, data any) error {
	if v == nil {
		return errors.New("view renderer is nil")
	}

	source, err := v.compileView(name, nil, nil)
	if err != nil {
		return err
	}

	tpl, err := template.New(name).Funcs(v.Funcs).Parse(source)
	if err != nil {
		return fmt.Errorf("parse view %q: %w", name, err)
	}

	var body bytes.Buffer
	if err := tpl.Execute(&body, data); err != nil {
		return fmt.Errorf("render view %q: %w", name, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err = w.Write(body.Bytes())
	return err
}

func defaultViewFuncs() template.FuncMap {
	return template.FuncMap{
		"raw": func(value any) template.HTML {
			return template.HTML(fmt.Sprint(value)) //nolint:gosec // Explicit opt-in raw HTML output.
		},
	}
}

func (v *ViewEngine) compileView(name string, sections map[string]string, stack []string) (string, error) {
	if contains(stack, name) {
		return "", fmt.Errorf("cyclic view reference: %s -> %s", strings.Join(stack, " -> "), name)
	}
	stack = append(stack, name)

	path, blade, err := v.resolve(name)
	if err != nil {
		return "", err
	}

	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read view %q: %w", name, err)
	}
	content := string(contentBytes)

	if !blade {
		return content, nil
	}

	parent := parseExtends(content)
	if parent != "" {
		childSections := parseSections(content)
		return v.compileView(parent, childSections, stack)
	}

	content = stripComments(content)
	content = removeExtends(content)
	content = replaceYields(content, sections)

	included, err := v.replaceIncludes(content, stack)
	if err != nil {
		return "", err
	}
	content = included

	content = stripSections(content)
	content = compileBlade(content)

	return content, nil
}

func (v *ViewEngine) replaceIncludes(content string, stack []string) (string, error) {
	re := regexp.MustCompile(`@include\(\s*["']([^"']+)["']\s*\)`)
	matches := re.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content, nil
	}

	var out strings.Builder
	last := 0
	for _, match := range matches {
		out.WriteString(content[last:match[0]])
		includeName := content[match[2]:match[3]]
		includeSource, err := v.compileView(includeName, nil, stack)
		if err != nil {
			return "", err
		}
		out.WriteString(includeSource)
		last = match[1]
	}
	out.WriteString(content[last:])
	return out.String(), nil
}

func (v *ViewEngine) resolve(name string) (string, bool, error) {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return "", false, errors.New("view name is empty")
	}

	variants := viewNameVariants(clean)
	for _, variant := range variants {
		path := filepath.Join(v.Dir, variant)
		if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
			return path, strings.HasSuffix(path, ".ordin.html"), nil
		}
	}

	return "", false, fmt.Errorf("view %q not found in %s", name, v.Dir)
}

func viewNameVariants(name string) []string {
	slash := strings.ReplaceAll(name, ".", string(filepath.Separator))
	slash = strings.Trim(slash, string(filepath.Separator))
	asPath := filepath.Clean(slash)
	if asPath == "." || asPath == ".." || strings.HasPrefix(asPath, ".."+string(filepath.Separator)) || filepath.IsAbs(asPath) {
		return nil
	}

	ext := filepath.Ext(asPath)
	if ext != "" {
		return []string{asPath}
	}

	return []string{
		asPath + ".ordin.html",
		asPath + ".html",
		asPath + ".tmpl",
	}
}

func parseExtends(content string) string {
	re := regexp.MustCompile(`@extends\(\s*["']([^"']+)["']\s*\)`)
	match := re.FindStringSubmatch(content)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func removeExtends(content string) string {
	re := regexp.MustCompile(`@extends\(\s*["'][^"']+["']\s*\)\s*`)
	return re.ReplaceAllString(content, "")
}

func parseSections(content string) map[string]string {
	sections := map[string]string{}
	re := regexp.MustCompile(`(?s)@section\(\s*["']([^"']+)["']\s*\)(.*?)@endsection`)
	for _, match := range re.FindAllStringSubmatch(content, -1) {
		sections[match[1]] = strings.TrimSpace(match[2])
	}
	return sections
}

func stripSections(content string) string {
	re := regexp.MustCompile(`(?s)@section\(\s*["'][^"']+["']\s*\)(.*?)@endsection`)
	return re.ReplaceAllString(content, "")
}

func replaceYields(content string, sections map[string]string) string {
	re := regexp.MustCompile(`@yield\(\s*["']([^"']+)["']\s*\)`)
	return re.ReplaceAllStringFunc(content, func(raw string) string {
		match := re.FindStringSubmatch(raw)
		if len(match) < 2 || sections == nil {
			return ""
		}
		return sections[match[1]]
	})
}

func stripComments(content string) string {
	re := regexp.MustCompile(`(?s)\{\{--.*?--\}\}`)
	return re.ReplaceAllString(content, "")
}

func compileBlade(content string) string {
	locals := collectBladeLocals(content)

	content = compileRawEcho(content, locals)
	content = compileEscapedEcho(content, locals)
	content = compileControlDirectives(content, locals)

	return content
}

func collectBladeLocals(content string) map[string]struct{} {
	locals := map[string]struct{}{}
	re := regexp.MustCompile(`@foreach\s+([^\n]+?)\s+as\s+([A-Za-z_][A-Za-z0-9_]*)`)
	for _, match := range re.FindAllStringSubmatch(content, -1) {
		locals[match[2]] = struct{}{}
	}
	return locals
}

func compileRawEcho(content string, locals map[string]struct{}) string {
	re := regexp.MustCompile(`(?s)\{!!\s*(.*?)\s*!!\}`)
	return re.ReplaceAllStringFunc(content, func(raw string) string {
		match := re.FindStringSubmatch(raw)
		if len(match) < 2 {
			return raw
		}
		return "{{ raw " + bladeExpr(match[1], locals) + " }}"
	})
}

func compileEscapedEcho(content string, locals map[string]struct{}) string {
	re := regexp.MustCompile(`(?s)\{\{\s*(.*?)\s*\}\}`)
	return re.ReplaceAllStringFunc(content, func(raw string) string {
		match := re.FindStringSubmatch(raw)
		if len(match) < 2 {
			return raw
		}
		expr := strings.TrimSpace(match[1])
		if isGoTemplateExpression(expr) {
			return raw
		}
		return "{{ " + bladeExpr(expr, locals) + " }}"
	})
}

func compileControlDirectives(content string, locals map[string]struct{}) string {
	foreach := regexp.MustCompile(`@foreach\s+([^\n]+?)\s+as\s+([A-Za-z_][A-Za-z0-9_]*)`)
	content = foreach.ReplaceAllStringFunc(content, func(raw string) string {
		match := foreach.FindStringSubmatch(raw)
		if len(match) < 3 {
			return raw
		}
		return "{{ range $" + match[2] + " := " + bladeExpr(match[1], locals) + " }}"
	})

	elseif := regexp.MustCompile(`@elseif\s+([^\n]+)`) // keep before @if
	content = elseif.ReplaceAllStringFunc(content, func(raw string) string {
		match := elseif.FindStringSubmatch(raw)
		if len(match) < 2 {
			return raw
		}
		return "{{ else if " + bladeExpr(match[1], locals) + " }}"
	})

	ifRe := regexp.MustCompile(`@if\s+([^\n]+)`)
	content = ifRe.ReplaceAllStringFunc(content, func(raw string) string {
		match := ifRe.FindStringSubmatch(raw)
		if len(match) < 2 {
			return raw
		}
		return "{{ if " + bladeExpr(match[1], locals) + " }}"
	})

	unless := regexp.MustCompile(`@unless\s+([^\n]+)`)
	content = unless.ReplaceAllStringFunc(content, func(raw string) string {
		match := unless.FindStringSubmatch(raw)
		if len(match) < 2 {
			return raw
		}
		return "{{ if not " + bladeExpr(match[1], locals) + " }}"
	})

	replacements := map[string]string{
		"@else":       "{{ else }}",
		"@endif":      "{{ end }}",
		"@endforeach": "{{ end }}",
		"@endunless":  "{{ end }}",
	}
	for from, to := range replacements {
		content = strings.ReplaceAll(content, from, to)
	}

	return content
}

func bladeExpr(expr string, locals map[string]struct{}) string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "."
	}
	if strings.HasPrefix(expr, ".") || strings.HasPrefix(expr, "$") || isLiteral(expr) {
		return expr
	}
	if strings.HasPrefix(expr, "!") {
		return "not " + bladeExpr(strings.TrimSpace(strings.TrimPrefix(expr, "!")), locals)
	}

	fields := strings.Fields(expr)
	if len(fields) > 1 && isTemplateFunction(fields[0]) {
		parts := []string{fields[0]}
		for _, field := range fields[1:] {
			parts = append(parts, bladeExpr(field, locals))
		}
		return strings.Join(parts, " ")
	}

	first := expr
	rest := ""
	if idx := strings.IndexAny(expr, ".|"); idx >= 0 {
		first = expr[:idx]
		rest = expr[idx:]
	}
	if _, ok := locals[first]; ok {
		return "$" + first + rest
	}

	return "." + expr
}

func isGoTemplateExpression(expr string) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false
	}
	if expr == "else" || expr == "end" {
		return true
	}
	prefixes := []string{".", "$", "if ", "range ", "else ", "template ", "define ", "block ", "with ", "/*"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(expr, prefix) {
			return true
		}
	}
	return false
}

func isLiteral(expr string) bool {
	if expr == "true" || expr == "false" || expr == "nil" {
		return true
	}
	if strings.HasPrefix(expr, "\"") || strings.HasPrefix(expr, "'") || strings.HasPrefix(expr, "`") {
		return true
	}
	if len(expr) > 0 && expr[0] >= '0' && expr[0] <= '9' {
		return true
	}
	return false
}

func isTemplateFunction(name string) bool {
	switch name {
	case "and", "or", "not", "eq", "ne", "lt", "le", "gt", "ge", "len", "printf", "print", "println", "index", "raw":
		return true
	default:
		return false
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// ViewNames returns available view names. It is useful for diagnostics and tests.
func (v *ViewEngine) ViewNames() ([]string, error) {
	if v == nil {
		return nil, errors.New("view renderer is nil")
	}

	var names []string
	err := filepath.WalkDir(v.Dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(v.Dir, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		switch {
		case strings.HasSuffix(name, ".ordin.html"):
			name = strings.TrimSuffix(name, ".ordin.html")
		case strings.HasSuffix(name, ".html"):
			name = strings.TrimSuffix(name, ".html")
		case strings.HasSuffix(name, ".tmpl"):
			name = strings.TrimSuffix(name, ".tmpl")
		default:
			return nil
		}
		names = append(names, strings.ReplaceAll(name, "/", "."))
		return nil
	})
	return names, err
}
