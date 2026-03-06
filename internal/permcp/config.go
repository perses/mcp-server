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

package permcp

import (
	"fmt"
	"strings"

	commonconfig "github.com/perses/common/config"
	"github.com/perses/common/set"
	"github.com/perses/mcp-server/pkg/tools"
	"github.com/perses/perses/pkg/client/config"
	"github.com/perses/perses/pkg/model/api/v1/common"
)

type Transport string

const (
	HTTPTransport  Transport = "http"
	STDIOTransport Transport = "stdio"
)

func ResolveConfig(configFile string) (Config, error) {
	c := Config{}
	return c, commonconfig.NewResolver[Config]().
		SetEnvPrefix("PERMCP").
		SetConfigFile(configFile).
		Resolve(&c).
		Verify()
}

type Config struct {
	// Transport mechanism for the MCP server (e.g., "stdio", "http")
	Transport Transport `yaml:"transport,omitempty"`

	// ListenAddress is the address to listen on for HTTP transport (e.g., ":8000")
	ListenAddress string `yaml:"listen_address,omitempty"`

	// ReadOnly indicates if the server should operate in read-only mode
	ReadOnly bool `yaml:"read_only,omitempty"`

	// Resources is a comma-separated list of resources to register.
	Resources string `yaml:"resources,omitempty"`

	// AllowedResources is the normalized list of resources to register.
	AllowedResources []string `yaml:"-"`

	// PersesServer is the configuration for connecting to the Perses backend server.
	// Supports multiple authentication methods: Authorization (Bearer token),
	// OAuth, BasicAuth, K8sAuth, and NativeAuth.
	PersesServer config.RestConfigClient `yaml:"perses_server"`
}

func (c *Config) Verify() error {
	if c.Transport == "" {
		c.Transport = STDIOTransport
	}

	switch strings.ToLower(strings.TrimSpace(string(c.Transport))) {
	case "stdio":
		c.Transport = "stdio"
	case "http", "http-streamable", "streamable-http":
		c.Transport = HTTPTransport
	default:
		return fmt.Errorf("unsupported transport %q. valid values are: stdio, http", c.Transport)
	}

	if c.PersesServer.URL == nil {
		c.PersesServer.URL = common.MustParseURL("http://localhost:8080")
	}

	if c.ListenAddress == "" {
		c.ListenAddress = ":8000"
	} else if !strings.Contains(c.ListenAddress, ":") {
		c.ListenAddress = ":" + c.ListenAddress
	}

	if err := c.PersesServer.Validate(); err != nil {
		return fmt.Errorf("invalid perses client configuration: %w", err)
	}

	c.Resources = strings.TrimSpace(c.Resources)
	c.parseAllowedResources()

	return c.validateAllowedResources()
}

func (c *Config) validateAllowedResources() error {
	validSet := set.New(tools.ValidResources...)
	var invalid []string
	for _, rs := range c.AllowedResources {
		if !validSet.Contains(tools.Resource(rs)) {
			invalid = append(invalid, rs)
		}
	}

	if len(invalid) > 0 {
		validNames := make([]string, len(tools.ValidResources))
		for i, r := range tools.ValidResources {
			validNames[i] = string(r)
		}
		return fmt.Errorf("invalid resource(s): %s. Valid resources are: %s",
			strings.Join(invalid, ", "),
			strings.Join(validNames, ", "))
	}
	return nil
}

func (c *Config) parseAllowedResources() {
	for resource := range strings.SplitSeq(c.Resources, ",") {
		resource = strings.TrimSpace(resource)
		if resource == "" {
			continue
		}
		c.AllowedResources = append(c.AllowedResources, strings.ToLower(resource))
	}
}
