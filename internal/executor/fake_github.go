package executor

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type FakePR struct {
	Number    int
	Title     string
	State     string
	Mergeable bool
	HeadSHA   string
	Labels    []string
	Merged    bool
	Closed    bool
}

type FakeComment struct {
	PRNumber int
	Body     string
	At       time.Time
}

type ExecutorLogEntry struct {
	At       time.Time
	Action   string
	PRNumber int
	DryRun   bool
	Result   string
}

type FakeGitHub struct {
	mu                sync.Mutex
	prs               map[int]FakePR
	comments          map[int][]FakeComment
	labels            map[int]map[string]bool
	log               []ExecutorLogEntry
	now               func() time.Time
	ciStatus          string
	mergeable         *bool  // nil means use PR's mergeable field
	requiredReviews   bool
	rateLimitRemaining int
	writes            int // count of non-dry-run writes
}

func NewFakeGitHub() *FakeGitHub {
	return &FakeGitHub{
		prs:               map[int]FakePR{},
		comments:          map[int][]FakeComment{},
		labels:            map[int]map[string]bool{},
		now:               func() time.Time { return time.Now().UTC() },
		ciStatus:          "success",  // default: CI is green
		mergeable:         nil,        // default: use PR's mergeable field
		requiredReviews:   true,       // default: reviews are satisfied
		rateLimitRemaining: 5000,       // default: high rate limit
	}
}

func (f *FakeGitHub) UpsertPR(pr FakePR) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if pr.State == "" {
		pr.State = "open"
	}
	f.prs[pr.Number] = pr
}

// SetTestConfig allows setting test control fields for scenario testing
func (f *FakeGitHub) SetTestConfig(ciStatus string, mergeable bool, requiredReviews bool, rateLimitRemaining int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ciStatus = ciStatus
	f.mergeable = &mergeable
	f.requiredReviews = requiredReviews
	f.rateLimitRemaining = rateLimitRemaining
}

func (f *FakeGitHub) GetPR(number int) (FakePR, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, ok := f.prs[number]
	return pr, ok
}

func (f *FakeGitHub) GetPRState(ctx context.Context, repo string, prNumber int) (PRState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, ok := f.prs[prNumber]
	if !ok {
		return PRState{}, fmt.Errorf("PR #%d not found", prNumber)
	}
	return PRState{
		Number:     pr.Number,
		State:      pr.State,
		HeadSHA:    pr.HeadSHA,
		BaseBranch: "main", // default for testing
		Mergeable:  pr.Mergeable,
		CIStatus:   f.ciStatus, // use configurable field
	}, nil
}

func (f *FakeGitHub) GetHeadSHA(ctx context.Context, repo string, prNumber int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, ok := f.prs[prNumber]
	if !ok {
		return "", fmt.Errorf("PR #%d not found", prNumber)
	}
	return pr.HeadSHA, nil
}

func (f *FakeGitHub) GetBaseBranch(ctx context.Context, repo string, prNumber int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.prs[prNumber]; !ok {
		return "", fmt.Errorf("PR #%d not found", prNumber)
	}
	// For testing, return a default base branch
	return "main", nil
}

func (f *FakeGitHub) GetCIStatus(ctx context.Context, repo string, prNumber int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.prs[prNumber]; !ok {
		return "", fmt.Errorf("PR #%d not found", prNumber)
	}
	// Use configurable field, default to "success" for backward compatibility
	if f.ciStatus == "" {
		return "success", nil
	}
	return f.ciStatus, nil
}

func (f *FakeGitHub) GetMergeable(ctx context.Context, repo string, prNumber int) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, ok := f.prs[prNumber]
	if !ok {
		return false, fmt.Errorf("PR #%d not found", prNumber)
	}
	// Use configurable field if set, otherwise use PR's mergeable field
	if f.mergeable != nil {
		return *f.mergeable, nil
	}
	return pr.Mergeable, nil
}

func (f *FakeGitHub) GetRequiredReviews(ctx context.Context, repo string, prNumber int) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.prs[prNumber]; !ok {
		return false, fmt.Errorf("PR #%d not found", prNumber)
	}
	// Use configurable field, default to true for backward compatibility
	return f.requiredReviews, nil
}

func (f *FakeGitHub) GetRateLimitRemaining(ctx context.Context) (int, error) {
	// Use configurable field
	return f.rateLimitRemaining, nil
}

func (f *FakeGitHub) Comments(number int) []FakeComment {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]FakeComment(nil), f.comments[number]...)
}

func (f *FakeGitHub) Merge(ctx context.Context, repo string, prNumber int, opts MergeOptions, dryRun bool) (MergeResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, ok := f.prs[prNumber]
	if !ok {
		return MergeResult{}, fmt.Errorf("PR #%d not found", prNumber)
	}
	if pr.Closed || pr.State == "closed" {
		return MergeResult{}, fmt.Errorf("PR #%d is closed", prNumber)
	}
	if pr.Merged || pr.State == "merged" {
		f.appendLogLocked("merge", prNumber, dryRun, "already_merged")
		return MergeResult{Merged: true, SHA: pr.HeadSHA, AlreadyMerged: true}, nil
	}
	if !pr.Mergeable {
		return MergeResult{}, fmt.Errorf("PR #%d is not mergeable", prNumber)
	}
	if !dryRun {
		pr.Merged = true
		pr.State = "merged"
		f.prs[prNumber] = pr
		f.writes++
	}
	f.appendLogLocked("merge", prNumber, dryRun, "merged")
	return MergeResult{Merged: true, SHA: pr.HeadSHA}, nil
}

func (f *FakeGitHub) Close(ctx context.Context, repo string, prNumber int, reason string, dryRun bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, ok := f.prs[prNumber]
	if !ok {
		return fmt.Errorf("PR #%d not found", prNumber)
	}
	if pr.Merged || pr.State == "merged" {
		return fmt.Errorf("PR #%d is already merged", prNumber)
	}
	if !dryRun {
		pr.Closed = true
		pr.State = "closed"
		f.prs[prNumber] = pr
		f.writes++
	}
	f.appendLogLocked("close", prNumber, dryRun, "closed")
	return nil
}

func (f *FakeGitHub) AddComment(ctx context.Context, repo string, prNumber int, body string, dryRun bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.prs[prNumber]; !ok {
		return fmt.Errorf("PR #%d not found", prNumber)
	}
	if !dryRun {
		f.comments[prNumber] = append(f.comments[prNumber], FakeComment{PRNumber: prNumber, Body: body, At: f.now()})
		f.writes++
	}
	f.appendLogLocked("comment", prNumber, dryRun, "commented")
	return nil
}

func (f *FakeGitHub) AddLabels(ctx context.Context, repo string, prNumber int, labels []string, dryRun bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.prs[prNumber]; !ok {
		return fmt.Errorf("PR #%d not found", prNumber)
	}
	if !dryRun {
		if f.labels[prNumber] == nil {
			f.labels[prNumber] = map[string]bool{}
		}
		for _, label := range labels {
			f.labels[prNumber][label] = true
		}
		f.writes++
	}
	f.appendLogLocked("label", prNumber, dryRun, "labeled")
	return nil
}

func (f *FakeGitHub) ApplyFix(ctx context.Context, repo string, prNumber int, patch string, dryRun bool) (ApplyFixResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.prs[prNumber]; !ok {
		return ApplyFixResult{}, fmt.Errorf("PR #%d not found", prNumber)
	}
	f.appendLogLocked("apply_fix", prNumber, dryRun, "fix_applied")
	if !dryRun {
		f.writes++
	}
	return ApplyFixResult{Applied: !dryRun, NewBranch: fmt.Sprintf("pratc/fix-%d", prNumber)}, nil
}

func (f *FakeGitHub) GetComments(ctx context.Context, repo string, prNumber int) ([]Comment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.prs[prNumber]; !ok {
		return nil, fmt.Errorf("PR #%d not found", prNumber)
	}
	comments := f.comments[prNumber]
	result := make([]Comment, len(comments))
	for i, c := range comments {
		result[i] = Comment{Body: c.Body}
	}
	return result, nil
}

func (f *FakeGitHub) GetLabels(ctx context.Context, repo string, prNumber int) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.prs[prNumber]; !ok {
		return nil, fmt.Errorf("PR #%d not found", prNumber)
	}
	labels := f.labels[prNumber]
	result := make([]string, 0, len(labels))
	for label := range labels {
		result = append(result, label)
	}
	return result, nil
}

func (f *FakeGitHub) appendLogLocked(action string, prNumber int, dryRun bool, result string) {
	f.log = append(f.log, ExecutorLogEntry{At: f.now(), Action: action, PRNumber: prNumber, DryRun: dryRun, Result: result})
}

// HasWritten returns true if any non-dry-run writes occurred.
func (f *FakeGitHub) HasWritten() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writes > 0
}

// WriteCount returns the number of non-dry-run writes.
func (f *FakeGitHub) WriteCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writes
}

// ResetWrites resets the write counter.
func (f *FakeGitHub) ResetWrites() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes = 0
}
