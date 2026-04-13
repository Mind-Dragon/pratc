package sync

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jeffersonnunn/pratc/internal/types"
)

const (
	SourceLiveAPI       = "live_api"
	SourceLocalMirror   = "local_mirror"
	SourcePublicArchive = "public_archive"
	SourceBackfill      = "backfill"
)

type BootstrapFileSource struct {
	Path         string
	SourceLabel  string
	RepoFallback string
}

func NewBootstrapFileSource(path string, sourceLabel string) *BootstrapFileSource {
	return &BootstrapFileSource{Path: strings.TrimSpace(path), SourceLabel: strings.TrimSpace(sourceLabel)}
}

func (s *BootstrapFileSource) Bootstrap(ctx context.Context, repo string) ([]types.PR, error) {
	_ = ctx
	if s == nil || strings.TrimSpace(s.Path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, fmt.Errorf("read bootstrap source %q: %w", s.Path, err)
	}

	parsed, err := decodeBootstrapPRs(data)
	if err != nil {
		return nil, fmt.Errorf("decode bootstrap source %q: %w", s.Path, err)
	}

	label := s.SourceLabel
	if label == "" {
		label = SourcePublicArchive
	}
	out := make([]types.PR, 0, len(parsed))
	for _, pr := range parsed {
		if strings.TrimSpace(pr.Repo) != "" && pr.Repo != repo {
			continue
		}
		pr.Repo = repo
		annotateProvenance(&pr, label)
		out = append(out, pr)
	}
	return out, nil
}

type CompositeBootstrapSource struct {
	Sources []BootstrapSource
}

func (c CompositeBootstrapSource) Bootstrap(ctx context.Context, repo string) ([]types.PR, error) {
	for _, source := range c.Sources {
		if source == nil {
			continue
		}
		prs, err := source.Bootstrap(ctx, repo)
		if err != nil {
			return nil, err
		}
		if len(prs) > 0 {
			return prs, nil
		}
	}
	return nil, nil
}

func decodeBootstrapPRs(data []byte) ([]types.PR, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil
	}

	if strings.HasPrefix(trimmed, "[") {
		var prs []types.PR
		if err := json.Unmarshal([]byte(trimmed), &prs); err != nil {
			return nil, err
		}
		return prs, nil
	}

	scanner := bufio.NewScanner(strings.NewReader(trimmed))
	var prs []types.PR
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var pr types.PR
		if err := json.Unmarshal([]byte(line), &pr); err != nil {
			return nil, err
		}
		prs = append(prs, pr)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return prs, nil
}

func annotateProvenance(pr *types.PR, source string) {
	if pr == nil {
		return
	}
	if pr.Provenance == nil {
		pr.Provenance = map[string]string{}
	}
	for field, value := range map[string]bool{
		"id":                  pr.ID != "",
		"repo":                pr.Repo != "",
		"title":               pr.Title != "",
		"body":                pr.Body != "",
		"url":                 pr.URL != "",
		"author":              pr.Author != "",
		"labels":              len(pr.Labels) > 0,
		"files_changed":       len(pr.FilesChanged) > 0,
		"review_status":       pr.ReviewStatus != "",
		"ci_status":           pr.CIStatus != "",
		"mergeable":           pr.Mergeable != "",
		"base_branch":         pr.BaseBranch != "",
		"head_branch":         pr.HeadBranch != "",
		"cluster_id":          pr.ClusterID != "",
		"created_at":          pr.CreatedAt != "",
		"updated_at":          pr.UpdatedAt != "",
		"is_draft":            true,
		"is_bot":              true,
		"additions":           pr.Additions != 0,
		"deletions":           pr.Deletions != 0,
		"changed_files_count": pr.ChangedFilesCount != 0,
	} {
		if value {
			pr.Provenance[field] = source
		}
	}
}

func bootstrapPathLabel(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jsonl", ".ndjson":
		return SourcePublicArchive
	case ".json":
		return SourcePublicArchive
	default:
		return SourceBackfill
	}
}
