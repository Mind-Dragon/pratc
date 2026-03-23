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

	"github.com/jeffersonnunn/pratc/internal/app"
	"github.com/jeffersonnunn/pratc/internal/audit"
	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/formula"
	"github.com/jeffersonnunn/pratc/internal/repo"
	"github.com/jeffersonnunn/pratc/internal/settings"
	prsync "github.com/jeffersonnunn/pratc/internal/sync"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pratc",
	Short: "PR Air Traffic Control",
	Long:  "prATC is a CLI for pull request analysis, clustering, graphing, planning, and API serving.",
}

func ExecuteContext(ctx context.Context) {
	err := rootCmd.ExecuteContext(ctx)
	if err == nil {
		return
	}

	_, _ = fmt.Fprintln(os.Stderr, err)
	if isInvalidArgumentError(err) {
		os.Exit(2)
	}

	os.Exit(1)
}

func isInvalidArgumentError(err error) bool {
	message := err.Error()
	patterns := []string{
		"required flag",
		"unknown command",
		"unknown flag",
		"unknown shorthand flag",
		"accepts",
		"invalid value for",
		"invalid argument",
		"invalid format",
		"invalid mode",
	}

	for _, pattern := range patterns {
		if strings.Contains(message, pattern) {
			return true
		}
	}

	return false
}

func checkHasSyncData(repo string) (bool, error) {
	dbPath := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = filepath.Join(home, ".pratc", "pratc.db")
	}
	store, err := cache.Open(dbPath)
	if err != nil {
		return false, nil
	}
	defer store.Close()
	prs, err := store.ListPRs(cache.PRFilter{Repo: repo})
	if err != nil || len(prs) == 0 {
		return false, nil
	}
	return true, nil
}

func RegisterAnalyzeCommand() {
	var repo string
	var format string
	var useCacheFirst bool
	var forceLive bool

	command := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze pull requests for a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := app.Config{UseCacheFirst: useCacheFirst}
			service := app.NewService(cfg)

			if !forceLive && useCacheFirst {
				if hasSync, _ := checkHasSyncData(repo); !hasSync {
					warning := `⚠️  No recent sync data found for %s
   Starting background sync job...
   Run 'pratc sync --repo=%s --watch' to monitor progress.

`
					fmt.Fprintf(os.Stderr, warning, repo, repo)

					go func() {
						manager := prsync.NewManager(nil)
						_ = manager.Start(repo)
					}()
				}
			}

			response, err := service.Analyze(cmd.Context(), repo)
			if err != nil {
				return err
			}

			switch strings.ToLower(format) {
			case "json", "":
				return writeJSON(cmd, response)
			default:
				return fmt.Errorf("invalid format %q for analyze", format)
			}
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	command.Flags().StringVar(&format, "format", "json", "Output format: json")
	command.Flags().BoolVar(&useCacheFirst, "use-cache-first", true, "Check cache before live fetch")
	command.Flags().BoolVar(&forceLive, "force-live", false, "Skip cache check and force live fetch")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}

func RegisterClusterCommand() {
	var repo string
	var format string

	command := &cobra.Command{
		Use:   "cluster",
		Short: "Cluster pull requests for a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			service := app.NewService(app.Config{})
			response, err := service.Cluster(cmd.Context(), repo)
			if err != nil {
				return err
			}

			switch strings.ToLower(format) {
			case "json", "":
				return writeJSON(cmd, response)
			default:
				return fmt.Errorf("invalid format %q for cluster", format)
			}
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	command.Flags().StringVar(&format, "format", "json", "Output format: json")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}

func RegisterGraphCommand() {
	var repo string
	var format string

	command := &cobra.Command{
		Use:   "graph",
		Short: "Render a dependency graph for a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			service := app.NewService(app.Config{})
			response, err := service.Graph(cmd.Context(), repo)
			if err != nil {
				return err
			}

			switch strings.ToLower(format) {
			case "dot", "":
				_, err := fmt.Fprintln(cmd.OutOrStdout(), response.DOT)
				return err
			case "json":
				return writeJSON(cmd, response)
			default:
				return fmt.Errorf("invalid format %q for graph", format)
			}
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	command.Flags().StringVar(&format, "format", "dot", "Output format: dot|json")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}

func RegisterPlanCommand() {
	var repo string
	var target int
	var mode string
	var format string
	var dryRun bool
	var includeBots bool

	command := &cobra.Command{
		Use:   "plan",
		Short: "Generate a merge plan for a repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			selectedMode, err := parseMode(mode)
			if err != nil {
				return err
			}

			if !cmd.Flags().Changed("dry-run") {
				dryRun = true
			}

			service := app.NewService(app.Config{})
			response, err := service.Plan(cmd.Context(), repo, target, selectedMode)
			if err != nil {
				return err
			}

			details := fmt.Sprintf("target=%d mode=%s dry_run=%t include_bots=%t", target, selectedMode, dryRun, includeBots)
			logAuditEntry("plan", repo, details)

			switch strings.ToLower(format) {
			case "json", "":
				return writeJSON(cmd, response)
			default:
				return fmt.Errorf("invalid format %q for plan", format)
			}
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	command.Flags().IntVar(&target, "target", 20, "Number of PRs to include in merge plan")
	command.Flags().StringVar(&mode, "mode", string(formula.ModeCombination), "Formula mode: combination|permutation|with_replacement")
	command.Flags().StringVar(&format, "format", "json", "Output format: json")
	command.Flags().BoolVar(&dryRun, "dry-run", true, "Plan only; do not execute (always true by default)")
	command.Flags().BoolVar(&includeBots, "include-bots", false, "Include bot PRs in merge plan (default excludes bots)")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}

func RegisterServeCommand() {
	var port int
	var repo string

	command := &cobra.Command{
		Use:   "serve",
		Short: "Serve the prATC API",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(cmd.Context(), port, repo)
		},
	}
	command.Flags().IntVar(&port, "port", 8080, "Port to bind the API server to")
	command.Flags().StringVar(&repo, "repo", "", "Optional default repository for API routes")
	rootCmd.AddCommand(command)
}

func RegisterSyncCommand() {
	var repo string
	var watch bool
	var interval time.Duration

	command := &cobra.Command{
		Use:   "sync",
		Short: "Sync repository metadata and refs",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := prsync.NewManager(nil)
			if err := manager.Start(repo); err != nil {
				return err
			}

			if !watch {
				return writeJSON(cmd, map[string]any{"started": true, "repo": repo})
			}

			if interval <= 0 {
				return fmt.Errorf("invalid value for --interval: must be greater than 0")
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "watching sync for %s every %s\n", repo, interval)
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-cmd.Context().Done():
					return nil
				case <-ticker.C:
					if err := manager.Start(repo); err != nil {
						return err
					}
				}
			}
		},
	}

	command.Flags().StringVar(&repo, "repo", "", "Repository in owner/repo format")
	command.Flags().BoolVar(&watch, "watch", false, "Run sync in watch mode")
	command.Flags().DurationVar(&interval, "interval", 5*time.Minute, "Watch mode interval")
	_ = command.MarkFlagRequired("repo")
	rootCmd.AddCommand(command)
}

func writeJSON(cmd *cobra.Command, payload any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func parseMode(raw string) (formula.Mode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(formula.ModeCombination):
		return formula.ModeCombination, nil
	case string(formula.ModePermutation):
		return formula.ModePermutation, nil
	case string(formula.ModeWithReplacement):
		return formula.ModeWithReplacement, nil
	default:
		return "", fmt.Errorf("invalid mode %q", raw)
	}
}

func runServer(ctx context.Context, port int, defaultRepo string) error {
	service := app.NewService(app.Config{})
	settingsStore, err := openSettingsStore()
	if err != nil {
		return err
	}
	defer settingsStore.Close()
	repoSync := prsync.NewManager(nil)

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

func getSyncStatus(repo string) map[string]any {
	dbPath := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = filepath.Join(home, ".pratc", "pratc.db")
	}
	store, err := cache.Open(dbPath)
	if err != nil {
		return map[string]any{"error": "cache unavailable", "repo": repo}
	}
	defer store.Close()

	lastSync, err := store.LastSync(repo)
	if err != nil {
		return map[string]any{"repo": repo, "last_sync": nil, "error": err.Error()}
	}
	prs, _ := store.ListPRs(cache.PRFilter{Repo: repo})
	return map[string]any{
		"repo":      repo,
		"last_sync": lastSync,
		"pr_count":  len(prs),
	}
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
	default:
		writeHTTPError(w, http.StatusNotFound, "route not found")
	}
}

func openSettingsStore() (*settings.Store, error) {
	path := strings.TrimSpace(os.Getenv("PRATC_SETTINGS_DB"))
	if path == "" {
		path = "pratc-settings.db"
	}
	store, err := settings.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open settings store: %w", err)
	}
	return store, nil
}

func openAuditStore() (*cache.AuditStore, error) {
	path := strings.TrimSpace(os.Getenv("PRATC_DB_PATH"))
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".pratc", "pratc.db")
	}
	store, err := cache.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return cache.NewAuditStore(store), nil
}

func logAuditEntry(action, repo, details string) {
	auditStore, err := openAuditStore()
	if err != nil {
		return
	}
	defer auditStore.Close()
	entry := audit.AuditEntry{
		Timestamp: time.Now().UTC(),
		Action:    action,
		Repo:      repo,
		Details:   details,
	}
	_ = auditStore.Append(entry)
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

func handleAnalyze(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}

	response, err := service.Analyze(r.Context(), repo)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeHTTPJSON(w, http.StatusOK, response)
}

func handleCluster(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}

	response, err := service.Cluster(r.Context(), repo)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeHTTPJSON(w, http.StatusOK, response)
}

func handleGraph(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}

	response, err := service.Graph(r.Context(), repo)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("format")), "dot") {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, response.DOT)
		return
	}

	writeHTTPJSON(w, http.StatusOK, response)
}

func handlePlan(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
	if !ensureGET(w, r) || !ensureRepo(w, repo) {
		return
	}

	// Parse and validate query parameters
	query := r.URL.Query()
	var validationErrors []string

	// target: int, default 20, must be > 0
	target := 20
	if raw := strings.TrimSpace(query.Get("target")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			validationErrors = append(validationErrors, "target must be a positive integer")
		} else {
			target = parsed
		}
	}

	// mode: string, default "combination"
	mode, err := parseMode(query.Get("mode"))
	if err != nil {
		validationErrors = append(validationErrors, err.Error())
	}

	// cluster_id: string, optional, no validation needed beyond presence
	clusterID := strings.TrimSpace(query.Get("cluster_id"))
	_ = clusterID // reserved for future use

	// exclude_conflicts: bool, default false
	excludeConflicts := false
	if raw := strings.TrimSpace(query.Get("exclude_conflicts")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			validationErrors = append(validationErrors, "exclude_conflicts must be a boolean (true/false)")
		} else {
			excludeConflicts = parsed
		}
	}
	_ = excludeConflicts // reserved for future use

	// stale_score_threshold: float 0..1, default 0.0
	staleScoreThreshold := 0.0
	if raw := strings.TrimSpace(query.Get("stale_score_threshold")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			validationErrors = append(validationErrors, "stale_score_threshold must be a number")
		} else if parsed < 0 || parsed > 1 {
			validationErrors = append(validationErrors, "stale_score_threshold must be between 0 and 1")
		} else {
			staleScoreThreshold = parsed
		}
	}
	_ = staleScoreThreshold // reserved for future use

	// candidate_pool_cap: int 1..500, default 100
	candidatePoolCap := 100
	if raw := strings.TrimSpace(query.Get("candidate_pool_cap")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			validationErrors = append(validationErrors, "candidate_pool_cap must be an integer")
		} else if parsed < 1 || parsed > 500 {
			validationErrors = append(validationErrors, "candidate_pool_cap must be between 1 and 500")
		} else {
			candidatePoolCap = parsed
		}
	}
	_ = candidatePoolCap // reserved for future use

	// score_min: float 0..100, default 0.0
	scoreMin := 0.0
	if raw := strings.TrimSpace(query.Get("score_min")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			validationErrors = append(validationErrors, "score_min must be a number")
		} else if parsed < 0 || parsed > 100 {
			validationErrors = append(validationErrors, "score_min must be between 0 and 100")
		} else {
			scoreMin = parsed
		}
	}
	_ = scoreMin // reserved for future use

	// Return validation errors if any
	if len(validationErrors) > 0 {
		writeHTTPError(w, http.StatusBadRequest, strings.Join(validationErrors, "; "))
		return
	}

	response, err := service.Plan(r.Context(), repo, target, mode)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeHTTPJSON(w, http.StatusOK, response)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseRepoActionPath(path string) (repo string, action string, ok bool) {
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 5 {
		return "", "", false
	}
	if parts[0] != "api" || parts[1] != "repos" {
		return "", "", false
	}

	return parts[2] + "/" + parts[3], strings.Join(parts[4:], "/"), true
}

func repoFromQuery(r *http.Request, fallback string) string {
	repo := strings.TrimSpace(r.URL.Query().Get("repo"))
	if repo != "" {
		return repo
	}
	return strings.TrimSpace(fallback)
}

func ensureGET(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
	return false
}

func ensureRepo(w http.ResponseWriter, repo string) bool {
	if strings.TrimSpace(repo) != "" {
		return true
	}
	writeHTTPError(w, http.StatusBadRequest, "missing repo parameter")
	return false
}

func writeHTTPJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeHTTPError(w http.ResponseWriter, status int, message string) {
	writeHTTPJSON(w, status, map[string]string{"error": message})
}

func RegisterMirrorCommand() {
	baseDir, err := repo.DefaultBaseDir()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "failed to resolve mirror base directory:", err)
		return
	}

	mirrorCmd := &cobra.Command{
		Use:   "mirror",
		Short: "Manage git mirrors",
		Long:  "List, inspect, and clean up git mirrors used for PR analysis",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all synced repos",
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := os.ReadDir(baseDir)
			if err != nil {
				if os.IsNotExist(err) {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No mirrors found (directory does not exist)")
					return nil
				}
				return fmt.Errorf("read mirror directory: %w", err)
			}
			if len(entries) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No mirrors found")
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-40s %s\n", "REPO", "PATH")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 80))
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				owner := entry.Name()
				ownerPath := filepath.Join(baseDir, owner)
				subEntries, err := os.ReadDir(ownerPath)
				if err != nil {
					continue
				}
				for _, sub := range subEntries {
					if !sub.IsDir() {
						continue
					}
					repoName := sub.Name()
					fullRepo := owner + "/" + repoName
					repoPath := filepath.Join(ownerPath, repoName)
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%-40s %s\n", fullRepo, repoPath)
				}
			}
			return nil
		},
	}

	infoCmd := &cobra.Command{
		Use:   "info [owner/repo]",
		Short: "Show detailed stats for a mirror",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoIdentifier := args[0]
			repoPath, err := repo.MirrorPath(baseDir, repoIdentifier)
			if err != nil {
				return fmt.Errorf("invalid repo format: %w", err)
			}
			info, err := os.Stat(repoPath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("mirror not found for %s", repoIdentifier)
				}
				return fmt.Errorf("stat mirror path: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Repository: %s\n", repoIdentifier)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Path: %s\n", repoPath)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Exists: %v\n", info.IsDir())
			if info.IsDir() {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Last modified: %s\n", info.ModTime().Format(time.RFC3339))
			}
			return nil
		},
	}

	pruneCmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove mirrors for repos no longer tracked (dry-run)",
		RunE: func(cmd *cobra.Command, args []string) error {
			confirm, _ := cmd.Flags().GetBool("yes")
			if !confirm {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Prune would remove the following mirrors (use --yes to confirm):")
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "(prune not yet implemented - this is a dry-run)")
			return nil
		},
	}
	pruneCmd.Flags().Bool("yes", false, "Skip confirmation prompt")

	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove ALL mirrors (nuclear option)",
		RunE: func(cmd *cobra.Command, args []string) error {
			confirm, _ := cmd.Flags().GetBool("yes")
			if !confirm {
				return fmt.Errorf("refusing to clean all mirrors without --yes flag")
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Clean not yet implemented - would remove all mirrors in %s\n", baseDir)
			return nil
		},
	}
	cleanCmd.Flags().Bool("yes", false, "Skip confirmation prompt")

	mirrorCmd.AddCommand(listCmd, infoCmd, pruneCmd, cleanCmd)
	rootCmd.AddCommand(mirrorCmd)
}
