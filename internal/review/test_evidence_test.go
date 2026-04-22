package review

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestDetectTestEvidence(t *testing.T) {
	t.Parallel()

	t.Run("production with tests emits evidence", func(t *testing.T) {
		files := []types.PRFile{
			{Path: "internal/api/handler.go", Additions: 40, Patch: "+func Serve() {}"},
			{Path: "internal/api/handler_test.go", Additions: 20, Patch: "+func TestServe() {}"},
		}
		findings := detectTestEvidence(files)
		if len(findings) == 0 {
			t.Fatal("expected test evidence finding")
		}
		if findings[0].SignalType != "test_evidence" {
			t.Fatalf("expected signal type test_evidence, got %q", findings[0].SignalType)
		}
	})

	t.Run("partial test coverage emits partial signal", func(t *testing.T) {
		files := []types.PRFile{
			{Path: "internal/api/handler.go", Additions: 80, Patch: "+func Serve() {}"},
			{Path: "internal/api/handler_test.go", Additions: 10, Patch: "+func TestServe() {}"},
		}
		findings := detectTestEvidence(files)
		seenPartial := false
		for _, finding := range findings {
			if finding.SignalType == "coverage_partial" {
				seenPartial = true
			}
		}
		if !seenPartial {
			t.Fatal("expected coverage_partial finding")
		}
	})
}
