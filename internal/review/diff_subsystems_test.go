package review

import "testing"

func TestClassifySubsystem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "security folder", path: "internal/security/policy.go", want: "security"},
		{name: "auth path", path: "internal/auth/jwt.go", want: "auth"},
		{name: "api handler", path: "internal/cmd/serve.go", want: "api"},
		{name: "database migration", path: "migrations/0002_add_index.sql", want: "database"},
		{name: "config file", path: "configs/app.yaml", want: "config"},
		{name: "infra manifest", path: ".github/workflows/ci.yml", want: "infra"},
		{name: "test file", path: "internal/review/analyzer_quality_test.go", want: "tests"},
		{name: "docs file", path: "docs/plan.md", want: "docs"},
		{name: "frontend file", path: "web/src/types/api.ts", want: "frontend"},
		{name: "unknown fallback", path: "pkg/misc/helpers.go", want: "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifySubsystem(tc.path)
			if got != tc.want {
				t.Fatalf("classifySubsystem(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}
