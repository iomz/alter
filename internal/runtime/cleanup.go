package runtime

import (
	"errors"
	"fmt"
	"os"
)

type CleanupItem struct {
	Label   string `json:"label"`
	Path    string `json:"path"`
	Removed bool   `json:"removed"`
}

func CleanupManagedMise() ([]CleanupItem, error) {
	return cleanupManagedMise(NewMiseResolver())
}

func cleanupManagedMise(resolver *MisePathResolver) ([]CleanupItem, error) {
	targets, err := cleanupTargets(resolver)
	if err != nil {
		return nil, err
	}
	items := make([]CleanupItem, 0, len(targets))
	for _, target := range targets {
		removed, err := removeCleanupTarget(target.path)
		if err != nil {
			return nil, fmt.Errorf("remove %s %q: %w", target.label, target.path, err)
		}
		items = append(items, CleanupItem{
			Label:   target.label,
			Path:    target.path,
			Removed: removed,
		})
	}
	return items, nil
}

type cleanupTarget struct {
	label string
	path  string
}

func cleanupTargets(resolver *MisePathResolver) ([]cleanupTarget, error) {
	installPath, err := resolver.ManagedInstallPath()
	if err != nil {
		return nil, err
	}
	cacheDir, err := resolver.ManagedCacheDir()
	if err != nil {
		return nil, err
	}
	dataDir, err := resolver.ManagedDataDir()
	if err != nil {
		return nil, err
	}
	configFile, err := resolver.ManagedConfigFile()
	if err != nil {
		return nil, err
	}
	stateDir, err := resolver.ManagedStateDir()
	if err != nil {
		return nil, err
	}
	return []cleanupTarget{
		{label: "binary", path: installPath},
		{label: "data", path: dataDir},
		{label: "config", path: configFile},
		{label: "state", path: stateDir},
		{label: "cache", path: cacheDir},
	}, nil
}

func removeCleanupTarget(path string) (bool, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err := os.RemoveAll(path); err != nil {
		return false, err
	}
	return true, nil
}
