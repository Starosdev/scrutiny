package omnibus

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Runner executes one configured collector.
type Runner interface {
	Run(context.Context, *CollectorSpec, CollectorConfig, string) error
}

// ExecRunner runs bundled collector executables as child processes.
type ExecRunner struct {
	Stdout io.Writer
	Stderr io.Writer
}

// Run starts one collector in run-once mode.
func (r ExecRunner) Run(ctx context.Context, spec *CollectorSpec, cfg CollectorConfig, binaryDir string) error {
	binaryPath := filepath.Join(binaryDir, executableName(spec.BinaryName))
	args := append(OneShotArgs(spec.BinaryName), "--config", cfg.ConfigPath)
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr
	cmd.Env = childEnvironment()
	return cmd.Run()
}

func childEnvironment() []string {
	suppressed := make(map[string]struct{})
	for _, spec := range CollectorSpecs {
		for _, name := range spec.SuppressSchedule {
			suppressed[name] = struct{}{}
		}
	}

	env := make([]string, 0, len(os.Environ())+1)
	for _, item := range os.Environ() {
		name, _, _ := strings.Cut(item, "=")
		if _, skip := suppressed[name]; skip {
			continue
		}
		env = append(env, item)
	}
	env = append(env, "SCRUTINY_SUPPRESS_STARTUP_BANNER=true")
	return env
}

// OneShotArgs selects a single collection even when SMART scheduling is configured.
// Other collectors do not implement the cron-schedule flag.
func OneShotArgs(binaryName string) []string {
	if binaryName == "scrutiny-collector-metrics" {
		return []string{"run", "--cron-schedule="}
	}
	return []string{"run"}
}
