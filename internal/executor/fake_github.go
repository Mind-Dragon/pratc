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
	mu       sync.Mutex
	prs      map[int]FakePR
	comments map[int][]FakeComment
	labels   map[int]map[string]bool
	log      []ExecutorLogEntry
	now      func() time.Time
}

func NewFakeGitHub() *FakeGitHub {
	return &FakeGitHub{
		prs:      map[int]FakePR{},
		comments: map[int][]FakeComment{},
		labels:   map[int]map[string]bool{},
		now:      func() time.Time { return time.Now().UTC() },
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

func (f *FakeGitHub) GetPR(number int) (FakePR, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pr, ok := f.prs[number]
	return pr, ok
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
	return ApplyFixResult{Applied: !dryRun, NewBranch: fmt.Sprintf("pratc/fix-%d", prNumber)}, nil
}

func (f *FakeGitHub) appendLogLocked(action string, prNumber int, dryRun bool, result string) {
	f.log = append(f.log, ExecutorLogEntry{At: f.now(), Action: action, PRNumber: prNumber, DryRun: dryRun, Result: result})
}
