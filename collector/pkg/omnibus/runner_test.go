package omnibus

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecRunnerOneShotInvocation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	t.Setenv("COLLECTOR_CRON_SCHEDULE", "0 0 * * *")
	t.Setenv("COLLECTOR_API_ENDPOINT", "http://example.invalid")
	for _, spec := range CollectorSpecs {
		t.Run(spec.Name, func(t *testing.T) {
			dir := t.TempDir()
			script := "#!/bin/sh\nprintf '%s\\n' \"$@\"\nprintf 'api=%s\\n' \"$COLLECTOR_API_ENDPOINT\"\nprintf 'schedule=%s\\n' \"$COLLECTOR_CRON_SCHEDULE\"\n"
			require.NoError(t, os.WriteFile(filepath.Join(dir, spec.BinaryName), []byte(script), 0700))
			var output bytes.Buffer
			runner := ExecRunner{Stdout: &output, Stderr: &output}
			require.NoError(t, runner.Run(context.Background(), spec, CollectorConfig{ConfigPath: "custom config.yaml"}, dir))
			want := "run\n"
			if spec.Name == "metrics" {
				want += "--cron-schedule=\n"
			}
			want += "--config\ncustom config.yaml\napi=http://example.invalid\nschedule=\n"
			require.Equal(t, want, output.String())
		})
	}
}
