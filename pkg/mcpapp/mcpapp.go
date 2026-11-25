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

// Package mcpapp implements the server side of the MCP Apps extension
// (https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/2026-01-26/apps.mdx): the metadata that
// links a tool to its view (a ui:// resource) and the tools that only the views can call.
package mcpapp

import (
	"context"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// ExtensionID identifies the MCP Apps extension in the client capabilities.
	ExtensionID = "io.modelcontextprotocol/ui"
	// ResourceMIMEType is the MIME type of the HTML views.
	ResourceMIMEType = "text/html;profile=mcp-app"
)

// Visibility tells who can see and call a tool: the model, the views (app), or both (the default).
type Visibility string

const (
	VisibilityModel Visibility = "model"
	VisibilityApp   Visibility = "app"
)

// ToolMeta returns the _meta of a tool: the URI of its view (empty for none) and its visibility (default for none).
func ToolMeta(resourceURI string, visibility ...Visibility) mcp.Meta {
	ui := map[string]any{}
	if resourceURI != "" {
		ui["resourceUri"] = resourceURI
	}
	if len(visibility) > 0 {
		ui["visibility"] = visibility
	}
	return mcp.Meta{"ui": ui}
}

// IsAppOnly reports whether the visibility of a tool doesn't include the model.
func IsAppOnly(tool *mcp.Tool) bool {
	ui, ok := tool.Meta["ui"].(map[string]any)
	if !ok {
		return false
	}
	var visibility []string
	switch v := ui["visibility"].(type) {
	case []Visibility:
		for _, item := range v {
			visibility = append(visibility, string(item))
		}
	case []string:
		visibility = v
	case []any:
		for _, item := range v {
			if s, isString := item.(string); isString {
				visibility = append(visibility, s)
			}
		}
	default:
		return false
	}
	return !slices.Contains(visibility, string(VisibilityModel))
}

// Supported reports whether a client supports MCP Apps: it declares the extension, with the MIME type of the HTML
// views when it lists MIME types.
func Supported(capabilities *mcp.ClientCapabilities) bool {
	if capabilities == nil {
		return false
	}
	settings, declared := capabilities.Extensions[ExtensionID]
	if !declared {
		return false
	}
	fields, isMap := settings.(map[string]any)
	if !isMap {
		return true
	}
	mimeTypes, listed := fields["mimeTypes"].([]any)
	if !listed {
		return true
	}
	return slices.Contains(mimeTypes, any(ResourceMIMEType))
}

// HideAppOnlyTools is a receiving middleware that removes the app-only tools from tools/list for the clients that don't
// declare MCP Apps: a client that doesn't know the visibility of the tools would show these tools to the model. The
// tools can still be called: the views of some hosts that don't declare the extension call them.
func HideAppOnlyTools(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		result, err := next(ctx, method, req)
		if err != nil || method != "tools/list" {
			return result, err
		}
		list, isList := result.(*mcp.ListToolsResult)
		listRequest, isListRequest := req.(*mcp.ListToolsRequest)
		if !isList || !isListRequest || Supported(listRequest.ClientCapabilities()) {
			return result, nil
		}
		filtered := *list
		filtered.Tools = slices.DeleteFunc(slices.Clone(list.Tools), IsAppOnly)
		return &filtered, nil
	}
}
