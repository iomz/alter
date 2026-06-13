package runtime

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/iomz/alter/internal/plugin"
)

type MiseRunner struct {
	stdout io.Writer
	stderr io.Writer
}

func NewMiseRunner(stdout, stderr io.Writer) *MiseRunner {
	return &MiseRunner{stdout: stdout, stderr: stderr}
}

func (r *MiseRunner) Prepare(p plugin.Plugin) error {
	if err := requireMise(); err != nil {
		return err
	}
	if hasMiseConfig(p.Path) {
		fmt.Fprintf(r.stderr, "warning: plugin %q has mise.toml; review and trust it before running untrusted code\n", p.Manifest.Plugin.Name)
	}
	cmd := exec.Command("mise", "install")
	cmd.Dir = p.Path
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	return cmd.Run()
}

func (r *MiseRunner) Run(p plugin.Plugin, args ...string) ([]byte, error) {
	if p.Manifest.Runtime.Manager != "mise" {
		return nil, fmt.Errorf("unsupported runtime manager %q", p.Manifest.Runtime.Manager)
	}
	if err := requireMise(); err != nil {
		return nil, err
	}
	if hasMiseConfig(p.Path) {
		fmt.Fprintf(r.stderr, "warning: plugin %q has mise.toml; review and trust it before running untrusted code\n", p.Manifest.Plugin.Name)
	}

	entrypoint := p.Manifest.Plugin.Entrypoint
	if _, err := os.Stat(filepath.Join(p.Path, entrypoint)); err == nil {
		entrypoint = "./" + entrypoint
	}

	cmdArgs := append([]string{"exec", "--", entrypoint}, args...)
	cmd := exec.Command("mise", cmdArgs...)
	cmd.Dir = p.Path
	cmd.Stderr = r.stderr
	return cmd.Output()
}

func requireMise() error {
	if _, err := exec.LookPath("mise"); err != nil {
		return errors.New("mise is required: install mise from https://mise.jdx.dev, then rerun alter")
	}
	return nil
}

func hasMiseConfig(path string) bool {
	_, err := os.Stat(filepath.Join(path, "mise.toml"))
	return err == nil
}
