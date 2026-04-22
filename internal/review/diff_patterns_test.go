package review

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestDetectRiskyDiffPatterns(t *testing.T) {
	t.Parallel()

	files := []types.PRFile{
		{
			Path:      "internal/auth/session.go",
			Status:    "modified",
			Additions: 12,
			Patch:     "+token := readToken()\n+if role == \"admin\" { allow = true }\n",
		},
		{
			Path:      "internal/db/query.go",
			Status:    "modified",
			Additions: 10,
			Patch:     "+query := \"SELECT * FROM users WHERE id = \" + userID\n",
		},
		{
			Path:      "internal/security/crypto.go",
			Status:    "modified",
			Additions: 4,
			Patch:     "+hash := sha256.Sum256(payload)\n",
		},
	}
	hunks := []types.DiffHunk{{
		OldPath:  "a/internal/auth/session.go",
		NewPath:  "b/internal/auth/session.go",
		OldStart: 10,
		OldLines: 1,
		NewStart: 10,
		NewLines: 2,
		Section:  "func ValidateSession() {",
		Content:  "+token := readToken()\n+if role == \"admin\" { allow = true }",
	}}

	findings := detectRiskyDiffPatterns(files, hunks)
	if len(findings) < 3 {
		t.Fatalf("expected at least 3 findings, got %d", len(findings))
	}

	var authSeen, dbSeen, cryptoSeen bool
	for _, f := range findings {
		switch f.Subsystem {
		case "auth":
			if f.SignalType == "risky_pattern" {
				authSeen = true
			}
		case "database":
			if f.SignalType == "risky_pattern" {
				dbSeen = true
			}
		case "security":
			if f.SignalType == "risky_pattern" {
				cryptoSeen = true
			}
		}
	}
	if !authSeen {
		t.Fatal("expected auth risky pattern finding")
	}
	if !dbSeen {
		t.Fatal("expected database risky pattern finding")
	}
	if !cryptoSeen {
		t.Fatal("expected crypto risky pattern finding")
	}
}
