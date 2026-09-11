package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	got, err := ExpandPath("~/scrutiny.yaml")
	if err != nil {
		t.Fatalf("ExpandPath returned error: %v", err)
	}
	if want := filepath.Join(homeDir, "scrutiny.yaml"); got != want {
		t.Fatalf("ExpandPath = %q, want %q", got, want)
	}
}

func TestExpandPathRejectsOtherUsers(t *testing.T) {
	if _, err := ExpandPath("~other/scrutiny.yaml"); err == nil {
		t.Fatal("ExpandPath accepted user-specific home directory")
	}
}

func TestFileExists(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "scrutiny.yaml")
	if err := os.WriteFile(filePath, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if !FileExists(filePath) {
		t.Fatal("FileExists returned false for existing file")
	}
	if !FileExists(tempDir) {
		t.Fatal("FileExists returned false for existing directory")
	}
	if FileExists(filepath.Join(tempDir, "missing")) {
		t.Fatal("FileExists returned true for missing path")
	}
}
