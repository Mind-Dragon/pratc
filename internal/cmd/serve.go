package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jeffersonnunn/pratc/internal/app"
	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/formula"
	"github.com/jeffersonnunn/pratc/internal/github"
	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/monitor/data"
	"github.com/jeffersonnunn/pratc/internal/monitor/server"
	prsync "github.com/jeffersonnunn/pratc/internal/sync"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"
)

// rootCmdRef is a function to access rootCmd from root.go
var rootCmdRef = func() *cobra.Command { return nil }

type settingsStore interface {
	Get(ctx context.Context, repo string) (map[string]any, error)
	Set(ctx context.Context, scope, repo, key string, value any) error
	Delete(ctx context.Context, scope, repo, key string) error
	ValidateSet(ctx context.Context, scope, repo, key string, value any) error
	ExportYAML(ctx context.Context, scope, repo string) ([]byte, error)
	ImportYAML(ctx context.Context, scope, repo string, content []byte) error
}

type repoSyncAPI interface {
	Start(repo string) error
	Stream(repo string, w http.ResponseWriter, r *http.Request)
}

var newRepoSyncManager = func(jobDBPath, jobID string) *prsync.Manager {
	var jobRecorder prsync.JobRecorder
	if strings.TrimSpace(jobDBPath) != "" {
		jobRecorder = prsync.NewDBJobRecorder(jobDBPath)
	}
	dbPath := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = filepath.Join(home, ".pratc", "pratc.db")
	}
	cacheStore, _ := cache.Open(dbPath)
	return prsync.NewManager(prsync.NewDefaultRunner(jobRecorder, jobID, cacheStore))
}

func RegisterServeCommand() {
	var port int
	var repo string
	var useCacheFirst bool

	command := &cobra.Command{
		Use:   "serve",
		Short: "Serve the prATC API",
		RunE: func(cmd *cobra.Command, args []string) error {
			requestID := uuid.New().String()
			ctx := logger.ContextWithRequestID(cmd.Context(), requestID)
			log := logger.FromContext(ctx)
			log.Info("starting server", "port", port, "repo", repo)
			return runServer(ctx, port, repo, useCacheFirst)
		},
	}
	command.Flags().IntVar(&port, "port", 8080, "Port to bind the API server to")
	command.Flags().StringVar(&repo, "repo", "", "Optional default repository for API routes")
	command.Flags().BoolVar(&useCacheFirst, "use-cache-first", false, "Check cache before live fetch for analyze/cluster endpoints")
	rootCmd.AddCommand(command)
}

func writeHTTPJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeHTTPError(w http.ResponseWriter, status int, message string) {
	writeHTTPJSON(w, status, map[string]string{"error": message})
}

// sanitizedError returns a user-safe error message based on the error type.
// It preserves specific, helpful error messages while preventing information leakage.
// Full error details are logged server-side with request ID correlation.
func sanitizedError(err error) string {
	if err == nil {
		return "internal server error"
	}

	errMsg := err.Error()

	// Known safe error patterns that provide useful feedback without leaking internals
	safeMessages := map[string]string{
		// Settings errors
		"key is required":   "key is required",
		"scope is required": "scope is required",
		"invalid scope":     "invalid scope",
		"invalid key":       "invalid key",
		"value is required": "value is required",
		// Repo errors
		"repo is required":     "repo is required",
		"invalid repo format":  "invalid repo format",
		"repository not found": "repository not found",
		"repo not found":       "repo not found",
		// Sync errors
		"sync in progress":   "sync already in progress",
		"no active sync job": "no active sync job",
		"sync job not found": "sync job not found",
		// Plan errors
		"invalid target parameter": "invalid target parameter",
		"target must be positive":  "target must be a positive number",
		"invalid mode":             "invalid mode",
		// Auth errors
		"missing API key":     "missing API key",
		"invalid API key":     "invalid API key",
		"API key unavailable": "API key unavailable",
		// General safe messages
		"route not found":                 "route not found",
		"method not allowed":              "method not allowed",
		"invalid JSON body":               "invalid JSON body",
		"failed to read request body":     "failed to read request body",
		"streaming not supported":         "streaming not supported",
		"sync API unavailable":            "sync API unavailable",
		"review endpoint not implemented": "review endpoint not implemented",
		"analysis unavailable":            "analysis unavailable",
	}

	// Check for exact match first
	if msg, ok := safeMessages[errMsg]; ok {
		return msg
	}

	// Check for substring matches that indicate safe errors
	substringPatterns := []string{
		"key is required",
		"repo is required",
		"scope is required",
		"invalid JSON",
		"not found",
		"route not found",
	}

	for _, pattern := range substringPatterns {
		if strings.Contains(strings.ToLower(errMsg), strings.ToLower(pattern)) {
			return errMsg
		}
	}

	// Unknown error - return generic message, log full details server-side
	return "internal server error"
}

func ensureGET(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func ensureRepo(w http.ResponseWriter, repo string) bool {
	if strings.TrimSpace(repo) == "" {
		writeHTTPError(w, http.StatusBadRequest, "repo is required")
		return false
	}
	return true
}

func repoFromQuery(r *http.Request, defaultRepo string) string {
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	if repo == "" {
		repo = defaultRepo
	}
	return repo
}

type setSettingRequest struct {
	Scope string `json:"scope"`
	Repo  string `json:"repo"`
	Key   string `json:"key"`
	Value any    `json:"value"`
}

func handleSettings(w http.ResponseWriter, r *http.Request, store settingsStore) {
	log := logger.FromContext(r.Context())
	switch r.Method {
	case http.MethodGet:
		repo := strings.TrimSpace(r.URL.Query().Get("repo"))
		payload, err := store.Get(r.Context(), repo)
		if err != nil {
			log.Error("handleSettings: store.Get failed", "error", err.Error(), "repo", repo)
			writeHTTPError(w, http.StatusInternalServerError, sanitizedError(err))
			return
		}
		writeHTTPJSON(w, http.StatusOK, payload)
	case http.MethodPost:
		var req setSettingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if strings.TrimSpace(req.Key) == "" {
			writeHTTPError(w, http.StatusBadRequest, "key is required")
			return
		}
		validateOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("validateOnly")), "true")
		if validateOnly {
			if err := store.ValidateSet(r.Context(), req.Scope, req.Repo, req.Key, req.Value); err != nil {
				log.Error("handleSettings: ValidateSet failed", "error", err.Error(), "scope", req.Scope, "repo", req.Repo, "key", req.Key)
				writeHTTPError(w, http.StatusBadRequest, sanitizedError(err))
				return
			}
			writeHTTPJSON(w, http.StatusOK, map[string]any{"valid": true})
			return
		}
		if err := store.Set(r.Context(), req.Scope, req.Repo, req.Key, req.Value); err != nil {
			log.Error("handleSettings: Set failed", "error", err.Error(), "scope", req.Scope, "repo", req.Repo, "key", req.Key)
			writeHTTPError(w, http.StatusBadRequest, sanitizedError(err))
			return
		}
		writeHTTPJSON(w, http.StatusOK, map[string]any{"updated": true})
	case http.MethodDelete:
		scope := strings.TrimSpace(r.URL.Query().Get("scope"))
		repo := strings.TrimSpace(r.URL.Query().Get("repo"))
		key := strings.TrimSpace(r.URL.Query().Get("key"))
		if key == "" {
			writeHTTPError(w, http.StatusBadRequest, "key is required")
			return
		}
		if err := store.Delete(r.Context(), scope, repo, key); err != nil {
			log.Error("handleSettings: Delete failed", "error", err.Error(), "scope", scope, "repo", repo, "key", key)
			writeHTTPError(w, http.StatusBadRequest, sanitizedError(err))
			return
		}
		writeHTTPJSON(w, http.StatusOK, map[string]any{"deleted": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleExportSettings(w http.ResponseWriter, r *http.Request, store settingsStore) {
	if !ensureGET(w, r) {
		return
	}
	log := logger.FromContext(r.Context())
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	content, err := store.ExportYAML(r.Context(), scope, repo)
	if err != nil {
		log.Error("handleExportSettings: ExportYAML failed", "error", err.Error(), "scope", scope, "repo", repo)
		writeHTTPError(w, http.StatusBadRequest, sanitizedError(err))
		return
	}
	w.Header().Set("Content-Type", "application/x-yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func handleImportSettings(w http.ResponseWriter, r *http.Request, store settingsStore) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	log := logger.FromContext(r.Context())
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	if err := store.ImportYAML(r.Context(), scope, repo, body); err != nil {
		log.Error("handleImportSettings: ImportYAML failed", "error", err.Error(), "scope", scope, "repo", repo)
		writeHTTPError(w, http.StatusBadRequest, sanitizedError(err))
		return
	}
	writeHTTPJSON(w, http.StatusOK, map[string]any{"imported": true})
}

func handleListSyncJobs(w http.ResponseWriter, r *http.Request) {
	if !ensureGET(w, r) {
		return
	}
	log := logger.FromContext(r.Context())
	store, err := cache.Open(analyzeSyncDBPath())
	if err != nil {
		log.Error("handleListSyncJobs: cache.Open failed", "error", err.Error())
		writeHTTPError(w, http.StatusInternalServerError, sanitizedError(err))
		return
	}
	defer store.Close()

	jobs, err := store.ListSyncJobs()
	if err != nil {
		log.Error("handleListSyncJobs: ListSyncJobs failed", "error", err.Error())
		writeHTTPError(w, http.StatusInternalServerError, sanitizedError(err))
		return
	}
	writeHTTPJSON(w, http.StatusOK, jobs)
}

func handleListPausedSyncJobs(w http.ResponseWriter, r *http.Request) {
	if !ensureGET(w, r) {
		return
	}
	log := logger.FromContext(r.Context())
	store, err := cache.Open(analyzeSyncDBPath())
	if err != nil {
		log.Error("handleListPausedSyncJobs: cache.Open failed", "error", err.Error())
		writeHTTPError(w, http.StatusInternalServerError, sanitizedError(err))
		return
	}
	defer store.Close()

	jobs, err := store.ListPausedSyncJobs()
	if err != nil {
		log.Error("handleListPausedSyncJobs: ListPausedSyncJobs failed", "error", err.Error())
		writeHTTPError(w, http.StatusInternalServerError, sanitizedError(err))
		return
	}
	writeHTTPJSON(w, http.StatusOK, jobs)
}

func handleSyncEvents(store *cache.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ensureGET(w, r) {
			return
		}
		log := logger.FromContext(r.Context())
		repo := r.URL.Query().Get("repo")
		if !ensureRepo(w, repo) {
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		job, ok, err := store.ResumeSyncJob(repo)
		if err != nil {
			log.Error("handleSyncEvents: ResumeSyncJob failed", "error", err.Error(), "repo", repo)
			writeHTTPError(w, http.StatusInternalServerError, sanitizedError(err))
			return
		}
		if !ok {
			writeHTTPError(w, http.StatusNotFound, "no active sync job")
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeHTTPError(w, http.StatusInternalServerError, "streaming not supported")
			return
		}

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		done := r.Context().Done()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				progress := job.Progress
				percent := 0
				if progress.TotalPRs > 0 {
					percent = (progress.ProcessedPRs * 100) / progress.TotalPRs
				}
				data := map[string]any{
					"processed": progress.ProcessedPRs,
					"total":     progress.TotalPRs,
					"percent":   percent,
				}
				fmt.Fprintf(w, "data: %s\n\n", mustMarshalJSON(data))
				flusher.Flush()
			}
		}
	}
}

func mustMarshalJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func handleAnalyze(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}
	log := logger.FromContext(r.Context())

	// Check for active sync job — return 202 Accepted if one is running
	hasActive, jobID, err := service.GetActiveSyncJob(repo)
	if err == nil && hasActive {
		writeHTTPJSON(w, http.StatusAccepted, map[string]any{
			"sync_status": "running",
			"job_id":      jobID,
			"message":     "sync in progress, retry after completion",
		})
		return
	}

	result, err := service.Analyze(r.Context(), repo)
	if err != nil {
		log.Error("handleAnalyze: service.Analyze failed", "error", err.Error(), "repo", repo)
		writeHTTPError(w, http.StatusInternalServerError, sanitizedError(err))
		return
	}
	writeHTTPJSON(w, http.StatusOK, result)
}

func handleCluster(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}
	log := logger.FromContext(r.Context())
	result, err := service.Cluster(r.Context(), repo)
	if err != nil {
		log.Error("handleCluster: service.Cluster failed", "error", err.Error(), "repo", repo)
		writeHTTPError(w, http.StatusInternalServerError, sanitizedError(err))
		return
	}
	writeHTTPJSON(w, http.StatusOK, result)
}

func handleGraph(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}
	log := logger.FromContext(r.Context())
	result, err := service.Graph(r.Context(), repo)
	if err != nil {
		log.Error("handleGraph: service.Graph failed", "error", err.Error(), "repo", repo)
		writeHTTPError(w, http.StatusInternalServerError, sanitizedError(err))
		return
	}
	writeHTTPJSON(w, http.StatusOK, result)
}

func handlePlan(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}
	log := logger.FromContext(r.Context())
	target := types.DefaultTarget
	if t := r.URL.Query().Get("target"); t != "" {
		if parsed, err := strconv.Atoi(t); err == nil && parsed > 0 {
			target = parsed
		} else {
			writeHTTPError(w, http.StatusBadRequest, "target must be a positive integer")
			return
		}
	}
	mode := formula.ModeCombination
	if m := r.URL.Query().Get("mode"); m != "" {
		switch m {
		case "combination":
			mode = formula.ModeCombination
		case "permutation":
			mode = formula.ModePermutation
		case "with_replacement":
			mode = formula.ModeWithReplacement
		default:
			writeHTTPError(w, http.StatusBadRequest, "invalid mode")
			return
		}
	}
	if v := r.URL.Query().Get("exclude_conflicts"); v != "" {
		if _, err := strconv.ParseBool(v); err != nil {
			writeHTTPError(w, http.StatusBadRequest, "invalid exclude_conflicts")
			return
		}
	}
	if v := r.URL.Query().Get("stale_score_threshold"); v != "" {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil || parsed < 0 || parsed > 1 {
			writeHTTPError(w, http.StatusBadRequest, "invalid stale_score_threshold")
			return
		}
	}
	if v := r.URL.Query().Get("candidate_pool_cap"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 1 || parsed > 500 {
			writeHTTPError(w, http.StatusBadRequest, "invalid candidate_pool_cap")
			return
		}
	}
	if v := r.URL.Query().Get("score_min"); v != "" {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil || parsed < 0 || parsed > 100 {
			writeHTTPError(w, http.StatusBadRequest, "invalid score_min")
			return
		}
	}
	result, err := service.Plan(r.Context(), repo, target, mode)
	if err != nil {
		log.Error("handlePlan: service.Plan failed", "error", err.Error(), "repo", repo, "target", target, "mode", mode)
		writeHTTPError(w, http.StatusInternalServerError, sanitizedError(err))
		return
	}
	writeHTTPJSON(w, http.StatusOK, result)
}

// handleReview is not implemented - service.Review does not exist
// func handleReview(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
// 	if !ensureGET(w, r) || !ensureRepo(w, repo) {
// 		return
// 	}
// 	result, err := service.Review(r.Context(), repo)
// 	if err != nil {
// 		writeHTTPError(w, http.StatusInternalServerError, err.Error())
// 		return
// 	}
// 	writeHTTPJSON(w, http.StatusOK, result)
// }

// handlePlanOmni handles the omni-batch plan endpoint.
func handlePlanOmni(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}
	log := logger.FromContext(r.Context())
	selector := r.URL.Query().Get("selector")
	result, err := service.PlanOmni(r.Context(), repo, selector)
	if err != nil {
		log.Error("handlePlanOmni: service.PlanOmni failed", "error", err.Error(), "repo", repo)
		writeHTTPError(w, http.StatusInternalServerError, sanitizedError(err))
		return
	}
	writeHTTPJSON(w, http.StatusOK, result)
}

func parseRepoActionPath(path string) (repo, action string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(path, "/api/repos/"), "/")
	if len(parts) < 3 {
		return "", "", false
	}
	// Join remaining parts for nested actions like sync/stream
	action = strings.Join(parts[2:], "/")
	return parts[0] + "/" + parts[1], action, true
}

func getSyncStatus(repo string) map[string]any {
	status := map[string]any{
		"repo":             repo,
		"last_sync":        time.Time{},
		"pr_count":         0,
		"status":           "never",
		"progress_percent": 0,
	}

	dbPath := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = filepath.Join(home, ".pratc", "pratc.db")
	}
	store, err := cache.Open(dbPath)
	if err != nil {
		status["error"] = "cache unavailable"
		return status
	}
	defer store.Close()

	if prs, err := store.ListPRs(cache.PRFilter{Repo: repo}); err == nil {
		status["pr_count"] = len(prs)
	}

	// First check for active jobs (queued, running, resuming)
	if job, ok, err := store.ResumeSyncJob(repo); err == nil && ok {
		status["job_id"] = job.ID
		status["status"] = string(job.Status)
		status["last_sync"] = job.LastSyncAt

		// Include resume_at for paused_rate_limit jobs
		if job.Status == cache.SyncJobStatusPausedRateLimit {
			if job.Progress.ScheduledResumeAt.IsZero() {
				if pausedJob, err := store.GetPausedSyncJobByRepo(repo); err == nil {
					job.Progress.ScheduledResumeAt = pausedJob.Progress.ScheduledResumeAt
				}
			}
			if !job.Progress.ScheduledResumeAt.IsZero() {
				status["resume_at"] = job.Progress.ScheduledResumeAt
			}
		}

		// Include error for failed jobs
		if job.Status == cache.SyncJobStatusFailed {
			if job.Error == "" {
				if latestJob, ok, err := store.GetLatestSyncJob(repo); err == nil && ok {
					job.Error = latestJob.Error
				}
			}
			if job.Error != "" {
				status["error"] = job.Error
			}
		}

		if job.Progress.TotalPRs > 0 {
			percent := (job.Progress.ProcessedPRs * 100) / job.Progress.TotalPRs
			if percent < 0 {
				percent = 0
			}
			if percent > 100 {
				percent = 100
			}
			status["progress_percent"] = percent
		}
		return status
	}

	// Check for paused_rate_limit or failed jobs using GetLatestSyncJob
	if job, ok, err := store.GetLatestSyncJob(repo); err == nil && ok {
		status["job_id"] = job.ID
		status["status"] = string(job.Status)
		status["last_sync"] = job.LastSyncAt

		// Include resume_at for paused_rate_limit jobs
		if job.Status == cache.SyncJobStatusPausedRateLimit {
			if job.Progress.ScheduledResumeAt.IsZero() {
				if pausedJob, err := store.GetPausedSyncJobByRepo(repo); err == nil {
					job.Progress.ScheduledResumeAt = pausedJob.Progress.ScheduledResumeAt
				}
			}
			if !job.Progress.ScheduledResumeAt.IsZero() {
				status["resume_at"] = job.Progress.ScheduledResumeAt
			}
		}

		// Include error for failed jobs
		if job.Status == cache.SyncJobStatusFailed {
			if job.Error == "" {
				if latestJob, ok, err := store.GetLatestSyncJob(repo); err == nil && ok {
					job.Error = latestJob.Error
				}
			}
			if job.Error != "" {
				status["error"] = job.Error
			}
		}

		// For completed jobs, always report 100% progress regardless of progress values
		if job.Status == cache.SyncJobStatusCompleted {
			status["progress_percent"] = 100
			return status
		}

		if job.Progress.TotalPRs > 0 {
			percent := (job.Progress.ProcessedPRs * 100) / job.Progress.TotalPRs
			if percent < 0 {
				percent = 0
			}
			if percent > 100 {
				percent = 100
			}
			status["progress_percent"] = percent
		}
		return status
	}

	lastSync, err := store.LastSync(repo)
	if err != nil {
		// Log but don't expose in status response
		return status
	}
	if !lastSync.IsZero() {
		status["last_sync"] = lastSync
		status["status"] = "completed"
		status["progress_percent"] = 100
	}

	return status
}

func handleRepoAction(w http.ResponseWriter, r *http.Request, service app.Service, syncAPI repoSyncAPI) {
	log := logger.FromContext(r.Context())
	repo, action, ok := parseRepoActionPath(r.URL.Path)
	if !ok {
		writeHTTPError(w, http.StatusNotFound, "route not found")
		return
	}

	switch action {
	case "analyze":
		handleAnalyze(w, r, service, repo)
	case "cluster":
		handleCluster(w, r, service, repo)
	case "graph":
		handleGraph(w, r, service, repo)
	case "plan", "plans":
		handlePlan(w, r, service, repo)
	case "sync":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if syncAPI == nil {
			writeHTTPError(w, http.StatusInternalServerError, "sync API unavailable")
			return
		}
		if err := syncAPI.Start(repo); err != nil {
			log.Error("handleRepoAction: syncAPI.Start failed", "error", err.Error(), "repo", repo)
			writeHTTPError(w, http.StatusInternalServerError, sanitizedError(err))
			return
		}
		writeHTTPJSON(w, http.StatusAccepted, map[string]any{"started": true, "repo": repo})
	case "sync/stream":
		if syncAPI == nil {
			writeHTTPError(w, http.StatusInternalServerError, "sync API unavailable")
			return
		}
		syncAPI.Stream(repo, w, r)
	case "sync/status":
		syncStatus := getSyncStatus(repo)
		writeHTTPJSON(w, http.StatusOK, syncStatus)
	// case "review":
	// handleReview is not implemented
	case "review":
		writeHTTPError(w, http.StatusNotImplemented, "review endpoint not implemented")
	default:
		writeHTTPError(w, http.StatusNotFound, "route not found")
	}
}

// corsAllowedOrigins returns the list of allowed CORS origins from environment.
func corsAllowedOrigins() []string {
	env := strings.TrimSpace(os.Getenv("PRATC_CORS_ALLOWED_ORIGINS"))
	if env == "" {
		return []string{"http://localhost:3000"}
	}
	parts := strings.Split(env, ",")
	allowed := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			allowed = append(allowed, trimmed)
		}
	}
	if len(allowed) == 0 {
		return []string{"http://localhost:3000"}
	}
	return allowed
}

// isOriginAllowed checks if the given origin is in the allowlist.
func isOriginAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return false
	}
	for _, a := range allowed {
		if a == origin {
			return true
		}
	}
	return false
}

func corsMiddleware(next http.Handler) http.Handler {
	allowedOrigins := corsAllowedOrigins()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if r.Method == http.MethodOptions {
			if isOriginAllowed(origin, allowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if isOriginAllowed(origin, allowedOrigins) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		next.ServeHTTP(w, r)
	})
}

// isHealthCheckPath returns true if the path is a health check endpoint.
func isHealthCheckPath(path string) bool {
	return path == "/healthz" || path == "/api/health"
}

// requestIDMiddleware generates a unique request ID for each request and adds it to the context.
// The request ID is also returned in the X-Request-ID response header for client correlation.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		ctx := logger.ContextWithRequestID(r.Context(), requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authMiddleware checks X-API-Key header on all non-healthz routes.
// rateLimiterConfig holds the configured rate limits.
type rateLimiterConfig struct {
	general   rate.Limit
	critical  rate.Limit
	burstSize int
}

// parseRateLimitConfig parses rate limit configuration from environment variables.
// Defaults: 100 requests/minute for general, 10 requests/minute for critical.
func parseRateLimitConfig() rateLimiterConfig {
	cfg := rateLimiterConfig{
		general:   rate.Inf,
		critical:  rate.Inf,
		burstSize: 10,
	}

	if generalStr := strings.TrimSpace(os.Getenv("PRATC_RATE_LIMIT_GENERAL")); generalStr != "" {
		if requestsPerMin, err := strconv.Atoi(generalStr); err == nil && requestsPerMin > 0 {
			cfg.general = rate.Limit(float64(requestsPerMin) / 60.0)
			cfg.burstSize = requestsPerMin / 10
			if cfg.burstSize < 1 {
				cfg.burstSize = 1
			}
		}
	}

	if criticalStr := strings.TrimSpace(os.Getenv("PRATC_RATE_LIMIT_CRITICAL")); criticalStr != "" {
		if requestsPerMin, err := strconv.Atoi(criticalStr); err == nil && requestsPerMin > 0 {
			cfg.critical = rate.Limit(float64(requestsPerMin) / 60.0)
		}
	}

	return cfg
}

// limiterEntry holds rate limiters for a single IP.
type limiterEntry struct {
	general  *rate.Limiter
	critical *rate.Limiter
	lastSeen time.Time
}

// ipRateLimiter manages per-IP rate limiters with automatic cleanup.
type ipRateLimiter struct {
	cfg      rateLimiterConfig
	limiters sync.Map // map[string]*limiterEntry
	mu       sync.Mutex
	stopCh   chan struct{}
}

// newIPRateLimiter creates a new IP rate limiter with periodic cleanup.
func newIPRateLimiter(cfg rateLimiterConfig) *ipRateLimiter {
	rl := &ipRateLimiter{
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// cleanupLoop periodically removes stale rate limiters to prevent memory growth.
func (rl *ipRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCh:
			return
		}
	}
}

// cleanup removes rate limiters that haven't been used in 10 minutes.
func (rl *ipRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	threshold := time.Now().Add(-10 * time.Minute)
	rl.limiters.Range(func(key, value any) bool {
		entry := value.(*limiterEntry)
		if entry.lastSeen.Before(threshold) {
			rl.limiters.Delete(key)
		}
		return true
	})
}

// Stop stops the cleanup goroutine.
func (rl *ipRateLimiter) Stop() {
	close(rl.stopCh)
}

// getLimiter retrieves or creates a rate limiter entry for the given IP.
func (rl *ipRateLimiter) getLimiter(ip string) *limiterEntry {
	if entry, ok := rl.limiters.Load(ip); ok {
		entry.(*limiterEntry).lastSeen = time.Now()
		return entry.(*limiterEntry)
	}

	entry := &limiterEntry{
		general:  rate.NewLimiter(rl.cfg.general, rl.cfg.burstSize),
		critical: rate.NewLimiter(rl.cfg.critical, rl.cfg.burstSize/2+1),
		lastSeen: time.Now(),
	}
	actual, _ := rl.limiters.LoadOrStore(ip, entry)
	return actual.(*limiterEntry)
}

// isCriticalEndpoint returns true if the path is a critical endpoint requiring stricter limits.
func isCriticalEndpoint(path string) bool {
	return strings.HasPrefix(path, "/analyze") ||
		strings.HasPrefix(path, "/sync") ||
		strings.Contains(path, "/analyze") ||
		strings.Contains(path, "/sync")
}

// rateLimitMiddleware creates middleware that enforces per-IP rate limiting.
func rateLimitMiddleware(rl *ipRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip rate limiting for health checks
			if isHealthCheckPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			ip := getClientIP(r)
			entry := rl.getLimiter(ip)

			var limiter *rate.Limiter
			if isCriticalEndpoint(r.URL.Path) {
				limiter = entry.critical
			} else {
				limiter = entry.general
			}

			if !limiter.Allow() {
				retryAfter := "60"
				w.Header().Set("Retry-After", retryAfter)
				w.Header().Set("X-RateLimit-Limit", getRateLimitHeader(rl, r.URL.Path))
				writeHTTPError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the client IP from the request, checking X-Forwarded-For and X-Real-IP headers.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxied requests)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// getRateLimitHeader returns the applicable rate limit for the endpoint.
func getRateLimitHeader(rl *ipRateLimiter, path string) string {
	if isCriticalEndpoint(path) {
		requestsPerMin := int(rl.cfg.critical * 60)
		if requestsPerMin == 0 {
			return "unlimited"
		}
		return fmt.Sprintf("%d/min", requestsPerMin)
	}
	requestsPerMin := int(rl.cfg.general * 60)
	if requestsPerMin == 0 {
		return "unlimited"
	}
	return fmt.Sprintf("%d/min", requestsPerMin)
}

// authMiddleware checks X-API-Key header on all non-healthz routes.
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isHealthCheckPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		apiKey, err := getAPIKey()
		if err != nil {
			writeHTTPError(w, http.StatusInternalServerError, "API key unavailable")
			return
		}

		providedKey := r.Header.Get("X-API-Key")
		if providedKey == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "missing API key"})
			return
		}

		if providedKey != apiKey {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid API key"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func runServer(ctx context.Context, port int, defaultRepo string, useCacheFirst bool) error {
	token, err := github.ResolveToken(ctx)
	if err != nil {
		return err
	}

	dbPath := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = filepath.Join(home, ".pratc", "pratc.db")
	}
	cacheStore, err := cache.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open cache store: %w", err)
	}
	defer cacheStore.Close()

	service := app.NewService(buildCacheFirstConfig(useCacheFirst, false, cacheStore))
	settingsStore, err := openSettingsStore()
	if err != nil {
		return err
	}
	defer settingsStore.Close()

	repoSync := newRepoSyncManager("", "")

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeHTTPJSON(w, http.StatusOK, service.Health())
	})
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeHTTPJSON(w, http.StatusOK, service.Health())
	})
	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		handleSettings(w, r, settingsStore)
	})
	mux.HandleFunc("/api/settings/export", func(w http.ResponseWriter, r *http.Request) {
		handleExportSettings(w, r, settingsStore)
	})
	mux.HandleFunc("/api/settings/import", func(w http.ResponseWriter, r *http.Request) {
		handleImportSettings(w, r, settingsStore)
	})

	mux.HandleFunc("/api/sync/jobs", handleListSyncJobs)
	mux.HandleFunc("/api/sync/jobs/paused", handleListPausedSyncJobs)
	mux.HandleFunc("/api/sync/events", handleSyncEvents(cacheStore))

	// Monitor WebSocket endpoint
	monitorBroadcaster := data.NewBroadcaster(
		data.NewStore(cacheStore),
		data.NewRateLimitFetcher(token),
		data.NewTimelineAggregator(cacheStore),
	)
	go monitorBroadcaster.Start(ctx)
	defer monitorBroadcaster.Stop()

	// Configure WebSocket server with same origin allowlist as CORS
	server.SetAllowedOrigins(corsAllowedOrigins())
	websocketServer := server.NewWebSocketServer(monitorBroadcaster)
	mux.HandleFunc("/monitor/stream", websocketServer.ServeHTTP)

	mux.HandleFunc("/analyze", func(w http.ResponseWriter, r *http.Request) {
		handleAnalyze(w, r, service, repoFromQuery(r, defaultRepo))
	})
	mux.HandleFunc("/cluster", func(w http.ResponseWriter, r *http.Request) {
		handleCluster(w, r, service, repoFromQuery(r, defaultRepo))
	})
	mux.HandleFunc("/graph", func(w http.ResponseWriter, r *http.Request) {
		handleGraph(w, r, service, repoFromQuery(r, defaultRepo))
	})
	mux.HandleFunc("/plan", func(w http.ResponseWriter, r *http.Request) {
		handlePlan(w, r, service, repoFromQuery(r, defaultRepo))
	})

	mux.HandleFunc("/api/repos/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/repos/") && strings.Contains(path, "/plan/omni") {
			// Extract repo from path
			parts := strings.Split(strings.TrimPrefix(path, "/api/repos/"), "/")
			if len(parts) >= 3 {
				owner := parts[0]
				repoName := parts[1]
				fullRepo := owner + "/" + repoName
				handlePlanOmni(w, r, service, fullRepo)
				return
			}
		}
		handleRepoAction(w, r, service, repoSync)
	})

	// Initialize rate limiter with config from environment
	rateLimitCfg := parseRateLimitConfig()
	rateLimiter := newIPRateLimiter(rateLimitCfg)
	defer rateLimiter.Stop()

	server := &http.Server{Addr: ":" + strconv.Itoa(port), Handler: authMiddleware(corsMiddleware(requestIDMiddleware(rateLimitMiddleware(rateLimiter)(mux))))}

	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()

	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
