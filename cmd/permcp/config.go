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

package main

import (
	"fmt"
	"os"
	"strings"

	commonconfig "github.com/perses/common/config"

	permcp "github.com/perses/mcp-server/internal/permcp"
)

type commandInput struct {
	ConfigFile string
}

type appConfig struct {
	Transport       string    `yaml:"transport,omitempty"`
	PersesServerURL string    `yaml:"perses_server_url,omitempty"`
	Port            string    `yaml:"port,omitempty"`
	ReadOnly        bool      `yaml:"read_only,omitempty"`
	Resources       string    `yaml:"resources,omitempty"`
	Log             logConfig `yaml:"log,omitempty"`
}

type logConfig struct {
	Level    string `yaml:"level,omitempty"`
	FilePath string `yaml:"file_path,omitempty"`
}

func (c *appConfig) Verify() error {
	if c.Transport == "" {
		c.Transport = "stdio"
	}

	switch strings.ToLower(strings.TrimSpace(c.Transport)) {
	case "stdio":
		c.Transport = "stdio"
	case "http", "http-streamable", "streamable-http":
		c.Transport = "http"
	default:
		return fmt.Errorf("unsupported transport %q. valid values are: stdio, http", c.Transport)
	}

	if c.PersesServerURL == "" {
		c.PersesServerURL = "http://localhost:8080"
	}

	if c.Port == "" {
		c.Port = "8000"
	}

	if c.Log.Level == "" {
		c.Log.Level = "info"
	}

	c.Resources = strings.TrimSpace(c.Resources)

	return nil
}

func resolveConfig(input commandInput) (permcp.Config, error) {
	resolved := appConfig{}
	if err := commonconfig.NewResolver[appConfig]().SetEnvPrefix("PERMCP").SetConfigFile(input.ConfigFile).Resolve(&resolved).Verify(); err != nil {
		return permcp.Config{}, fmt.Errorf("unable to resolve configuration: %w", err)
	}

	token := strings.TrimSpace(os.Getenv(envPersesToken))
	if token == "" {
		return permcp.Config{}, fmt.Errorf("environment variable %s is required", envPersesToken)
	}

	return permcp.Config{
		Transport:        resolved.Transport,
		PersesServerURL:  resolved.PersesServerURL,
		Token:            token,
		ReadOnly:         resolved.ReadOnly,
		LogFilePath:      resolved.Log.FilePath,
		LogLevel:         resolved.Log.Level,
		Port:             resolved.Port,
		AllowedResources: parseAllowedResources(resolved.Resources),
	}, nil
}

func parseAllowedResources(resources string) []string {
	if resources == "" {
		return nil
	}

	allowedResources := make([]string, 0)
	for resource := range strings.SplitSeq(resources, ",") {
		resource = strings.TrimSpace(resource)
		if resource == "" {
			continue
		}
		allowedResources = append(allowedResources, strings.ToLower(resource))
	}

	if len(allowedResources) == 0 {
		return nil
	}

	return allowedResources
}
