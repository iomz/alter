package trust

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/iomz/alter/internal/plugin"
)

const StoreRelativePath = ".local/state/alter/trust/plugins.json"

type Status string

const (
	StatusTrusted     Status = "trusted"
	StatusUntrusted   Status = "untrusted"
	StatusInvalidated Status = "invalidated"
	StatusNotRequired Status = "not-required"
)

type Record struct {
	Name             string `json:"name"`
	WorkspacePath    string `json:"workspace_path"`
	ManifestHash     string `json:"manifest_hash"`
	MiseHash         string `json:"alter_mise_toml_hash,omitempty"`
	ToolVersionsHash string `json:"alter_tool_versions_hash,omitempty"`
	EntrypointHash   string `json:"adapter_entrypoint_hash,omitempty"`
	TrustedAt        string `json:"trusted_at"`
}

type Fingerprints struct {
	Name             string
	WorkspacePath    string
	ManifestPath     string
	MisePath         string
	ToolVersionsPath string
	EntrypointPath   string
	ManifestHash     string
	MiseHash         string
	ToolVersionsHash string
	EntrypointHash   string
	HasMise          bool
	HasToolVersions  bool
	HasEntrypoint    bool
}

type Evaluation struct {
	Status      Status
	Reason      string
	StorePath   string
	Current     Fingerprints
	Stored      *Record
	Mismatches  []string
	FilesHashed []string
}

type storeFile struct {
	Plugins []Record `json:"plugins"`
}

func Evaluate(p plugin.Plugin) (Evaluation, error) {
	return evaluate(p)
}

func Trust(p plugin.Plugin) (Record, string, error) {
	return trustPlugin(p, time.Now)
}

func Untrust(name string) (bool, string, error) {
	storePath, err := defaultStorePath()
	if err != nil {
		return false, "", err
	}
	store, err := readStore(storePath)
	if err != nil {
		return false, "", err
	}
	removed := false
	next := store.Plugins[:0]
	for _, record := range store.Plugins {
		if record.Name == name {
			removed = true
			continue
		}
		next = append(next, record)
	}
	store.Plugins = next
	if err := writeStore(storePath, store); err != nil {
		return false, "", err
	}
	return removed, storePath, nil
}

func evaluate(p plugin.Plugin) (Evaluation, error) {
	storePath, err := defaultStorePath()
	if err != nil {
		return Evaluation{}, err
	}
	current, filesHashed, err := fingerprintPlugin(p)
	if err != nil {
		return Evaluation{}, err
	}
	store, err := readStore(storePath)
	if err != nil {
		return Evaluation{}, err
	}
	eval := Evaluation{
		Status:      StatusUntrusted,
		Reason:      "no trust record",
		StorePath:   storePath,
		Current:     current,
		FilesHashed: filesHashed,
	}
	for _, record := range store.Plugins {
		if record.Name != p.Manifest.Plugin.Name {
			continue
		}
		eval.Stored = &record
		mismatches := compare(record, current)
		if len(mismatches) > 0 {
			eval.Status = StatusInvalidated
			eval.Mismatches = mismatches
			eval.Reason = strings.Join(mismatches, "; ")
			return eval, nil
		}
		eval.Status = StatusTrusted
		eval.Reason = "trusted fingerprints match"
		return eval, nil
	}
	return eval, nil
}

func trustPlugin(p plugin.Plugin, now func() time.Time) (Record, string, error) {
	storePath, err := defaultStorePath()
	if err != nil {
		return Record{}, "", err
	}
	current, _, err := fingerprintPlugin(p)
	if err != nil {
		return Record{}, "", err
	}
	record := Record{
		Name:             current.Name,
		WorkspacePath:    current.WorkspacePath,
		ManifestHash:     current.ManifestHash,
		MiseHash:         current.MiseHash,
		ToolVersionsHash: current.ToolVersionsHash,
		EntrypointHash:   current.EntrypointHash,
		TrustedAt:        now().UTC().Format(time.RFC3339),
	}
	store, err := readStore(storePath)
	if err != nil {
		return Record{}, "", err
	}
	replaced := false
	for i := range store.Plugins {
		if store.Plugins[i].Name == record.Name {
			store.Plugins[i] = record
			replaced = true
			break
		}
	}
	if !replaced {
		store.Plugins = append(store.Plugins, record)
	}
	sort.Slice(store.Plugins, func(i, j int) bool {
		return store.Plugins[i].Name < store.Plugins[j].Name
	})
	if err := writeStore(storePath, store); err != nil {
		return Record{}, "", err
	}
	return record, storePath, nil
}

func compare(record Record, current Fingerprints) []string {
	checks := [][3]string{
		{"workspace path", record.WorkspacePath, current.WorkspacePath},
		{"manifest hash", record.ManifestHash, current.ManifestHash},
		{"alter.mise.toml hash", record.MiseHash, current.MiseHash},
		{"alter.tool-versions hash", record.ToolVersionsHash, current.ToolVersionsHash},
		{"adapter entrypoint hash", record.EntrypointHash, current.EntrypointHash},
	}
	var mismatches []string
	for _, check := range checks {
		if check[1] != check[2] {
			mismatches = append(mismatches, fmt.Sprintf("%s changed", check[0]))
		}
	}
	return mismatches
}

func fingerprintPlugin(p plugin.Plugin) (Fingerprints, []string, error) {
	workspacePath, err := filepath.Abs(p.Path)
	if err != nil {
		return Fingerprints{}, nil, fmt.Errorf("resolve plugin workspace: %w", err)
	}
	workspacePath = filepath.Clean(workspacePath)
	current := Fingerprints{
		Name:             p.Manifest.Plugin.Name,
		WorkspacePath:    workspacePath,
		ManifestPath:     filepath.Join(workspacePath, plugin.ManifestFileName),
		MisePath:         filepath.Join(workspacePath, "alter.mise.toml"),
		ToolVersionsPath: filepath.Join(workspacePath, "alter.tool-versions"),
	}
	var filesHashed []string
	hash, ok, err := hashIfExists(current.ManifestPath)
	if err != nil {
		return Fingerprints{}, nil, err
	}
	if !ok {
		return Fingerprints{}, nil, fmt.Errorf("trust fingerprint requires manifest %q", current.ManifestPath)
	}
	current.ManifestHash = hash
	filesHashed = append(filesHashed, current.ManifestPath)

	current.MiseHash, current.HasMise, err = hashIfExists(current.MisePath)
	if err != nil {
		return Fingerprints{}, nil, err
	}
	if current.HasMise {
		filesHashed = append(filesHashed, current.MisePath)
	}
	current.ToolVersionsHash, current.HasToolVersions, err = hashIfExists(current.ToolVersionsPath)
	if err != nil {
		return Fingerprints{}, nil, err
	}
	if current.HasToolVersions {
		filesHashed = append(filesHashed, current.ToolVersionsPath)
	}

	entrypointPath := filepath.Clean(filepath.Join(workspacePath, p.Manifest.Plugin.Entrypoint))
	if isInsideWorkspace(workspacePath, entrypointPath) {
		current.EntrypointPath = entrypointPath
		current.EntrypointHash, current.HasEntrypoint, err = hashIfExists(entrypointPath)
		if err != nil {
			return Fingerprints{}, nil, err
		}
		if current.HasEntrypoint {
			filesHashed = append(filesHashed, entrypointPath)
		}
	}
	return current, filesHashed, nil
}

func hashIfExists(path string) (string, bool, error) {
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("hash %q: %w", path, err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), true, nil
}

func isInsideWorkspace(workspace, path string) bool {
	rel, err := filepath.Rel(workspace, path)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

func defaultStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if home == "" {
		return "", errors.New("resolve home directory: empty path")
	}
	return filepath.Join(home, StoreRelativePath), nil
}

func readStore(path string) (storeFile, error) {
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return storeFile{}, nil
	}
	if err != nil {
		return storeFile{}, fmt.Errorf("read trust store: %w", err)
	}
	var store storeFile
	if err := json.Unmarshal(body, &store); err != nil {
		return storeFile{}, fmt.Errorf("parse trust store: %w", err)
	}
	return store, nil
}

func writeStore(path string, store storeFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create trust store directory: %w", err)
	}
	body, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("encode trust store: %w", err)
	}
	body = append(body, '\n')
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return fmt.Errorf("write trust store: %w", err)
	}
	return nil
}
