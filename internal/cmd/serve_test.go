package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQueueDBPathEnv(t *testing.T) {
	// Save original env var
	orig := os.Getenv("PRATC_QUEUE_DB_PATH")
	defer func() {
		if orig != "" {
			os.Setenv("PRATC_QUEUE_DB_PATH", orig)
		} else {
			os.Unsetenv("PRATC_QUEUE_DB_PATH")
		}
	}()

	// Test default path
	os.Unsetenv("PRATC_QUEUE_DB_PATH")
	defaultPath := queueDBPathFromEnv()
	home, _ := os.UserHomeDir()
	expectedDefault := filepath.Join(home, ".pratc", "queue.db")
	if defaultPath != expectedDefault {
		t.Errorf("default queue DB path = %q, want %q", defaultPath, expectedDefault)
	}

	// Test custom path via env
	customPath := "/custom/path/queue.db"
	os.Setenv("PRATC_QUEUE_DB_PATH", customPath)
	got := queueDBPathFromEnv()
	if got != customPath {
		t.Errorf("queueDBPathFromEnv with env = %q, want %q", got, customPath)
	}
}
