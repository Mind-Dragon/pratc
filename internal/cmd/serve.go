package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	"github.com/spf13/cobra"
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
	switch r.Method {
	case http.MethodGet:
		repo := strings.TrimSpace(r.URL.Query().Get("repo"))
		payload, err := store.Get(r.Context(), repo)
		if err != nil {
			writeHTTPError(w, http.StatusInternalServerError, err.Error())
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
				writeHTTPError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeHTTPJSON(w, http.StatusOK, map[string]any{"valid": true})
			return
		}
		if err := store.Set(r.Context(), req.Scope, req.Repo, req.Key, req.Value); err != nil {
			writeHTTPError(w, http.StatusBadRequest, err.Error())
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
			writeHTTPError(w, http.StatusBadRequest, err.Error())
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
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	content, err := store.ExportYAML(r.Context(), scope, repo)
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, err.Error())
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
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeHTTPError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	if err := store.ImportYAML(r.Context(), scope, repo, body); err != nil {
		writeHTTPError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeHTTPJSON(w, http.StatusOK, map[string]any{"imported": true})
}

func handleListSyncJobs(w http.ResponseWriter, r *http.Request) {
	if !ensureGET(w, r) {
		return
	}
	store, err := cache.Open(analyzeSyncDBPath())
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer store.Close()

	jobs, err := store.ListSyncJobs()
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeHTTPJSON(w, http.StatusOK, jobs)
}

func handleListPausedSyncJobs(w http.ResponseWriter, r *http.Request) {
	if !ensureGET(w, r) {
		return
	}
	store, err := cache.Open(analyzeSyncDBPath())
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer store.Close()

	jobs, err := store.ListPausedSyncJobs()
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeHTTPJSON(w, http.StatusOK, jobs)
}

func handleSyncEvents(store *cache.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ensureGET(w, r) {
			return
		}
		repo := r.URL.Query().Get("repo")
		if !ensureRepo(w, repo) {
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		job, ok, err := store.ResumeSyncJob(repo)
		if err != nil {
			writeHTTPError(w, http.StatusInternalServerError, err.Error())
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
					"percent":    percent,
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
	result, err := service.Analyze(r.Context(), repo)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeHTTPJSON(w, http.StatusOK, result)
}

func handleCluster(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}
	result, err := service.Cluster(r.Context(), repo)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeHTTPJSON(w, http.StatusOK, result)
}

func handleGraph(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}
	result, err := service.Graph(r.Context(), repo)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeHTTPJSON(w, http.StatusOK, result)
}

func handlePlan(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}
	target := 20
	if t := r.URL.Query().Get("target"); t != "" {
		if parsed, err := strconv.Atoi(t); err == nil && parsed > 0 {
			target = parsed
		}
	}
	mode := formula.ModeCombination
	if m := r.URL.Query().Get("mode"); m != "" {
		switch m {
		case "permutation":
			mode = formula.ModePermutation
		case "with_replacement":
			mode = formula.ModeWithReplacement
		}
	}
	result, err := service.Plan(r.Context(), repo, target, mode)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
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

// handlePlanOmni is not implemented - service.PlanOmni does not exist
// func handlePlanOmni(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
// 	if !ensureGET(w, r) || !ensureRepo(w, repo) {
// 		return
// 	}
// 	target := 20
// 	if t := r.URL.Query().Get("target"); t != "" {
// 		if parsed, err := strconv.Atoi(t); err == nil && parsed > 0 {
// 			target = parsed
// 		}
// 	}
// 	result, err := service.PlanOmni(r.Context(), repo, target)
// 	if err != nil {
// 		writeHTTPError(w, http.StatusInternalServerError, err.Error())
// 		return
// 	}
// 	writeHTTPJSON(w, http.StatusOK, result)
// }

func parseRepoActionPath(path string) (repo, action string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(path, "/api/repos/"), "/")
	if len(parts) < 3 {
		return "", "", false
	}
	return parts[0] + "/" + parts[1], parts[2], true
}

func getSyncStatus(repo string) map[string]any {
	status := map[string]any{
		"repo":             repo,
		"last_sync":        time.Time{},
		"pr_count":         0,
		"status":           "never",
		"in_progress":      false,
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

	if job, ok, err := store.ResumeSyncJob(repo); err == nil && ok {
		status["status"] = "in_progress"
		status["in_progress"] = true
		status["last_sync"] = job.LastSyncAt
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
	} else if err != nil {
		status["error"] = err.Error()
		return status
	}

	lastSync, err := store.LastSync(repo)
	if err != nil {
		status["error"] = err.Error()
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
			writeHTTPError(w, http.StatusInternalServerError, err.Error())
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
		if isOriginAllowed(origin, allowedOrigins) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func runServer(ctx context.Context, port int, defaultRepo string, useCacheFirst bool) error {
	token, err := github.ResolveToken(ctx)
	if err != nil {
		return err
	}
	service := app.NewService(buildCacheFirstConfig(useCacheFirst))
	settingsStore, err := openSettingsStore()
	if err != nil {
		return err
	}
	defer settingsStore.Close()

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
			// handlePlanOmni is not implemented
			writeHTTPError(w, http.StatusNotImplemented, "plan/omni endpoint not implemented")
			return
		}
		handleRepoAction(w, r, service, repoSync)
	})

	server := &http.Server{Addr: ":" + strconv.Itoa(port), Handler: corsMiddleware(mux)}

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
