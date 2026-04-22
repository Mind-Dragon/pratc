package github

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveTokenForLogin_UsesRequestedLogin(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "gh")
	contents := `#!/bin/sh
if [ "$1" = "auth" ] && [ "$2" = "status" ]; then
  if [ "$3" = "--json" ] && [ "$4" = "account" ]; then
    echo '{"accounts":[{"login":"Mind-Dragon","active":true},{"login":"avirweb","active":false}]}'
    exit 0
  fi
fi
if [ "$1" = "auth" ] && [ "$2" = "token" ]; then
  printf '%s' "token-for-Mind-Dragon"
  exit 0
fi
exit 1`
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", dir)

	_, err := ResolveTokenForLogin(context.Background(), "avirweb")
	if err == nil {
		t.Fatal("expected login mismatch error")
	}
	if !strings.Contains(err.Error(), "requested login") {
		t.Fatalf("expected requested login error, got %v", err)
	}
}
