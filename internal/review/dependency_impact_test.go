package review

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestDetectDependencyImpact(t *testing.T) {
	t.Parallel()

	files := []types.PRFile{
		{Path: "contracts/plan-response.json", Patch: "+new field"},
		{Path: "internal/types/models.go", Patch: "+type NewContract struct{}"},
		{Path: "migrations/0003_add_index.sql", Patch: "+create index"},
		{Path: "configs/app.yaml", Patch: "+feature: true"},
	}

	findings := detectDependencyImpact(files)
	if len(findings) < 4 {
		t.Fatalf("expected at least 4 dependency findings, got %d", len(findings))
	}

	seen := map[string]bool{}
	for _, finding := range findings {
		seen[finding.Finding] = true
	}
	for _, want := range []string{
		"public API surface changed",
		"shared module changed",
		"schema or migration change requires rollout coordination",
		"configuration surface changed",
	} {
		if !seen[want] {
			t.Fatalf("missing dependency finding %q in %#v", want, findings)
		}
	}
}
