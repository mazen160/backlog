package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResetWorkspaceFilesPreservesDirectoryAndUnmanagedFiles(t *testing.T) {
	dir := t.TempDir()
	managed := []string{
		"backlog.db",
		"backlog.db-shm",
		"backlog.db-wal",
		"backlog.db-journal",
		"backlog.json",
		"backlog.config",
		"config.toml",
	}
	for _, name := range managed {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("managed"), 0644); err != nil {
			t.Fatalf("write managed file %s: %v", name, err)
		}
	}
	unmanaged := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(unmanaged, []byte("keep"), 0644); err != nil {
		t.Fatalf("write unmanaged file: %v", err)
	}

	if err := resetWorkspaceFiles(dir); err != nil {
		t.Fatalf("resetWorkspaceFiles: %v", err)
	}

	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("workspace directory was not preserved: info=%v err=%v", info, err)
	}
	if _, err := os.Stat(unmanaged); err != nil {
		t.Fatalf("unmanaged file was not preserved: %v", err)
	}
	for _, name := range managed {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("managed file %s still exists or stat failed with unexpected error: %v", name, err)
		}
	}
}
