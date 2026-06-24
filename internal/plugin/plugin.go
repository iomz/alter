package plugin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const ManifestFileName = "alter.plugin.toml"

type Manifest struct {
	Plugin   PluginSection   `json:"plugin" toml:"plugin"`
	Upstream UpstreamSection `json:"upstream" toml:"upstream"`
	Runtime  RuntimeSection  `json:"runtime" toml:"runtime"`
	MCP      MCPSection      `json:"mcp" toml:"mcp"`
}

type PluginSection struct {
	Name        string `json:"name" toml:"name"`
	Description string `json:"description" toml:"description"`
	Maintainer  string `json:"maintainer" toml:"maintainer"`
	Entrypoint  string `json:"entrypoint" toml:"entrypoint"`
}

type UpstreamSection struct {
	Name       string `json:"name" toml:"name"`
	Repository string `json:"repository" toml:"repository"`
}

type RuntimeSection struct {
	Manager string `json:"manager" toml:"manager"`
}

type MCPSection struct {
	Enabled   bool   `json:"enabled" toml:"enabled"`
	Namespace string `json:"namespace" toml:"namespace"`
}

type Plugin struct {
	Path     string   `json:"path"`
	Manifest Manifest `json:"manifest"`
}

type DoctorReport struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Manifest   string   `json:"manifest"`
	Status     string   `json:"status"`
	Entrypoint string   `json:"entrypoint,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
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
	entries, err := os.ReadDir(s.pluginsDir())
	if err != nil {
		return nil, fmt.Errorf("read plugins directory: %w", err)
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
	if err := validatePluginName(name); err != nil {
		return Plugin{}, err
	}
	path := filepath.Join(s.pluginsDir(), name)
	manifestPath := filepath.Join(path, ManifestFileName)
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return Plugin{}, err
	}
	if err := manifest.Validate(name); err != nil {
		return Plugin{}, fmt.Errorf("%s: %w", manifestPath, err)
	}
	return Plugin{Path: path, Manifest: manifest}, nil
}

func (s *Store) Doctor(name string) (DoctorReport, error) {
	p, err := s.Load(name)
	if err != nil {
		return DoctorReport{}, err
	}

	report := DoctorReport{
		Name:       p.Manifest.Plugin.Name,
		Path:       p.Path,
		Manifest:   filepath.Join(p.Path, ManifestFileName),
		Status:     "ok",
		Entrypoint: p.Manifest.Plugin.Entrypoint,
	}

	entrypointPath := filepath.Join(p.Path, p.Manifest.Plugin.Entrypoint)
	if _, err := os.Stat(entrypointPath); errors.Is(err, os.ErrNotExist) {
		report.Warnings = append(report.Warnings, fmt.Sprintf("entrypoint %q does not exist yet", p.Manifest.Plugin.Entrypoint))
	} else if err != nil {
		return DoctorReport{}, fmt.Errorf("inspect plugin entrypoint: %w", err)
	}

	return report, nil
}

func (s *Store) pluginsDir() string {
	return filepath.Join(s.root, "plugins")
}

func readManifest(path string) (Manifest, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read plugin manifest: %w", err)
	}
	var manifest Manifest
	if err := toml.Unmarshal(body, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse plugin manifest: %w", err)
	}
	return manifest, nil
}

func (m Manifest) Validate(expectedName string) error {
	var missing []string
	if m.Plugin.Name == "" {
		missing = append(missing, "plugin.name")
	}
	if m.Plugin.Description == "" {
		missing = append(missing, "plugin.description")
	}
	if m.Plugin.Entrypoint == "" {
		missing = append(missing, "plugin.entrypoint")
	}
	if m.Runtime.Manager == "" {
		missing = append(missing, "runtime.manager")
	}
	if len(missing) > 0 {
		return fmt.Errorf("manifest missing required fields: %s", strings.Join(missing, ", "))
	}
	if m.Plugin.Name != expectedName {
		return fmt.Errorf("plugin path %q must match manifest plugin.name %q", expectedName, m.Plugin.Name)
	}
	if m.Runtime.Manager != "mise" {
		return fmt.Errorf("unsupported runtime manager %q", m.Runtime.Manager)
	}
	if m.MCP.Enabled && m.MCP.Namespace == "" {
		return errors.New("manifest requires mcp.namespace when mcp.enabled is true")
	}
	return nil
}

func validatePluginName(name string) error {
	if name == "" {
		return errors.New("plugin name is required")
	}
	if strings.Contains(name, "/") || strings.Contains(name, string(filepath.Separator)) || strings.Contains(name, "..") {
		return fmt.Errorf("invalid plugin name %q", name)
	}
	return nil
}
