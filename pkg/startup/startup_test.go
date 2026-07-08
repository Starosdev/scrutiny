package startup

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

type stubConfig map[string]string

func (s stubConfig) GetString(key string) string {
	return s[key]
}

func TestShouldPrintBanner(t *testing.T) {
	original, hadOriginal := os.LookupEnv(NoLogoEnv)
	t.Cleanup(func() {
		if hadOriginal {
			_ = os.Setenv(NoLogoEnv, original)
			return
		}
		_ = os.Unsetenv(NoLogoEnv)
	})

	_ = os.Unsetenv(NoLogoEnv)
	if !ShouldPrintBanner() {
		t.Fatalf("expected banner to print when %s is unset", NoLogoEnv)
	}

	_ = os.Setenv(NoLogoEnv, "true")
	if ShouldPrintBanner() {
		t.Fatalf("expected banner to be suppressed when %s=true", NoLogoEnv)
	}

	_ = os.Setenv(NoLogoEnv, "invalid")
	if !ShouldPrintBanner() {
		t.Fatalf("expected invalid %s value to keep banner enabled", NoLogoEnv)
	}
}

func TestBootstrapLevel(t *testing.T) {
	if level := bootstrapLevel(stubConfig{logLevelKey: "warn"}); level != logrus.WarnLevel {
		t.Fatalf("expected warn level, got %s", level)
	}

	if level := bootstrapLevel(stubConfig{logLevelKey: "not-a-level"}); level != logrus.InfoLevel {
		t.Fatalf("expected info fallback, got %s", level)
	}
}
