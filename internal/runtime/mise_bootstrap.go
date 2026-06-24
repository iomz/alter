package runtime

import (
	"bytes"
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
	runScript   func(context.Context, string, string) ([]byte, error)
	mkdirAll    func(string, os.FileMode) error
	createTemp  func(string, string) (*os.File, error)
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
		createTemp:  os.CreateTemp,
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

	scriptFile, err := b.createTemp(b.tempDir, "alter-mise-install-*.sh")
	if err != nil {
		return "", fmt.Errorf("create mise installer script: %w", err)
	}
	scriptPath := scriptFile.Name()
	defer b.remove(scriptPath)
	if _, err := scriptFile.Write(script); err != nil {
		_ = scriptFile.Close()
		return "", fmt.Errorf("write mise installer script: %w", err)
	}
	if err := scriptFile.Close(); err != nil {
		return "", fmt.Errorf("close mise installer script: %w", err)
	}

	output, err := b.runScript(ctx, scriptPath, target)
	if err != nil {
		if len(bytes.TrimSpace(output)) > 0 {
			fmt.Fprintln(b.stderr, "mise installer output:")
			_, _ = b.stderr.Write(output)
			if !bytes.HasSuffix(output, []byte("\n")) {
				fmt.Fprintln(b.stderr)
			}
		}
		return "", err
	}
	if err := verifyManagedMise(target); err != nil {
		return "", err
	}
	return target, nil
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

func runMiseInstallerScript(ctx context.Context, scriptPath, target string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "sh", scriptPath)
	cmd.Env = append(os.Environ(), "MISE_INSTALL_PATH="+target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("run mise installer: %w", err)
	}
	return output, nil
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
