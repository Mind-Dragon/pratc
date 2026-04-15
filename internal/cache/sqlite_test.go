package cache

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
	_ "modernc.org/sqlite"
)

const (
	supportedSchemaVersion = 4
)

func TestCacheUpsertAndQuery(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	for i := 1; i <= 10; i++ {
		pr := samplePR(i)
		if i%2 == 0 {
			pr.BaseBranch = "release"
			pr.CIStatus = "failing"
		}
		if err := store.UpsertPR(pr); err != nil {
			t.Fatalf("upsert pr %d: %v", i, err)
		}
	}

	mainPRs, err := store.ListPRs(PRFilter{Repo: "owner/repo", BaseBranch: "main"})
	if err != nil {
		t.Fatalf("list main prs: %v", err)
	}
	if len(mainPRs) != 5 {
		t.Fatalf("expected 5 main prs, got %d", len(mainPRs))
	}

	releasePRs, err := store.ListPRs(PRFilter{Repo: "owner/repo", BaseBranch: "release", CIStatus: "failing"})
	if err != nil {
		t.Fatalf("list release prs: %v", err)
	}
	if len(releasePRs) != 5 {
		t.Fatalf("expected 5 release prs, got %d", len(releasePRs))
	}
	if got := mainPRs[0].Provenance["files_changed"]; got != "local_mirror" {
		t.Fatalf("expected provenance for files_changed to survive round trip, got %q", got)
	}
}

func TestCacheListPRsUsesCursorPaginationAcrossLargeResultSets(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	for i := 1; i <= 1500; i++ {
		if err := store.UpsertPR(samplePR(i)); err != nil {
			t.Fatalf("upsert pr %d: %v", i, err)
		}
	}

	prs, err := store.ListPRs(PRFilter{Repo: "owner/repo"})
	if err != nil {
		t.Fatalf("list prs: %v", err)
	}
	if len(prs) != 1500 {
		t.Fatalf("expected 1500 prs, got %d", len(prs))
	}
	if prs[0].Number != 1 || prs[len(prs)-1].Number != 1500 {
		t.Fatalf("expected sorted results from 1 to 1500, got %d..%d", prs[0].Number, prs[len(prs)-1].Number)
	}
}

func TestCacheUpdatedSinceFilter(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	oldPR := samplePR(1)
	oldPR.UpdatedAt = "2026-03-12T10:00:00Z"
	if err := store.UpsertPR(oldPR); err != nil {
		t.Fatalf("upsert old pr: %v", err)
	}

	newPR := samplePR(2)
	newPR.UpdatedAt = "2026-03-12T12:00:00Z"
	if err := store.UpsertPR(newPR); err != nil {
		t.Fatalf("upsert new pr: %v", err)
	}

	prs, err := store.ListPRs(PRFilter{
		Repo:         "owner/repo",
		UpdatedSince: mustParseTime(t, "2026-03-12T11:00:00Z"),
	})
	if err != nil {
		t.Fatalf("list prs by updated since: %v", err)
	}
	if len(prs) != 1 || prs[0].Number != 2 {
		t.Fatalf("expected only PR 2, got %+v", prs)
	}
}

func TestCacheListPRsPageAdvancesCursor(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	for i := 1; i <= 7; i++ {
		if err := store.UpsertPR(samplePR(i)); err != nil {
			t.Fatalf("upsert pr %d: %v", i, err)
		}
	}

	page1, err := store.ListPRsPage(PRFilter{Repo: "owner/repo"}, "", 3)
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if !page1.HasMore {
		t.Fatal("expected first page to report more results")
	}
	if page1.NextCursor != "3" {
		t.Fatalf("expected first next cursor 3, got %q", page1.NextCursor)
	}
	if got := numbersFromPRs(page1.PRs); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("expected first page numbers [1 2 3], got %v", got)
	}

	page2, err := store.ListPRsPage(PRFilter{Repo: "owner/repo"}, page1.NextCursor, 3)
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if !page2.HasMore {
		t.Fatal("expected second page to report more results")
	}
	if page2.NextCursor != "6" {
		t.Fatalf("expected second next cursor 6, got %q", page2.NextCursor)
	}
	if got := numbersFromPRs(page2.PRs); !reflect.DeepEqual(got, []int{4, 5, 6}) {
		t.Fatalf("expected second page numbers [4 5 6], got %v", got)
	}

	page3, err := store.ListPRsPage(PRFilter{Repo: "owner/repo"}, page2.NextCursor, 3)
	if err != nil {
		t.Fatalf("list third page: %v", err)
	}
	if page3.HasMore {
		t.Fatal("expected third page to be terminal")
	}
	if page3.NextCursor != "" {
		t.Fatalf("expected terminal cursor to be empty, got %q", page3.NextCursor)
	}
	if got := numbersFromPRs(page3.PRs); !reflect.DeepEqual(got, []int{7}) {
		t.Fatalf("expected third page numbers [7], got %v", got)
	}
}

func TestCacheListPRsIterStopsOnError(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	for i := 1; i <= 5; i++ {
		if err := store.UpsertPR(samplePR(i)); err != nil {
			t.Fatalf("upsert pr %d: %v", i, err)
		}
	}

	count := 0
	err := store.ListPRsIter(PRFilter{Repo: "owner/repo"}, func(pr types.PR) error {
		count++
		if count == 3 {
			return fmt.Errorf("stop at %d", pr.Number)
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected iterator to stop with an error")
	}
	if count != 3 {
		t.Fatalf("expected iterator to stop after 3 items, got %d", count)
	}
}

func TestCacheLastSyncRoundTrip(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	want := mustParseTime(t, "2026-03-12T13:00:00Z")
	if err := store.SetLastSync("owner/repo", want); err != nil {
		t.Fatalf("set last sync: %v", err)
	}

	got, err := store.LastSync("owner/repo")
	if err != nil {
		t.Fatalf("get last sync: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestCacheSyncProgressRoundTrip(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	if err := store.SaveCursor("owner/repo", "cursor-42", 7, 9); err != nil {
		t.Fatalf("save cursor: %v", err)
	}
	progress, ok, err := store.GetSyncProgress("owner/repo")
	if err != nil {
		t.Fatalf("get sync progress: %v", err)
	}
	if !ok {
		t.Fatal("expected progress row to exist")
	}
	if progress.Cursor != "cursor-42" || progress.ProcessedPRs != 7 || progress.TotalPRs != 9 {
		t.Fatalf("unexpected progress round trip: %+v", progress)
	}
	if progress.EstimatedRequests != 6 {
		t.Fatalf("expected estimated requests 6, got %+v", progress)
	}
}

func TestCacheMergedPRRoundTrip(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	merged := MergedPR{
		Repo:         "owner/repo",
		Number:       99,
		MergedAt:     mustParseTime(t, "2026-03-11T18:00:00Z"),
		FilesTouched: []string{"internal/planner/plan.go", "internal/types/models.go"},
	}

	if err := store.UpsertMergedPR(merged); err != nil {
		t.Fatalf("upsert merged pr: %v", err)
	}

	got, err := store.ListMergedPRs("owner/repo")
	if err != nil {
		t.Fatalf("list merged prs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 merged pr, got %d", len(got))
	}
	if got[0].Number != merged.Number || len(got[0].FilesTouched) != 2 {
		t.Fatalf("unexpected merged pr: %+v", got[0])
	}
}

func TestCachePRFilesRoundTripAndReplace(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	if _, found, err := store.GetPRFiles("owner/repo", 42); err != nil {
		t.Fatalf("get missing pr files: %v", err)
	} else if found {
		t.Fatal("expected missing pr files to be unfound")
	}

	wantFirst := []string{"z.go", "a.go", "internal/cache/sqlite.go"}
	if err := store.UpsertPRFiles("owner/repo", 42, wantFirst); err != nil {
		t.Fatalf("upsert first file set: %v", err)
	}

	got, found, err := store.GetPRFiles("owner/repo", 42)
	if err != nil {
		t.Fatalf("get first file set: %v", err)
	}
	if !found {
		t.Fatal("expected file set to be found")
	}
	wantSortedFirst := []string{"a.go", "internal/cache/sqlite.go", "z.go"}
	if !reflect.DeepEqual(got, wantSortedFirst) {
		t.Fatalf("expected sorted files %v, got %v", wantSortedFirst, got)
	}

	wantSecond := []string{"b.go"}
	if err := store.UpsertPRFiles("owner/repo", 42, wantSecond); err != nil {
		t.Fatalf("replace file set: %v", err)
	}

	got, found, err = store.GetPRFiles("owner/repo", 42)
	if err != nil {
		t.Fatalf("get replaced file set: %v", err)
	}
	if !found {
		t.Fatal("expected replaced file set to be found")
	}
	if !reflect.DeepEqual(got, wantSecond) {
		t.Fatalf("expected replaced files %v, got %v", wantSecond, got)
	}

	if err := store.ClearPRFiles("owner/repo", 42); err != nil {
		t.Fatalf("clear pr files: %v", err)
	}

	if _, found, err := store.GetPRFiles("owner/repo", 42); err != nil {
		t.Fatalf("get cleared pr files: %v", err)
	} else if found {
		t.Fatal("expected cleared pr files to be unfound")
	}
}

func TestCacheSyncJobLifecycle(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	store.now = func() time.Time {
		return mustParseTime(t, "2026-03-12T14:00:00Z")
	}

	job, err := store.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	if err := store.UpdateSyncJobProgress(job.ID, SyncProgress{
		Cursor:          "cursor-2",
		ProcessedPRs:    25,
		TotalPRs:        100,
		LastBudgetCheck: mustParseTime(t, "2026-03-12T14:03:30Z"),
	}); err != nil {
		t.Fatalf("update sync job progress: %v", err)
	}

	if err := store.MarkSyncJobComplete(job.ID, mustParseTime(t, "2026-03-12T14:05:00Z")); err != nil {
		t.Fatalf("mark job complete: %v", err)
	}

	got, err := store.GetSyncJob(job.ID)
	if err != nil {
		t.Fatalf("get sync job: %v", err)
	}
	if got.Status != SyncJobStatusCompleted {
		t.Fatalf("expected completed status, got %s", got.Status)
	}
	if got.Progress.Cursor != "cursor-2" || got.Progress.ProcessedPRs != 25 || got.Progress.TotalPRs != 100 {
		t.Fatalf("unexpected job progress: %+v", got.Progress)
	}
	if got.Progress.LastBudgetCheck != mustParseTime(t, "2026-03-12T14:03:30Z") {
		t.Fatalf("unexpected last budget check: %s", got.Progress.LastBudgetCheck)
	}
}

func TestCacheResumeSyncJobReturnsLatestInProgress(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	first, err := store.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create first job: %v", err)
	}
	if err := store.MarkSyncJobFailed(first.ID, "network error"); err != nil {
		t.Fatalf("fail first job: %v", err)
	}

	second, err := store.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create second job: %v", err)
	}
	if err := store.UpdateSyncJobProgress(second.ID, SyncProgress{Cursor: "cursor-live", ProcessedPRs: 12, TotalPRs: 50}); err != nil {
		t.Fatalf("update second job: %v", err)
	}

	got, ok, err := store.ResumeSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("resume sync job: %v", err)
	}
	if !ok {
		t.Fatal("expected resumable sync job")
	}
	if got.ID != second.ID || got.Progress.Cursor != "cursor-live" {
		t.Fatalf("unexpected resumed job: %+v", got)
	}
}

func TestCacheWALConcurrentReadWrite(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	if mode, err := store.JournalMode(); err != nil {
		t.Fatalf("journal mode: %v", err)
	} else if mode != "wal" {
		t.Fatalf("expected wal journal mode, got %q", mode)
	}

	var writer sync.WaitGroup
	writer.Add(1)
	go func() {
		defer writer.Done()
		for i := 1; i <= 25; i++ {
			if err := store.UpsertPR(samplePR(i)); err != nil {
				t.Errorf("upsert pr %d: %v", i, err)
				return
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := store.ListPRs(PRFilter{Repo: "owner/repo"}); err != nil {
				t.Errorf("list prs during write: %v", err)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	writer.Wait()
	<-done
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "cache.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})

	return store
}

func samplePR(number int) types.PR {
	return types.PR{
		ID:                fmt.Sprintf("PR_%d", number),
		Repo:              "owner/repo",
		Number:            number,
		Title:             fmt.Sprintf("PR %d", number),
		Body:              "Body",
		URL:               fmt.Sprintf(types.GitHubURLPrefix+"owner/repo/pull/%d", number),
		Author:            "octocat",
		Labels:            []string{"triage"},
		FilesChanged:      []string{fmt.Sprintf("internal/service/file_%d.go", number)},
		ReviewStatus:      "approved",
		CIStatus:          "passing",
		Mergeable:         "mergeable",
		BaseBranch:        "main",
		HeadBranch:        fmt.Sprintf("feature/%d", number),
		CreatedAt:         "2026-03-12T09:00:00Z",
		UpdatedAt:         fmt.Sprintf("2026-03-12T%02d:00:00Z", 10+number%10),
		Additions:         10 + number,
		Deletions:         number,
		ChangedFilesCount: 1,
		Provenance: map[string]string{
			"title":               "live_api",
			"body":                "live_api",
			"url":                 "live_api",
			"author":              "live_api",
			"labels":              "live_api",
			"files_changed":       "local_mirror",
			"review_status":       "live_api",
			"ci_status":           "live_api",
			"mergeable":           "live_api",
			"base_branch":         "live_api",
			"head_branch":         "live_api",
			"created_at":          "live_api",
			"updated_at":          "live_api",
			"additions":           "live_api",
			"deletions":           "live_api",
			"changed_files_count": "live_api",
		},
	}
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return parsed
}

func numbersFromPRs(prs []types.PR) []int {
	numbers := make([]int, 0, len(prs))
	for _, pr := range prs {
		numbers = append(numbers, pr.Number)
	}
	return numbers
}

func TestMigrationFreshInstall(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fresh.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open fresh store: %v", err)
	}
	defer store.Close()

	var userVersion int
	if err := store.db.QueryRow(`PRAGMA user_version;`).Scan(&userVersion); err != nil {
		t.Fatalf("query user_version: %v", err)
	}
	if userVersion != supportedSchemaVersion {
		t.Fatalf("expected user_version %d, got %d", supportedSchemaVersion, userVersion)
	}

	var version int
	var name string
	var appliedAt string
	if err := store.db.QueryRow(`SELECT version, name, applied_at FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version, &name, &appliedAt); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if version != 4 || name != "field_provenance" {
		t.Fatalf("expected version=4 name=field_provenance, got version=%d name=%s", version, name)
	}

	requiredTables := []string{
		"schema_migrations",
		"pull_requests",
		"pr_files",
		"pr_reviews",
		"ci_status",
		"sync_jobs",
		"sync_progress",
		"merged_pr_index",
		"audit_log",
	}

	for _, table := range requiredTables {
		var exists string
		if err := store.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&exists); err != nil {
			t.Fatalf("check table %s exists: %v", table, err)
		}
	}

	pr := samplePR(1)
	if err := store.UpsertPR(pr); err != nil {
		t.Fatalf("upsert pr on fresh db: %v", err)
	}

	prs, err := store.ListPRs(PRFilter{Repo: "owner/repo"})
	if err != nil {
		t.Fatalf("list prs on fresh db: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 pr on fresh db, got %d", len(prs))
	}
}

func TestMigrationUpgradeFromNminus1(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nminus1.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite for n-1 setup: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE pull_requests (
			id TEXT NOT NULL,
			repo TEXT NOT NULL,
			number INTEGER NOT NULL,
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			url TEXT NOT NULL,
			author TEXT NOT NULL,
			labels_json TEXT NOT NULL,
			files_changed_json TEXT NOT NULL,
			review_status TEXT NOT NULL,
			ci_status TEXT NOT NULL,
			mergeable TEXT NOT NULL,
			base_branch TEXT NOT NULL,
			head_branch TEXT NOT NULL,
			cluster_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			is_draft INTEGER NOT NULL,
			is_bot INTEGER NOT NULL,
			additions INTEGER NOT NULL,
			deletions INTEGER NOT NULL,
			changed_files_count INTEGER NOT NULL,
			PRIMARY KEY (repo, number)
		)
	`)
	if err != nil {
		db.Close()
		t.Fatalf("create pre-migration schema: %v", err)
	}
	db.Close()

	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store for n-1 upgrade: %v", err)
	}
	defer store.Close()

	var userVersion int
	if err := store.db.QueryRow(`PRAGMA user_version;`).Scan(&userVersion); err != nil {
		t.Fatalf("query user_version after upgrade: %v", err)
	}
	if userVersion != supportedSchemaVersion {
		t.Fatalf("expected user_version %d after n-1 upgrade, got %d", supportedSchemaVersion, userVersion)
	}

	var version int
	var name string
	if err := store.db.QueryRow(`SELECT version, name FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version, &name); err != nil {
		t.Fatalf("query schema_migrations after upgrade: %v", err)
	}
	if version != supportedSchemaVersion || name != "field_provenance" {
		t.Fatalf("expected migration version=%d name=field_provenance, got version=%d name=%s", supportedSchemaVersion, version, name)
	}

	requiredTables := []string{
		"schema_migrations",
		"pull_requests",
		"pr_files",
		"pr_reviews",
		"ci_status",
		"sync_jobs",
		"sync_progress",
		"merged_pr_index",
		"audit_log",
	}

	for _, table := range requiredTables {
		var exists string
		if err := store.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&exists); err != nil {
			t.Fatalf("check table %s exists after upgrade: %v", table, err)
		}
	}

	pr := samplePR(42)
	if err := store.UpsertPR(pr); err != nil {
		t.Fatalf("upsert pr after n-1 upgrade: %v", err)
	}

	prs, err := store.ListPRs(PRFilter{Repo: "owner/repo"})
	if err != nil {
		t.Fatalf("list prs after n-1 upgrade: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 pr after n-1 upgrade, got %d", len(prs))
	}
}

func TestMigrationUpgradeFromNminus2(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nminus2.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite for n-2 setup: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE sync_progress (
			repo TEXT PRIMARY KEY,
			cursor TEXT NOT NULL DEFAULT '',
			processed_prs INTEGER NOT NULL DEFAULT 0,
			total_prs INTEGER NOT NULL DEFAULT 0,
			last_sync_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL
		)
	`)
	if err != nil {
		db.Close()
		t.Fatalf("create n-2 schema: %v", err)
	}

	_, err = db.Exec(`INSERT INTO sync_progress (repo, cursor, processed_prs, total_prs, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"test/repo", "cursor-123", 50, 100, "2026-03-12T10:00:00Z")
	if err != nil {
		db.Close()
		t.Fatalf("insert n-2 test data: %v", err)
	}

	db.Close()

	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store for n-2 upgrade: %v", err)
	}
	defer store.Close()

	var userVersion int
	if err := store.db.QueryRow(`PRAGMA user_version;`).Scan(&userVersion); err != nil {
		t.Fatalf("query user_version after n-2 upgrade: %v", err)
	}
	if userVersion != supportedSchemaVersion {
		t.Fatalf("expected user_version %d after n-2 upgrade, got %d", supportedSchemaVersion, userVersion)
	}

	var repo string
	var cursor string
	var processed int
	if err := store.db.QueryRow(`SELECT repo, cursor, processed_prs FROM sync_progress WHERE repo = ?`, "test/repo").Scan(&repo, &cursor, &processed); err != nil {
		t.Fatalf("query persisted sync data after n-2 upgrade: %v", err)
	}
	if cursor != "cursor-123" || processed != 50 {
		t.Fatalf("sync data not preserved through upgrade: cursor=%s processed=%d", cursor, processed)
	}

	requiredTables := []string{
		"schema_migrations",
		"pull_requests",
		"pr_files",
		"pr_reviews",
		"ci_status",
		"sync_jobs",
		"sync_progress",
		"merged_pr_index",
	}

	for _, table := range requiredTables {
		var exists string
		if err := store.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&exists); err != nil {
			t.Fatalf("check table %s exists after n-2 upgrade: %v", table, err)
		}
	}

	pr := samplePR(99)
	if err := store.UpsertPR(pr); err != nil {
		t.Fatalf("upsert pr after n-2 upgrade: %v", err)
	}

	prs, err := store.ListPRs(PRFilter{Repo: "owner/repo"})
	if err != nil {
		t.Fatalf("list prs after n-2 upgrade: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 pr after n-2 upgrade, got %d", len(prs))
	}
}

func TestMigrationFailFastOnFutureSchema(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "future.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite for future setup: %v", err)
	}

	futureVersion := supportedSchemaVersion + 1
	schema := []string{
		`CREATE TABLE schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL
		)`,
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (1, 'baseline', '2026-03-12T00:00:00Z')`,
		fmt.Sprintf(`PRAGMA user_version = %d;`, futureVersion),
		`CREATE TABLE pull_requests (
			id TEXT NOT NULL,
			repo TEXT NOT NULL,
			number INTEGER NOT NULL,
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			url TEXT NOT NULL,
			author TEXT NOT NULL,
			labels_json TEXT NOT NULL,
			files_changed_json TEXT NOT NULL,
			review_status TEXT NOT NULL,
			ci_status TEXT NOT NULL,
			mergeable TEXT NOT NULL,
			base_branch TEXT NOT NULL,
			head_branch TEXT NOT NULL,
			cluster_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			is_draft INTEGER NOT NULL,
			is_bot INTEGER NOT NULL,
			additions INTEGER NOT NULL,
			deletions INTEGER NOT NULL,
			changed_files_count INTEGER NOT NULL,
			PRIMARY KEY (repo, number)
		)`,
	}

	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			t.Fatalf("create future schema: %v", err)
		}
	}
	db.Close()

	_, err = Open(path)
	if err == nil {
		t.Fatalf("expected error when opening future schema database, got nil")
	}

	errMsg := err.Error()
	if !(contains(errMsg, "version") || contains(errMsg, "schema") || contains(errMsg, "unsupported") || contains(errMsg, "newer")) {
		t.Fatalf("expected version incompatibility error, got: %v", err)
	}

	db2, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("verify db file integrity: %v", err)
	}
	defer db2.Close()

	var actualVersion int
	if err := db2.QueryRow(`PRAGMA user_version;`).Scan(&actualVersion); err != nil {
		t.Fatalf("verify user_version preserved: %v", err)
	}
	if actualVersion != futureVersion {
		t.Fatalf("expected user_version %d preserved, got %d", futureVersion, actualVersion)
	}
}

func TestCachePauseAndListPausedSyncJobs(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	store.now = func() time.Time {
		return mustParseTime(t, "2026-04-02T10:00:00Z")
	}

	job, err := store.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	if err := store.UpdateSyncJobProgress(job.ID, SyncProgress{
		Cursor:       "cursor-1",
		ProcessedPRs: 10,
		TotalPRs:     100,
	}); err != nil {
		t.Fatalf("update sync job progress: %v", err)
	}

	pauseTime := mustParseTime(t, "2026-04-02T12:00:00Z")
	pauseReason := "rate limit exhausted"
	if err := store.PauseSyncJob(job.ID, pauseTime, pauseReason); err != nil {
		t.Fatalf("pause sync job: %v", err)
	}

	got, err := store.GetSyncJob(job.ID)
	if err != nil {
		t.Fatalf("get sync job after pause: %v", err)
	}
	if got.Status != "paused" {
		t.Fatalf("expected status 'paused', got %q", got.Status)
	}
	if got.Error != pauseReason {
		t.Fatalf("expected error %q, got %q", pauseReason, got.Error)
	}

	pausedJobs, err := store.ListPausedSyncJobs()
	if err != nil {
		t.Fatalf("list paused sync jobs: %v", err)
	}
	if len(pausedJobs) != 1 {
		t.Fatalf("expected 1 paused job, got %d", len(pausedJobs))
	}
	if pausedJobs[0].ID != job.ID {
		t.Fatalf("expected job ID %q, got %q", job.ID, pausedJobs[0].ID)
	}
	if pausedJobs[0].Error != pauseReason {
		t.Fatalf("expected error %q, got %q", pauseReason, pausedJobs[0].Error)
	}

	job2, err := store.CreateSyncJob("owner/repo2")
	if err != nil {
		t.Fatalf("create second sync job: %v", err)
	}
	if err := store.PauseSyncJob(job2.ID, mustParseTime(t, "2026-04-02T14:00:00Z"), "another limit"); err != nil {
		t.Fatalf("pause second sync job: %v", err)
	}

	pausedJobs, err = store.ListPausedSyncJobs()
	if err != nil {
		t.Fatalf("list paused sync jobs after second pause: %v", err)
	}
	if len(pausedJobs) != 2 {
		t.Fatalf("expected 2 paused jobs, got %d", len(pausedJobs))
	}
	if pausedJobs[0].ID != job.ID || pausedJobs[1].ID != job2.ID {
		t.Fatalf("expected jobs in order by updated_at, got %q then %q", pausedJobs[0].ID, pausedJobs[1].ID)
	}
}

func TestCachePauseWithZeroTime(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	job, err := store.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	if err := store.PauseSyncJob(job.ID, time.Time{}, "zero time test"); err != nil {
		t.Fatalf("pause sync job with zero time: %v", err)
	}

	pausedJobs, err := store.ListPausedSyncJobs()
	if err != nil {
		t.Fatalf("list paused sync jobs: %v", err)
	}
	if len(pausedJobs) != 1 {
		t.Fatalf("expected 1 paused job, got %d", len(pausedJobs))
	}
	if pausedJobs[0].Error != "zero time test" {
		t.Fatalf("expected error %q, got %q", "zero time test", pausedJobs[0].Error)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestMigrationIdempotency(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idempotent.db")

	store1, err := Open(path)
	if err != nil {
		t.Fatalf("open store1: %v", err)
	}

	pr := samplePR(1)
	if err := store1.UpsertPR(pr); err != nil {
		store1.Close()
		t.Fatalf("upsert pr: %v", err)
	}
	store1.Close()

	store2, err := Open(path)
	if err != nil {
		t.Fatalf("re-open store: %v", err)
	}
	defer store2.Close()

	prs, err := store2.ListPRs(PRFilter{Repo: "owner/repo"})
	if err != nil {
		t.Fatalf("list prs after reopen: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 pr after reopen, got %d", len(prs))
	}

	var count int
	if err := store2.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != 4 {
		t.Fatalf("expected 4 migration records, got %d", count)
	}

	var userVersion int
	if err := store2.db.QueryRow(`PRAGMA user_version;`).Scan(&userVersion); err != nil {
		t.Fatalf("query user_version: %v", err)
	}
	if userVersion != supportedSchemaVersion {
		t.Fatalf("expected user_version %d, got %d", supportedSchemaVersion, userVersion)
	}
}

func TestMigrationSyncJobsPersistAcrossUpgrade(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "sync-persist.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE sync_jobs (
			id TEXT PRIMARY KEY,
			repo TEXT NOT NULL,
			status TEXT NOT NULL,
			error_message TEXT NOT NULL DEFAULT '',
			last_sync_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)
	`)
	if err != nil {
		db.Close()
		t.Fatalf("create sync_jobs: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO sync_jobs (id, repo, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, "test-job-1", "test/repo", "in_progress", "2026-03-12T10:00:00Z", "2026-03-12T10:00:00Z")
	if err != nil {
		db.Close()
		t.Fatalf("insert sync job: %v", err)
	}
	db.Close()

	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store for upgrade: %v", err)
	}
	defer store.Close()

	job, err := store.GetSyncJob("test-job-1")
	if err != nil {
		t.Fatalf("get sync job after upgrade: %v", err)
	}
	if job.Repo != "test/repo" {
		t.Fatalf("expected repo test/repo, got %s", job.Repo)
	}
}
