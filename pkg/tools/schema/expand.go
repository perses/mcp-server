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
)

const (
	// pluginDefinitionPrefix prefixes the plugin definitions of the dashboard schema: #Plugin_<Type>_<Kind>.
	pluginDefinitionPrefix = "#Plugin_"
	pluginBlocksTTL        = 5 * time.Minute
)

// pluginBlocks caches the self-contained plugin definitions of the dashboard schema, in which the shared types
// (calculation, format, thresholds, datasource selector...) are inlined. They are refreshed lazily once expired.
type pluginBlocks struct {
	fetch func() ([]byte, error)
	now   func() time.Time
	ttl   time.Duration

	mutex     sync.Mutex
	blocks    map[string][]byte
	fetchedAt time.Time
}

func newPluginBlocks(fetch func() ([]byte, error)) *pluginBlocks {
	return &pluginBlocks{fetch: fetch, now: time.Now, ttl: pluginBlocksTTL}
}

// get returns the expanded schema of the plugin kind and whether it was found.
func (p *pluginBlocks) get(kind string) ([]byte, bool, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.blocks == nil || p.now().Sub(p.fetchedAt) >= p.ttl {
		source, err := p.fetch()
		if err != nil {
			return nil, false, fmt.Errorf("unable to retrieve the dashboard schema: %w", err)
		}
		blocks, err := extractPluginBlocks(source)
		if err != nil {
			return nil, false, err
		}
		p.blocks = blocks
		p.fetchedAt = p.now()
	}
	block, ok := p.blocks[kind]
	return block, ok, nil
}

// extractPluginBlocks parses the dashboard schema and returns each top-level #Plugin_<Type>_<Kind> definition,
// renamed #<Kind> and preceded by the imports it uses, indexed by kind.
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
	for _, decl := range file.Decls {
		field, ok := decl.(*ast.Field)
		if !ok {
			continue
		}
		label, ok := field.Label.(*ast.Ident)
		if !ok || !strings.HasPrefix(label.Name, pluginDefinitionPrefix) {
			continue
		}
		_, kind, ok := strings.Cut(strings.TrimPrefix(label.Name, pluginDefinitionPrefix), "_")
		if !ok || kind == "" {
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

func formatBlock(kind string, field *ast.Field, imports map[string]*ast.ImportSpec) ([]byte, error) {
	renamed := &ast.Field{Label: ast.NewIdent("#" + kind), Value: field.Value}
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
