package sync

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

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
	var prs []types.PR
	if err := s.StreamBootstrap(ctx, repo, func(pr types.PR) error {
		prs = append(prs, pr)
		return nil
	}); err != nil {
		return nil, err
	}
	return prs, nil
}

func (s *BootstrapFileSource) StreamBootstrap(ctx context.Context, repo string, emit func(types.PR) error) error {
	_ = ctx
	if s == nil || strings.TrimSpace(s.Path) == "" {
		return nil
	}

	f, err := os.Open(s.Path)
	if err != nil {
		return fmt.Errorf("read bootstrap source %q: %w", s.Path, err)
	}
	defer f.Close()

	label := s.SourceLabel
	if label == "" {
		label = SourcePublicArchive
	}

	reader := bufio.NewReader(f)
	first, err := peekFirstNonSpaceByte(reader)
	if err != nil {
		return err
	}
	if first == '[' {
		dec := json.NewDecoder(reader)
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("decode bootstrap source %q: %w", s.Path, err)
		}
		if delim, ok := tok.(json.Delim); !ok || delim != '[' {
			return fmt.Errorf("decode bootstrap source %q: expected JSON array", s.Path)
		}
		for dec.More() {
			var pr types.PR
			if err := dec.Decode(&pr); err != nil {
				return fmt.Errorf("decode bootstrap source %q: %w", s.Path, err)
			}
			if strings.TrimSpace(pr.Repo) != "" && pr.Repo != repo {
				continue
			}
			pr.Repo = repo
			annotateProvenance(&pr, label)
			if err := emit(pr); err != nil {
				return err
			}
		}
		return nil
	}

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var pr types.PR
		if err := json.Unmarshal([]byte(line), &pr); err != nil {
			return fmt.Errorf("decode bootstrap source %q: %w", s.Path, err)
		}
		if strings.TrimSpace(pr.Repo) != "" && pr.Repo != repo {
			continue
		}
		pr.Repo = repo
		annotateProvenance(&pr, label)
		if err := emit(pr); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("decode bootstrap source %q: %w", s.Path, err)
	}
	return nil
}

func decodeBootstrapPRs(data []byte) ([]types.PR, error) {
	reader := bufio.NewReader(bytes.NewReader(data))
	first, err := peekFirstNonSpaceByte(reader)
	if err != nil {
		return nil, err
	}
	if first == 0 {
		return nil, nil
	}

	var prs []types.PR
	if first == '[' {
		dec := json.NewDecoder(reader)
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("decode bootstrap payload: %w", err)
		}
		if delim, ok := tok.(json.Delim); !ok || delim != '[' {
			return nil, fmt.Errorf("decode bootstrap payload: expected JSON array")
		}
		for dec.More() {
			var pr types.PR
			if err := dec.Decode(&pr); err != nil {
				return nil, fmt.Errorf("decode bootstrap payload: %w", err)
			}
			prs = append(prs, pr)
		}
		return prs, nil
	}

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var pr types.PR
		if err := json.Unmarshal([]byte(line), &pr); err != nil {
			return nil, fmt.Errorf("decode bootstrap payload: %w", err)
		}
		prs = append(prs, pr)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("decode bootstrap payload: %w", err)
	}
	return prs, nil
}

type CompositeBootstrapSource struct {
	Sources []BootstrapSource
}

func (c CompositeBootstrapSource) Bootstrap(ctx context.Context, repo string) ([]types.PR, error) {
	var out []types.PR
	if err := c.StreamBootstrap(ctx, repo, func(pr types.PR) error {
		out = append(out, pr)
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (c CompositeBootstrapSource) StreamBootstrap(ctx context.Context, repo string, emit func(types.PR) error) error {
	for _, source := range c.Sources {
		if source == nil {
			continue
		}
		if streamer, ok := source.(BootstrapStreamer); ok {
			if err := streamer.StreamBootstrap(ctx, repo, emit); err != nil {
				return err
			}
			continue
		}
		prs, err := source.Bootstrap(ctx, repo)
		if err != nil {
			return err
		}
		for _, pr := range prs {
			if err := emit(pr); err != nil {
				return err
			}
		}
	}
	return nil
}

func peekFirstNonSpaceByte(r *bufio.Reader) (byte, error) {
	for {
		b, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, nil
			}
			return 0, err
		}
		if unicode.IsSpace(rune(b)) {
			continue
		}
		if err := r.UnreadByte(); err != nil {
			return 0, err
		}
		return b, nil
	}
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
