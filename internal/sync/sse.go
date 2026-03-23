package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Runner interface {
	Run(ctx context.Context, repo string, emit func(eventType string, payload map[string]any)) error
}

type Manager struct {
	runner Runner

	mu      sync.Mutex
	running map[string]bool
	subs    map[string]map[chan sseEvent]struct{}
}

type sseEvent struct {
	eventType string
	data      string
}

func NewManager(runner Runner) *Manager {
	return &Manager{
		runner:  runner,
		running: map[string]bool{},
		subs:    map[string]map[chan sseEvent]struct{}{},
	}
}

func (m *Manager) Start(repo string) error {
	if repo == "" {
		return fmt.Errorf("repo is required")
	}

	m.mu.Lock()
	if m.running[repo] {
		m.mu.Unlock()
		return nil
	}
	m.running[repo] = true
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			m.running[repo] = false
			m.mu.Unlock()
		}()

		runner := m.runner
		if runner == nil {
			runner = noopRunner{}
		}

		err := runner.Run(context.Background(), repo, func(eventType string, payload map[string]any) {
			m.publish(repo, eventType, payload)
		})
		if err != nil {
			m.publish(repo, "error", map[string]any{"message": err.Error()})
			return
		}
		m.publish(repo, "complete", map[string]any{"repo": repo})
	}()

	return nil
}

func (m *Manager) Stream(repo string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("streaming unsupported"))
		return
	}

	ch, unsubscribe := m.subscribe(repo)
	defer unsubscribe()

	_, _ = w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-ch:
			_, _ = fmt.Fprintf(w, "event: %s\n", event.eventType)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", event.data)
			flusher.Flush()
		}
	}
}

func (m *Manager) subscribe(repo string) (<-chan sseEvent, func()) {
	ch := make(chan sseEvent, 16)
	m.mu.Lock()
	if _, ok := m.subs[repo]; !ok {
		m.subs[repo] = map[chan sseEvent]struct{}{}
	}
	m.subs[repo][ch] = struct{}{}
	m.mu.Unlock()

	return ch, func() {
		m.mu.Lock()
		delete(m.subs[repo], ch)
		if len(m.subs[repo]) == 0 {
			delete(m.subs, repo)
		}
		m.mu.Unlock()
		close(ch)
	}
}

func (m *Manager) publish(repo, eventType string, payload map[string]any) {
	body, err := json.Marshal(payload)
	if err != nil {
		body = []byte(`{"message":"failed to encode event"}`)
	}
	event := sseEvent{eventType: eventType, data: string(body)}

	m.mu.Lock()
	defer m.mu.Unlock()
	for ch := range m.subs[repo] {
		select {
		case ch <- event:
		default:
		}
	}
}

type noopRunner struct{}

func (noopRunner) Run(_ context.Context, repo string, emit func(eventType string, payload map[string]any)) error {
	emit("progress", map[string]any{"processed": 1, "total": 1, "repo": repo, "eta_seconds": 0})
	time.Sleep(10 * time.Millisecond)
	return nil
}
