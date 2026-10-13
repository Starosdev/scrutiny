//go:build !windows

package statfs

import (
	"path/filepath"
	"testing"
)

func TestStat(t *testing.T) {
	res, err := Stat(t.TempDir())
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if res.Total <= 0 {
		t.Errorf("Total = %d, want > 0", res.Total)
	}
	if res.Used < 0 || res.Available < 0 {
		t.Errorf("negative figure: used=%d available=%d", res.Used, res.Available)
	}
	// Used is Blocks-Bfree and Available is Bavail (<= Bfree), so together they never exceed Total.
	if res.Used+res.Available > res.Total {
		t.Errorf("used(%d)+available(%d) > total(%d)", res.Used, res.Available, res.Total)
	}
}

func TestStatMissingPath(t *testing.T) {
	if _, err := Stat(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Error("expected error for missing path")
	}
}
