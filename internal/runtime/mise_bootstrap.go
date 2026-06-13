package runtime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const miseInstallURL = "https://mise.run"

type MiseBootstrapper struct {
	installPath func() (string, error)
	download    func(context.Context, string) ([]byte, error)
	runScript   func(context.Context, string, string, io.Writer, io.Writer) error
	mkdirAll    func(string, os.FileMode) error
	writeFile   func(string, []byte, os.FileMode) error
	remove      func(string) error
	tempDir     string
	stdout      io.Writer
	stderr      io.Writer
}

func NewMiseBootstrapper(stdout, stderr io.Writer) *MiseBootstrapper {
	resolver := NewMiseResolver()
	return &MiseBootstrapper{
		installPath: resolver.ManagedInstallPath,
		download:    downloadMiseInstaller,
		runScript:   runMiseInstallerScript,
		mkdirAll:    os.MkdirAll,
		writeFile:   os.WriteFile,
		remove:      os.Remove,
		tempDir:     os.TempDir(),
		stdout:      stdout,
		stderr:      stderr,
	}
}

func (b *MiseBootstrapper) Install(ctx context.Context) (string, error) {
	target, err := b.installPath()
	if err != nil {
		return "", err
	}
	if err := b.mkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", fmt.Errorf("create mise install directory: %w", err)
	}

	script, err := b.download(ctx, miseInstallURL)
	if err != nil {
		return "", err
	}

	scriptPath := filepath.Join(b.tempDir, fmt.Sprintf("alter-mise-install-%d.sh", time.Now().UnixNano()))
	if err := b.writeFile(scriptPath, script, 0o600); err != nil {
		return "", fmt.Errorf("write mise installer script: %w", err)
	}
	defer b.remove(scriptPath)

	if err := b.runScript(ctx, scriptPath, target, b.stdout, b.stderr); err != nil {
		return "", err
	}
	if err := verifyManagedMise(target); err != nil {
		return "", err
	}
	return target, nil
}

func (r *MisePathResolver) ManagedInstallPath() (string, error) {
	home, err := r.homeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if home == "" {
		return "", fmt.Errorf("resolve home directory: empty path")
	}
	return r.abs(filepath.Join(home, ".local", "share", "alter", "bin", "mise"))
}

func downloadMiseInstaller(ctx context.Context, url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create mise installer request: %w", err)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download mise installer: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("download mise installer: unexpected status %s", res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("read mise installer: %w", err)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("download mise installer: empty response")
	}
	return body, nil
}

func runMiseInstallerScript(ctx context.Context, scriptPath, target string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "sh", scriptPath)
	cmd.Env = append(os.Environ(), "MISE_INSTALL_PATH="+target)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run mise installer: %w", err)
	}
	return nil
}

func verifyManagedMise(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("verify installed mise at %q: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("verify installed mise at %q: path is a directory", path)
	}
	if info.Mode()&0o111 == 0 {
		return fmt.Errorf("verify installed mise at %q: path is not executable", path)
	}
	return nil
}
