// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schema

import (
	"bytes"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/perses/spec/go/plugin"
)

const (
	// pluginDefinitionPrefix prefixes the plugin definitions of the dashboard schema: #Plugin_<Type>_<Kind>.
	pluginDefinitionPrefix = "#Plugin_"
	pluginBlocksTTL        = 5 * time.Minute
)

// pluginBlocks caches the self-contained plugin definitions of the dashboard schema, in which the shared types
// (calculation, format, thresholds, datasource selector...) are inlined. They are refreshed lazily once expired.
type pluginBlocks struct {
	fetchSchema  func() ([]byte, error)
	fetchModules func() ([]v1.PluginModule, error)
	now          func() time.Time
	ttl          time.Duration

	mutex     sync.Mutex
	blocks    map[string][]byte
	fetchedAt time.Time
}

func newPluginBlocks(fetchSchema func() ([]byte, error), fetchModules func() ([]v1.PluginModule, error)) *pluginBlocks {
	return &pluginBlocks{fetchSchema: fetchSchema, fetchModules: fetchModules, now: time.Now, ttl: pluginBlocksTTL}
}

// get returns the expanded schema of the plugin kind and whether it was found.
func (p *pluginBlocks) get(kind string) ([]byte, bool, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.blocks == nil || p.now().Sub(p.fetchedAt) >= p.ttl {
		source, err := p.fetchSchema()
		if err != nil {
			return nil, false, fmt.Errorf("unable to retrieve the dashboard schema: %w", err)
		}
		blocks, err := extractPluginBlocks(source)
		if err != nil {
			return nil, false, err
		}
		modules, err := p.fetchModules()
		if err != nil {
			return nil, false, fmt.Errorf("unable to retrieve the plugin modules: %w", err)
		}
		keepDefaultRegistryOnly(blocks, modules)
		p.blocks = blocks
		p.fetchedAt = p.now()
	}
	block, ok := p.blocks[kind]
	return block, ok, nil
}

// keepDefaultRegistryOnly removes the blocks of the kinds not provided solely by the default registry. The dashboard
// schema keeps one schema per kind whatever its registry, so it can only stand for the default registry's schema
// when no other registry provides the kind.
func keepDefaultRegistryOnly(blocks map[string][]byte, modules []v1.PluginModule) {
	inDefaultRegistry := map[string]bool{}
	inOtherRegistry := map[string]bool{}
	for _, module := range modules {
		isDefault := module.Metadata.Registry == "" || module.Metadata.Registry == plugin.DefaultRegistry
		for _, p := range module.Spec.Plugins {
			if isDefault {
				inDefaultRegistry[p.Spec.Name] = true
			} else {
				inOtherRegistry[p.Spec.Name] = true
			}
		}
	}
	for kind := range blocks {
		if !inDefaultRegistry[kind] || inOtherRegistry[kind] {
			delete(blocks, kind)
		}
	}
}

// extractPluginBlocks parses the dashboard schema and returns each top-level #Plugin_<Type>_<Name> definition,
// renamed #<Kind> and preceded by the imports it uses, indexed by kind. The kind is read from the definition's kind
// field, as Perses sanitizes and may suffix the name in the definition label. Definitions whose kind can't be
// resolved, or whose kind is shared by several definitions, are skipped.
func extractPluginBlocks(source []byte) (map[string][]byte, error) {
	file, err := parser.ParseFile("dashboards.cue", source)
	if err != nil {
		return nil, fmt.Errorf("unable to parse the dashboard schema: %w", err)
	}
	imports := map[string]*ast.ImportSpec{}
	for spec := range file.ImportSpecs() {
		imports[importName(spec)] = spec
	}

	blocks := map[string][]byte{}
	ambiguous := map[string]bool{}
	for _, decl := range file.Decls {
		field, ok := decl.(*ast.Field)
		if !ok {
			continue
		}
		label, ok := field.Label.(*ast.Ident)
		if !ok || !strings.HasPrefix(label.Name, pluginDefinitionPrefix) {
			continue
		}
		kind, ok := pluginKind(field.Value)
		if !ok || ambiguous[kind] {
			continue
		}
		if _, exists := blocks[kind]; exists {
			delete(blocks, kind)
			ambiguous[kind] = true
			continue
		}
		block, err := formatBlock(kind, field, imports)
		if err != nil {
			return nil, fmt.Errorf("unable to format the schema of plugin %q: %w", kind, err)
		}
		blocks[kind] = block
	}
	return blocks, nil
}

// pluginKind returns the value of the kind field of a plugin definition, either a string literal (kind: "StatChart")
// or a reference to a sibling field holding one (#kind: "PrometheusDatasource", kind: #kind).
func pluginKind(value ast.Expr) (string, bool) {
	st, ok := value.(*ast.StructLit)
	if !ok {
		return "", false
	}
	fields := map[string]ast.Expr{}
	for _, decl := range st.Elts {
		if field, ok := decl.(*ast.Field); ok {
			if name, _, err := ast.LabelName(field.Label); err == nil {
				fields[name] = field.Value
			}
		}
	}
	kind := fields["kind"]
	if ident, ok := kind.(*ast.Ident); ok {
		kind = fields[ident.Name]
	}
	literal, ok := kind.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	name, err := strconv.Unquote(literal.Value)
	if err != nil || name == "" || !kindPattern.MatchString(name) {
		return "", false
	}
	return name, true
}

func formatBlock(kind string, field *ast.Field, imports map[string]*ast.ImportSpec) ([]byte, error) {
	// Kinds that aren't CUE identifiers (e.g. my-chart) keep the definition name of the dashboard schema.
	label := field.Label
	if name := "#" + kind; ast.IsValidIdent(name) {
		label = ast.NewIdent(name)
	}
	renamed := &ast.Field{Label: label, Value: field.Value}
	body, err := format.Node(renamed)
	if err != nil {
		return nil, err
	}

	var used []string
	for name := range referencedPackages(field.Value) {
		if spec, ok := imports[name]; ok {
			if spec.Name != nil {
				used = append(used, spec.Name.Name+" "+spec.Path.Value)
			} else {
				used = append(used, spec.Path.Value)
			}
		}
	}
	var buffer bytes.Buffer
	switch len(used) {
	case 0:
	case 1:
		fmt.Fprintf(&buffer, "import %s\n\n", used[0])
	default:
		slices.Sort(used)
		buffer.WriteString("import (\n")
		for _, spec := range used {
			fmt.Fprintf(&buffer, "\t%s\n", spec)
		}
		buffer.WriteString(")\n\n")
	}
	buffer.Write(body)
	buffer.WriteString("\n")
	return buffer.Bytes(), nil
}

// referencedPackages returns the identifiers used as the operand of a selector, e.g. strings in strings.MinRunes.
func referencedPackages(node ast.Node) map[string]bool {
	names := map[string]bool{}
	ast.Walk(node, func(n ast.Node) bool {
		if selector, ok := n.(*ast.SelectorExpr); ok {
			if ident, ok := selector.X.(*ast.Ident); ok {
				names[ident.Name] = true
			}
		}
		return true
	}, nil)
	return names
}

func importName(spec *ast.ImportSpec) string {
	if spec.Name != nil {
		return spec.Name.Name
	}
	importPath, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		importPath = spec.Path.Value
	}
	if _, packageName, ok := strings.Cut(importPath, ":"); ok {
		return packageName
	}
	return path.Base(importPath)
}
