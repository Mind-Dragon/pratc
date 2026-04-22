package review

import "strings"

func classifySubsystem(path string) string {
	p := strings.ToLower(strings.TrimSpace(path))
	if p == "" {
		return "unknown"
	}

	switch {
	case strings.Contains(p, "security/") || strings.Contains(p, "/security/"):
		return "security"
	case strings.Contains(p, "auth/") || strings.Contains(p, "/auth/") || strings.Contains(p, "jwt") || strings.Contains(p, "session"):
		return "auth"
	case strings.Contains(p, "/cmd/") || strings.Contains(p, "/api/") || strings.Contains(p, "serve.go"):
		return "api"
	case strings.Contains(p, "migration") || strings.HasSuffix(p, ".sql") || strings.Contains(p, "schema") || strings.Contains(p, "database") || strings.Contains(p, "/db/"):
		return "database"
	case strings.Contains(p, ".github/") || strings.Contains(p, "docker") || strings.Contains(p, "infra") || strings.Contains(p, "k8s") || strings.Contains(p, "terraform"):
		return "infra"
	case strings.Contains(p, "config") || strings.HasSuffix(p, ".yaml") || strings.HasSuffix(p, ".yml") || strings.HasSuffix(p, ".toml") || strings.HasSuffix(p, ".json"):
		return "config"
	case strings.Contains(p, "_test.") || strings.Contains(p, "/test") || strings.Contains(p, "/tests/"):
		return "tests"
	case strings.Contains(p, "docs/") || strings.HasSuffix(p, ".md"):
		return "docs"
	case strings.Contains(p, "web/") || strings.HasSuffix(p, ".tsx") || strings.HasSuffix(p, ".ts") || strings.HasSuffix(p, ".jsx") || strings.HasSuffix(p, ".js") || strings.HasSuffix(p, ".css"):
		return "frontend"
	default:
		return "unknown"
	}
}
