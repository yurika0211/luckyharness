// Package prompt provides a template engine for LuckyHarness prompts.
// It supports variable interpolation, conditionals, loops, partials,
// layout inheritance, and built-in helper functions.
package prompt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// PT-1: Template Definition & Parsing
// ---------------------------------------------------------------------------

// Template represents a compiled prompt template.
type Template struct {
	Name     string    `json:"name" yaml:"name"`
	Content  string    `json:"content" yaml:"content"`
	Layout   string    `json:"layout,omitempty" yaml:"layout,omitempty"`   // parent layout name
	Partials []string  `json:"partials,omitempty" yaml:"partials,omitempty"` // required partial names
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// RenderData holds the data context for rendering a template.
type RenderData map[string]interface{}

// node types for the parsed AST
type nodeType int

const (
	nodeText      nodeType = iota // plain text
	nodeVar                       // {{variable}}
	nodeIf                        // {{#if condition}} ... {{/if}}
	nodeEach                      // {{#each items}} ... {{/each}}
	nodePartial                   // {{>partial_name}}
	nodeBlock                     // {{#block name}} ... {{/block}}
	nodeHelper                    // {{helper arg1 arg2}}
)

// node is a single node in the parsed template AST.
type node struct {
	typ     nodeType
	text    string     // for nodeText, nodeVar
	children []*node   // for nodeIf, nodeEach, nodeBlock
	cond    string     // condition expression for nodeIf
	iterVar string     // iteration variable for nodeEach
	iterKey string     // iteration key for nodeEach (optional)
	name    string     // name for nodePartial, nodeBlock, nodeHelper
	args    []string   // args for nodeHelper
	alt     []*node    // else branch for nodeIf
}

// Parser parses template content into an AST.
type Parser struct{}

// NewParser creates a new template parser.
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses template content into a list of AST nodes.
func (p *Parser) Parse(content string) ([]*node, error) {
	return parseNodes(content)
}

// parseNodes recursively parses template content into nodes.
func parseNodes(content string) ([]*node, error) {
	var nodes []*node
	pos := 0

	// Regex patterns for template tags
	// Order matters: check longer patterns first
	tagPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\{\{#if\s+(.+?)\}\}`),        // {{#if cond}}
		regexp.MustCompile(`\{\{#each\s+(\w+)(?:\s+as\s+(\w+)(?:\s*,\s*(\w+))?)?\}\}`), // {{#each items [as val [, key]]}}
		regexp.MustCompile(`\{\{#block\s+(\w+)\}\}`),     // {{#block name}}
		regexp.MustCompile(`\{\{>\s*(\w+)\s*\}\}`),       // {{>partial}}
		regexp.MustCompile(`\{\{else\}\}`),                // {{else}}
		regexp.MustCompile(`\{\{/(if|each|block)\}\}`),   // {{/if}}, {{/each}}, {{/block}}
		regexp.MustCompile(`\{\{(\w+)\s+(.+?)\}\}`),      // {{helper args}}
		regexp.MustCompile(`\{\{(\w[\w.]*)\}\}`),         // {{variable}}
	}

	for pos < len(content) {
		// Find the next template tag
		nextTagIdx := findNextTag(content, pos)
		if nextTagIdx == -1 {
			// No more tags, rest is plain text
			if pos < len(content) {
				nodes = append(nodes, &node{typ: nodeText, text: content[pos:]})
			}
			break
		}

		// Add plain text before the tag
		if nextTagIdx > pos {
			nodes = append(nodes, &node{typ: nodeText, text: content[pos:nextTagIdx]})
		}

		// Match the tag
		matched := false
		for _, pat := range tagPatterns {
			loc := pat.FindStringSubmatchIndex(content[nextTagIdx:])
			if loc == nil || loc[0] != 0 {
				continue
			}
			tagEnd := nextTagIdx + loc[1]
			fullMatch := content[nextTagIdx:tagEnd]

			switch {
			case strings.HasPrefix(fullMatch, "{{#if"):
				cond := pat.FindStringSubmatch(fullMatch)[1]
				body, elseBody, endIdx, err := parseBlock(content, tagEnd, "if")
				if err != nil {
					return nil, err
				}
				bodyNodes, err := parseNodes(body)
				if err != nil {
					return nil, err
				}
				n := &node{typ: nodeIf, cond: cond, children: bodyNodes}
				if elseBody != "" {
					elseNodes, err := parseNodes(elseBody)
					if err != nil {
						return nil, err
					}
					n.alt = elseNodes
				}
				nodes = append(nodes, n)
				pos = endIdx
				matched = true

			case strings.HasPrefix(fullMatch, "{{#each"):
				subs := pat.FindStringSubmatch(fullMatch)
				iterVar := subs[1]
				var valName, keyName string
				if len(subs) > 2 {
					valName = subs[2]
				}
				if len(subs) > 3 {
					keyName = subs[3]
				}
				body, _, endIdx, err := parseBlock(content, tagEnd, "each")
				if err != nil {
					return nil, err
				}
				bodyNodes, err := parseNodes(body)
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, &node{
					typ:     nodeEach,
					name:    iterVar,
					iterVar: valName,
					iterKey: keyName,
					children: bodyNodes,
				})
				pos = endIdx
				matched = true

			case strings.HasPrefix(fullMatch, "{{#block"):
				blockName := pat.FindStringSubmatch(fullMatch)[1]
				body, _, endIdx, err := parseBlock(content, tagEnd, "block")
				if err != nil {
					return nil, err
				}
				bodyNodes, err := parseNodes(body)
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, &node{typ: nodeBlock, name: blockName, children: bodyNodes})
				pos = endIdx
				matched = true

			case strings.HasPrefix(fullMatch, "{{>"):
				partialName := pat.FindStringSubmatch(fullMatch)[1]
				nodes = append(nodes, &node{typ: nodePartial, name: partialName})
				pos = tagEnd
				matched = true

			case fullMatch == "{{else}}":
				// Handled by parseBlock, shouldn't appear at top level
				pos = tagEnd
				matched = true

			case strings.HasPrefix(fullMatch, "{{/") :
				// Closing tag at top level — skip
				pos = tagEnd
				matched = true

			case isHelperCall(fullMatch, pat):
				subs := pat.FindStringSubmatch(fullMatch)
				helperName := subs[1]
				args := strings.Fields(subs[2])
				nodes = append(nodes, &node{typ: nodeHelper, name: helperName, args: args})
				pos = tagEnd
				matched = true

			default:
				// Variable interpolation
				subs := pat.FindStringSubmatch(fullMatch)
				varName := subs[1]
				nodes = append(nodes, &node{typ: nodeVar, text: varName})
				pos = tagEnd
				matched = true
			}

			if matched {
				break
			}
		}

		if !matched {
			// Unknown tag, treat as text
			nodes = append(nodes, &node{typ: nodeText, text: content[nextTagIdx : nextTagIdx+2]})
			pos = nextTagIdx + 2
		}
	}

	return nodes, nil
}

// findNextTag finds the position of the next {{ tag.
func findNextTag(content string, from int) int {
	idx := strings.Index(content[from:], "{{")
	if idx == -1 {
		return -1
	}
	return from + idx
}

// parseBlock parses a block from start to its closing tag, handling {{else}} for if blocks.
func parseBlock(content string, start int, blockType string) (body string, elseBody string, endIdx int, err error) {
	depth := 1
	pos := start
	elsePos := -1

	for pos < len(content) && depth > 0 {
		openIdx := strings.Index(content[pos:], "{{#"+blockType)
		closeIdx := strings.Index(content[pos:], "{{/"+blockType+"}}")
		elseIdx := strings.Index(content[pos:], "{{else}}")

		// Find the nearest tag
		candidates := []int{}
		if openIdx != -1 {
			candidates = append(candidates, pos+openIdx)
		}
		if closeIdx != -1 {
			candidates = append(candidates, pos+closeIdx)
		}
		if elseIdx != -1 && depth == 1 {
			candidates = append(candidates, pos+elseIdx)
		}

		if len(candidates) == 0 {
			return "", "", 0, fmt.Errorf("unclosed {{#%s}} block", blockType)
		}

		// Find minimum
		minIdx := candidates[0]
		for _, c := range candidates[1:] {
			if c < minIdx {
				minIdx = c
			}
		}

		tagContent := content[minIdx:]
		if strings.HasPrefix(tagContent, "{{#"+blockType) {
			depth++
			pos = minIdx + len("{{#"+blockType)
		} else if strings.HasPrefix(tagContent, "{{/"+blockType+"}}") {
			depth--
			if depth == 0 {
				if elsePos != -1 {
					body = content[start:elsePos]
					elseBody = content[elsePos+8 : minIdx] // 8 = len("{{else}}")
				} else {
					body = content[start:minIdx]
				}
				endIdx = minIdx + len("{{/"+blockType+"}}")
				return
			}
			pos = minIdx + len("{{/"+blockType+"}}")
		} else if strings.HasPrefix(tagContent, "{{else}}") && depth == 1 {
			elsePos = minIdx
			pos = minIdx + 8
		} else {
			pos = minIdx + 2
		}
	}

	return "", "", 0, fmt.Errorf("unclosed {{#%s}} block", blockType)
}

func isHelperCall(fullMatch string, pat *regexp.Regexp) bool {
	subs := pat.FindStringSubmatch(fullMatch)
	if len(subs) < 3 {
		return false
	}
	// Any {{name args}} pattern is treated as a helper call
	// Unknown helpers will be rendered as-is during rendering
	return true
}

// ---------------------------------------------------------------------------
// PT-2: TemplateStore
// ---------------------------------------------------------------------------

// TemplateStore stores and retrieves templates.
type TemplateStore struct {
	mu        sync.RWMutex
	templates map[string]*Template
	dirs      []string // filesystem directories to watch
	watch     bool
}

// NewTemplateStore creates a new template store.
func NewTemplateStore() *TemplateStore {
	return &TemplateStore{
		templates: make(map[string]*Template),
	}
}

// AddDir adds a filesystem directory for template loading.
func (s *TemplateStore) AddDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirs = append(s.dirs, dir)
}

// SetWatch enables or disables hot-reloading from filesystem.
func (s *TemplateStore) SetWatch(watch bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watch = watch
}

// Register adds a template to the store.
func (s *TemplateStore) Register(t *Template) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t.Name == "" {
		return fmt.Errorf("template name is required")
	}
	s.templates[t.Name] = t
	return nil
}

// Get retrieves a template by name.
func (s *TemplateStore) Get(name string) (*Template, error) {
	s.mu.RLock()
	t, ok := s.templates[name]
	s.mu.RUnlock()

	if ok && !s.watch {
		return t, nil
	}

	// Try loading from filesystem
	if t == nil || s.watch {
		s.mu.Lock()
		for _, dir := range s.dirs {
			path := filepath.Join(dir, name+".tmpl")
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			t = &Template{
				Name:    name,
				Content: string(data),
			}
			s.templates[name] = t
			s.mu.Unlock()
			return t, nil
		}
		s.mu.Unlock()
	}

	if t == nil {
		return nil, fmt.Errorf("template not found: %s", name)
	}
	return t, nil
}

// List returns all registered template names.
func (s *TemplateStore) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Also scan directories
	names := make(map[string]bool)
	for name := range s.templates {
		names[name] = true
	}
	for _, dir := range s.dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			names[name] = true
		}
	}

	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	return result
}

// LoadFromDir loads all templates from a directory.
func (s *TemplateStore) LoadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".tmpl" && ext != ".prompt" && ext != ".txt" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		name := strings.TrimSuffix(entry.Name(), ext)
		t := &Template{
			Name:    name,
			Content: string(data),
		}
		if err := s.Register(t); err != nil {
			return err
		}
	}

	s.AddDir(dir)
	return nil
}

// Delete removes a template from the store.
func (s *TemplateStore) Delete(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.templates, name)
}

// ---------------------------------------------------------------------------
// PT-3: Render Engine
// ---------------------------------------------------------------------------

// HelperFunc is a function that can be called from templates.
type HelperFunc func(args []string, data RenderData) string

// builtinHelpers maps helper names to their implementations.
var builtinHelpers = map[string]HelperFunc{
	"upper":    helperUpper,
	"lower":    helperLower,
	"truncate": helperTruncate,
	"date":     helperDate,
	"join":     helperJoin,
	"default":  helperDefault,
}

// Engine renders templates using a store and parser.
type Engine struct {
	store   *TemplateStore
	parser  *Parser
	helpers map[string]HelperFunc
	mu      sync.RWMutex
}

// NewEngine creates a new render engine.
func NewEngine(store *TemplateStore) *Engine {
	e := &Engine{
		store:   store,
		parser:  NewParser(),
		helpers: make(map[string]HelperFunc),
	}
	// Register built-in helpers
	for name, fn := range builtinHelpers {
		e.helpers[name] = fn
	}
	return e
}

// RegisterHelper adds a custom helper function.
func (e *Engine) RegisterHelper(name string, fn HelperFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.helpers[name] = fn
}

// Render renders a template by name with the given data.
func (e *Engine) Render(name string, data RenderData) (string, error) {
	t, err := e.store.Get(name)
	if err != nil {
		return "", err
	}
	return e.RenderContent(t.Content, data)
}

// RenderContent renders raw template content with the given data.
func (e *Engine) RenderContent(content string, data RenderData) (string, error) {
	nodes, err := e.parser.Parse(content)
	if err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := e.renderNodes(&buf, nodes, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// renderNodes renders a list of AST nodes.
func (e *Engine) renderNodes(buf *bytes.Buffer, nodes []*node, data RenderData) error {
	for _, n := range nodes {
		switch n.typ {
		case nodeText:
			buf.WriteString(n.text)

		case nodeVar:
			val := resolveVar(n.text, data)
			buf.WriteString(fmt.Sprintf("%v", val))

		case nodeIf:
			cond := isTruthy(n.cond, data)
			if cond {
				if err := e.renderNodes(buf, n.children, data); err != nil {
					return err
				}
			} else if n.alt != nil {
				if err := e.renderNodes(buf, n.alt, data); err != nil {
					return err
				}
			}

		case nodeEach:
			items := resolveVar(n.name, data)
			slice, ok := toSlice(items)
			if !ok {
				continue
			}
			for i, item := range slice {
				iterData := make(RenderData)
				for k, v := range data {
					iterData[k] = v
				}
				if n.iterVar != "" {
					iterData[n.iterVar] = item
				} else {
					iterData["."] = item
				}
				if n.iterKey != "" {
					iterData[n.iterKey] = i
				}
				iterData["@index"] = i
				if err := e.renderNodes(buf, n.children, iterData); err != nil {
					return err
				}
			}

		case nodePartial:
			partial, err := e.store.Get(n.name)
			if err != nil {
				return fmt.Errorf("partial not found: %s", n.name)
			}
			partialNodes, err := e.parser.Parse(partial.Content)
			if err != nil {
				return fmt.Errorf("parse partial %s: %w", n.name, err)
			}
			if err := e.renderNodes(buf, partialNodes, data); err != nil {
				return err
			}

		case nodeBlock:
			// Blocks are rendered inline; layouts override them
			if err := e.renderNodes(buf, n.children, data); err != nil {
				return err
			}

		case nodeHelper:
			e.mu.RLock()
			fn, ok := e.helpers[n.name]
			e.mu.RUnlock()
			if !ok {
				buf.WriteString("{{" + n.name + " " + strings.Join(n.args, " ") + "}}")
				continue
			}
			result := fn(n.args, data)
			buf.WriteString(result)
		}
	}
	return nil
}

// RenderWithLayout renders a template with a layout (inheritance).
func (e *Engine) RenderWithLayout(name string, data RenderData) (string, error) {
	t, err := e.store.Get(name)
	if err != nil {
		return "", err
	}

	if t.Layout == "" {
		return e.Render(name, data)
	}

	// Render the child template first to get block content
	childContent, err := e.RenderContent(t.Content, data)
	if err != nil {
		return "", err
	}

	// Render the layout with the child content as "body"
	layoutData := make(RenderData)
	for k, v := range data {
		layoutData[k] = v
	}
	layoutData["body"] = childContent

	return e.Render(t.Layout, layoutData)
}

// Validate checks if a template can be parsed without errors.
func (e *Engine) Validate(content string) error {
	_, err := e.parser.Parse(content)
	return err
}

// ---------------------------------------------------------------------------
// Variable Resolution
// ---------------------------------------------------------------------------

// resolveVar resolves a dotted variable path from RenderData.
func resolveVar(path string, data RenderData) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, part := range parts {
		switch v := current.(type) {
		case RenderData:
			val, ok := v[part]
			if !ok {
				return ""
			}
			current = val
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return ""
			}
			current = val
		default:
			return ""
		}
	}

	return current
}

// isTruthy evaluates a condition expression.
func isTruthy(cond string, data RenderData) bool {
	cond = strings.TrimSpace(cond)

	// Check for negation
	if strings.HasPrefix(cond, "!") {
		return !isTruthy(cond[1:], data)
	}

	// Check for equality: var==value
	if parts := strings.SplitN(cond, "==", 2); len(parts) == 2 {
		left := strings.TrimSpace(resolveVarStr(parts[0], data))
		right := strings.TrimSpace(parts[1])
		return left == right
	}

	// Check for inequality: var!=value
	if parts := strings.SplitN(cond, "!=", 2); len(parts) == 2 {
		left := strings.TrimSpace(resolveVarStr(parts[0], data))
		right := strings.TrimSpace(parts[1])
		return left != right
	}

	// Simple variable truthiness
	val := resolveVar(cond, data)
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case int, int64, float64:
		return v != 0
	case nil:
		return false
	case []interface{}:
		return len(v) > 0
	default:
		return true
	}
}

// resolveVarStr resolves a variable and returns it as a string.
func resolveVarStr(path string, data RenderData) string {
	val := resolveVar(strings.TrimSpace(path), data)
	return fmt.Sprintf("%v", val)
}

// toSlice converts an interface to a slice.
func toSlice(v interface{}) ([]interface{}, bool) {
	switch val := v.(type) {
	case []interface{}:
		return val, true
	case []string:
		result := make([]interface{}, len(val))
		for i, s := range val {
			result[i] = s
		}
		return result, true
	case []int:
		result := make([]interface{}, len(val))
		for i, n := range val {
			result[i] = n
		}
		return result, true
	default:
		return nil, false
	}
}

// ---------------------------------------------------------------------------
// Built-in Helper Functions
// ---------------------------------------------------------------------------

func helperUpper(args []string, data RenderData) string {
	if len(args) == 0 {
		return ""
	}
	return strings.ToUpper(resolveVarStr(args[0], data))
}

func helperLower(args []string, data RenderData) string {
	if len(args) == 0 {
		return ""
	}
	return strings.ToLower(resolveVarStr(args[0], data))
}

func helperTruncate(args []string, data RenderData) string {
	if len(args) == 0 {
		return ""
	}
	s := resolveVarStr(args[0], data)
	maxLen := 100
	if len(args) > 1 {
		fmt.Sscanf(args[1], "%d", &maxLen)
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func helperDate(args []string, data RenderData) string {
	layout := time.RFC3339
	if len(args) > 0 {
		layout = args[0]
	}
	return time.Now().Format(layout)
}

func helperJoin(args []string, data RenderData) string {
	if len(args) == 0 {
		return ""
	}
	val := resolveVar(args[0], data)
	slice, ok := toSlice(val)
	if !ok {
		return resolveVarStr(args[0], data)
	}
	sep := ", "
	if len(args) > 1 {
		sep = args[1]
	}
	strs := make([]string, len(slice))
	for i, v := range slice {
		strs[i] = fmt.Sprintf("%v", v)
	}
	return strings.Join(strs, sep)
}

func helperDefault(args []string, data RenderData) string {
	if len(args) == 0 {
		return ""
	}
	val := resolveVarStr(args[0], data)
	if val != "" && val != "<nil>" {
		return val
	}
	if len(args) > 1 {
		// Try resolving as variable first, fall back to literal
		fallback := resolveVarStr(args[1], data)
		if fallback != "" && fallback != "<nil>" {
			return fallback
		}
		return args[1]
	}
	return ""
}