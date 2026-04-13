// Package logger provides structured JSON logging for prATC.
// Log format: JSON lines to stderr with fields: ts, level, component, request_id, repo, job_id, msg
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// contextKey is an unexported key type to prevent context collisions.
type contextKey string

var (
	requestIDKey = contextKey("request_id")
	repoKey      = contextKey("repo")
	jobIDKey     = contextKey("job_id")
)

// Logger is a structured logger pre-configured with component and context support.
type Logger struct {
	*slog.Logger
	component string
}

// New creates a new Logger for the given component.
func New(component string) *Logger {
	return &Logger{
		Logger:    slog.New(newContextHandler(component, os.Stderr)),
		component: component,
	}
}

// FromContext returns a new Logger with request_id, repo, and job_id extracted from context.
func FromContext(ctx context.Context) *Logger {
	l := New("")
	if ctx == nil {
		return l
	}
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		l.Logger = l.Logger.With(slog.String(string(requestIDKey), id))
	}
	if repo, ok := ctx.Value(repoKey).(string); ok {
		l.Logger = l.Logger.With(slog.String(string(repoKey), repo))
	}
	if jobID, ok := ctx.Value(jobIDKey).(string); ok {
		l.Logger = l.Logger.With(slog.String(string(jobIDKey), jobID))
	}
	return l
}

// WithRequestID returns a new Logger with the given request ID.
func (l *Logger) WithRequestID(requestID string) *Logger {
	return &Logger{
		Logger:    l.Logger.With(slog.String(string(requestIDKey), requestID)),
		component: l.component,
	}
}

// WithRepo returns a new Logger with the given repo.
func (l *Logger) WithRepo(repo string) *Logger {
	return &Logger{
		Logger:    l.Logger.With(slog.String(string(repoKey), repo)),
		component: l.component,
	}
}

// WithJobID returns a new Logger with the given job ID.
func (l *Logger) WithJobID(jobID string) *Logger {
	return &Logger{
		Logger:    l.Logger.With(slog.String(string(jobIDKey), jobID)),
		component: l.component,
	}
}

// contextHandler extracts request_id, repo, job_id from context and adds them to log records.
type contextHandler struct {
	component string
	handler   slog.Handler
}

// newContextHandler creates a contextHandler that writes JSON to the given writer.
func newContextHandler(component string, w io.Writer) *contextHandler {
	return &contextHandler{
		component: component,
		handler: slog.NewJSONHandler(w, &slog.HandlerOptions{
			ReplaceAttr: replaceAttr(component),
		}),
	}
}

// replaceAttr returns an attr replacer that prepends component.
func replaceAttr(component string) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == "component" && len(groups) == 0 {
			return slog.String("component", component)
		}
		return a
	}
}

// Handle implements slog.Handler by extracting context values and delegating.
func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		r.Add(string(requestIDKey), id)
	}
	if repo, ok := ctx.Value(repoKey).(string); ok {
		r.Add(string(repoKey), repo)
	}
	if jobID, ok := ctx.Value(jobIDKey).(string); ok {
		r.Add(string(jobIDKey), jobID)
	}
	return h.handler.Handle(ctx, r)
}

// Enabled implements slog.Handler.
func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// WithAttrs implements slog.Handler.
func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{
		component: h.component,
		handler:   h.handler.WithAttrs(attrs),
	}
}

// WithGroup implements slog.Handler.
func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{
		component: h.component,
		handler:   h.handler.WithGroup(name),
	}
}

// Info logs a message at INFO level.
func (l *Logger) Info(msg string, keyValues ...any) {
	l.Logger.Info(msg, keyValues...)
}

// Error logs a message at ERROR level.
func (l *Logger) Error(msg string, keyValues ...any) {
	l.Logger.Error(msg, keyValues...)
}

// Debug logs a message at DEBUG level.
func (l *Logger) Debug(msg string, keyValues ...any) {
	l.Logger.Debug(msg, keyValues...)
}

// Warn logs a message at WARN level.
func (l *Logger) Warn(msg string, keyValues ...any) {
	l.Logger.Warn(msg, keyValues...)
}

// RequestIDFromContext extracts the request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// ContextWithRequestID returns a new context with the given request ID.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RepoFromContext extracts the repo from context.
func RepoFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if repo, ok := ctx.Value(repoKey).(string); ok {
		return repo
	}
	return ""
}

// ContextWithRepo returns a new context with the given repo.
func ContextWithRepo(ctx context.Context, repo string) context.Context {
	return context.WithValue(ctx, repoKey, repo)
}

// JobIDFromContext extracts the job ID from context.
func JobIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if jobID, ok := ctx.Value(jobIDKey).(string); ok {
		return jobID
	}
	return ""
}

// ContextWithJobID returns a new context with the given job ID.
func ContextWithJobID(ctx context.Context, jobID string) context.Context {
	return context.WithValue(ctx, jobIDKey, jobID)
}

// NewForTest creates a Logger that writes JSON to the provided writer.
// This is intended for testing purposes only.
func NewForTest(writer io.Writer, component string) *Logger {
	return &Logger{
		Logger:    slog.New(newContextHandler(component, writer)),
		component: component,
	}
}
