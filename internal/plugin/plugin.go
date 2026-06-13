package plugin

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Manifest struct {
	Plugin   PluginSection   `json:"plugin"`
	Upstream UpstreamSection `json:"upstream"`
	Runtime  RuntimeSection  `json:"runtime"`
	MCP      MCPSection      `json:"mcp"`
}

type PluginSection struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Maintainer  string `json:"maintainer"`
	Entrypoint  string `json:"entrypoint"`
}

type UpstreamSection struct {
	Name       string `json:"name"`
	Repository string `json:"repository"`
}

type RuntimeSection struct {
	Manager string `json:"manager"`
}

type MCPSection struct {
	Enabled   bool   `json:"enabled"`
	Namespace string `json:"namespace"`
}

type Plugin struct {
	Path     string   `json:"path"`
	Manifest Manifest `json:"manifest"`
}

type Store struct {
	root string
}

func NewStore(root string) *Store {
	return &Store{root: root}
}

func FindRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "plugins")); err == nil {
			return dir, nil
		}
		if filepath.Dir(dir) == dir {
			return "", errors.New("could not find repo root containing plugins/")
		}
	}
}

func (s *Store) List() ([]Plugin, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "plugins"))
	if err != nil {
		return nil, err
	}
	var plugins []Plugin
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		p, err := s.Load(entry.Name())
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, p)
	}
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Manifest.Plugin.Name < plugins[j].Manifest.Plugin.Name
	})
	return plugins, nil
}

func (s *Store) Load(name string) (Plugin, error) {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		return Plugin{}, fmt.Errorf("invalid plugin name %q", name)
	}
	path := filepath.Join(s.root, "plugins", name)
	manifest, err := readManifest(filepath.Join(path, "alter.plugin.toml"))
	if err != nil {
		return Plugin{}, err
	}
	if manifest.Plugin.Name != name {
		return Plugin{}, fmt.Errorf("plugin path %q must match manifest name %q", name, manifest.Plugin.Name)
	}
	return Plugin{Path: path, Manifest: manifest}, nil
}

func readManifest(path string) (Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, err
	}
	defer file.Close()

	var manifest Manifest
	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return Manifest{}, fmt.Errorf("invalid manifest line %q", line)
		}
		setManifestValue(&manifest, section, strings.TrimSpace(key), parseValue(strings.TrimSpace(value)))
	}
	if err := scanner.Err(); err != nil {
		return Manifest{}, err
	}
	if manifest.Plugin.Name == "" || manifest.Plugin.Entrypoint == "" {
		return Manifest{}, errors.New("manifest requires plugin.name and plugin.entrypoint")
	}
	if manifest.Runtime.Manager == "" {
		manifest.Runtime.Manager = "mise"
	}
	return manifest, nil
}

func parseValue(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		return strings.TrimSuffix(strings.TrimPrefix(value, "\""), "\"")
	}
	return value
}

func setManifestValue(manifest *Manifest, section, key, value string) {
	switch section + "." + key {
	case "plugin.name":
		manifest.Plugin.Name = value
	case "plugin.description":
		manifest.Plugin.Description = value
	case "plugin.maintainer":
		manifest.Plugin.Maintainer = value
	case "plugin.entrypoint":
		manifest.Plugin.Entrypoint = value
	case "upstream.name":
		manifest.Upstream.Name = value
	case "upstream.repository":
		manifest.Upstream.Repository = value
	case "runtime.manager":
		manifest.Runtime.Manager = value
	case "mcp.enabled":
		manifest.MCP.Enabled = value == "true"
	case "mcp.namespace":
		manifest.MCP.Namespace = value
	}
}
