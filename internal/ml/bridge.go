package ml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

const defaultTimeout = 120 * time.Second

// Bridge invokes the Python ML service via JSON-over-STDIN/STDOUT subprocess.
type Bridge struct {
	python  string
	workDir string
	timeout time.Duration
}

// Config holds optional Bridge configuration.
type Config struct {
	Python  string        // path to python binary (auto-detected if empty)
	WorkDir string        // ml-service working directory (auto-detected if empty)
	Timeout time.Duration // subprocess timeout (default 120s)
}

// NewBridge creates a Bridge that shells out to the Python ML CLI.
func NewBridge(cfg Config) *Bridge {
	python := cfg.Python
	if python == "" {
		python = findPython()
	}

	workDir := cfg.WorkDir
	if workDir == "" {
		workDir = findMLServiceDir()
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	return &Bridge{python: python, workDir: workDir, timeout: timeout}
}

// Available returns true if a Python interpreter was found.
func (b *Bridge) Available() bool {
	return b.python != ""
}

// Cluster delegates clustering to the Python ML service.
// Returns nil clusters if the subprocess is unavailable or fails.
func (b *Bridge) Cluster(ctx context.Context, repo string, prs []types.PR, requestID string) ([]types.PRCluster, string, error) {
	if !b.Available() {
		return nil, "", nil
	}

	payload := buildClusterPayload(repo, prs, requestID)
	var result clusterResult
	if err := b.invoke(ctx, payload, &result); err != nil {
		return nil, "", err
	}

	clusters := make([]types.PRCluster, 0, len(result.Clusters))
	for _, c := range result.Clusters {
		clusters = append(clusters, types.PRCluster{
			ClusterID:         c.ClusterID,
			ClusterLabel:      c.ClusterLabel,
			Summary:           c.Summary,
			PRIDs:             c.PRIDs,
			HealthStatus:      c.HealthStatus,
			AverageSimilarity: c.AverageSimilarity,
			SampleTitles:      c.SampleTitles,
		})
	}

	model := result.Model
	if model == "" {
		model = "unknown"
	}

	return clusters, model, nil
}

// Duplicates delegates duplicate detection to the Python ML service.
// Returns nil groups if the subprocess is unavailable or fails.
func (b *Bridge) Duplicates(ctx context.Context, repo string, prs []types.PR, duplicateThreshold, overlapThreshold float64, requestID string) ([]types.DuplicateGroup, []types.DuplicateGroup, error) {
	if !b.Available() {
		return nil, nil, nil
	}

	payload := buildDuplicatePayload(repo, prs, duplicateThreshold, overlapThreshold, requestID)
	var result duplicateResult
	if err := b.invoke(ctx, payload, &result); err != nil {
		return nil, nil, err
	}

	dups := make([]types.DuplicateGroup, 0, len(result.Duplicates))
	for _, d := range result.Duplicates {
		dups = append(dups, types.DuplicateGroup{
			CanonicalPRNumber: d.CanonicalPRNumber,
			DuplicatePRNums:   d.DuplicatePRNums,
			Similarity:        d.Similarity,
			Reason:            d.Reason,
		})
	}

	overlaps := make([]types.DuplicateGroup, 0, len(result.Overlaps))
	for _, o := range result.Overlaps {
		overlaps = append(overlaps, types.DuplicateGroup{
			CanonicalPRNumber: o.CanonicalPRNumber,
			DuplicatePRNums:   o.DuplicatePRNums,
			Similarity:        o.Similarity,
			Reason:            o.Reason,
		})
	}

	return dups, overlaps, nil
}

func (b *Bridge) invoke(ctx context.Context, payload map[string]any, dest any) error {
	input, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("ml bridge: marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, b.python, "-m", "pratc_ml.cli")
	cmd.Dir = b.workDir
	cmd.Stdin = bytes.NewReader(input)

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ml bridge: subprocess: %w", err)
	}

	if err := json.Unmarshal(output, dest); err != nil {
		return fmt.Errorf("ml bridge: unmarshal response: %w", err)
	}

	return nil
}

func buildClusterPayload(repo string, prs []types.PR, requestID string) map[string]any {
	mlPRs := make([]map[string]any, 0, len(prs))
	for _, pr := range prs {
		mlPRs = append(mlPRs, prToML(pr))
	}
	payload := map[string]any{
		"action": "cluster",
		"repo":   repo,
		"prs":    mlPRs,
	}
	if requestID != "" {
		payload["request_id"] = requestID
	}
	return payload
}

func buildDuplicatePayload(repo string, prs []types.PR, dup, overlap float64, requestID string) map[string]any {
	mlPRs := make([]map[string]any, 0, len(prs))
	for _, pr := range prs {
		mlPRs = append(mlPRs, prToML(pr))
	}
	payload := map[string]any{
		"action":             "duplicates",
		"repo":               repo,
		"prs":                mlPRs,
		"duplicateThreshold": dup,
		"overlapThreshold":   overlap,
	}
	if requestID != "" {
		payload["request_id"] = requestID
	}
	return payload
}

func prToML(pr types.PR) map[string]any {
	return map[string]any{
		"id":                  pr.ID,
		"repo":                pr.Repo,
		"number":              pr.Number,
		"title":               pr.Title,
		"body":                pr.Body,
		"url":                 pr.URL,
		"author":              pr.Author,
		"labels":              pr.Labels,
		"files_changed":       pr.FilesChanged,
		"review_status":       pr.ReviewStatus,
		"ci_status":           pr.CIStatus,
		"mergeable":           pr.Mergeable,
		"base_branch":         pr.BaseBranch,
		"head_branch":         pr.HeadBranch,
		"cluster_id":          pr.ClusterID,
		"created_at":          pr.CreatedAt,
		"updated_at":          pr.UpdatedAt,
		"is_draft":            pr.IsDraft,
		"is_bot":              pr.IsBot,
		"additions":           pr.Additions,
		"deletions":           pr.Deletions,
		"changed_files_count": pr.ChangedFilesCount,
	}
}

func findPython() string {
	candidates := []string{"python3", "python"}
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return ""
}

func findMLServiceDir() string {
	// Walk up from executable looking for ml-service directory
	// Fall back to ./ml-service relative to working directory
	if _, err := os.Stat("ml-service/src/pratc_ml/cli.py"); err == nil {
		return "ml-service"
	}
	return ""
}

// JSON response shapes from the Python ML service.

type clusterResult struct {
	Clusters []clusterGroup `json:"clusters"`
	Model    string         `json:"model"`
}

type clusterGroup struct {
	ClusterID         string   `json:"cluster_id"`
	ClusterLabel      string   `json:"cluster_label"`
	Summary           string   `json:"summary"`
	PRIDs             []int    `json:"pr_ids"`
	HealthStatus      string   `json:"health_status"`
	AverageSimilarity float64  `json:"average_similarity"`
	SampleTitles      []string `json:"sample_titles"`
}

type duplicateResult struct {
	Duplicates []duplicateGroup `json:"duplicates"`
	Overlaps   []duplicateGroup `json:"overlaps"`
}

type duplicateGroup struct {
	CanonicalPRNumber int     `json:"canonical_pr_number"`
	DuplicatePRNums   []int   `json:"duplicate_pr_numbers"`
	Similarity        float64 `json:"similarity"`
	Reason            string  `json:"reason"`
}
