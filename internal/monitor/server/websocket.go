package server

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jeffersonnunn/pratc/internal/monitor/data"
)

// allowedOrigins holds the list of permitted origins for WebSocket connections.
// It is configured at server startup via NewWebSocketServer.
var allowedOrigins []string

// parsedAllowedOrigins holds the parsed form of allowed origins for efficient validation.
var parsedAllowedOrigins []*url.URL

// SetAllowedOrigins configures the global allowed origins list for WebSocket connections.
// Each origin is validated at startup; malformed origins cause a log warning and are skipped.
func SetAllowedOrigins(origins []string) {
	allowedOrigins = origins
	parsedAllowedOrigins = make([]*url.URL, 0, len(origins))

	for _, origin := range origins {
		parsed, err := url.Parse(origin)
		if err != nil {
			log.Printf("[WARN] WebSocket: skipping malformed allowed origin %q: %v", origin, err)
			continue
		}
		// Validate that the origin has a valid scheme
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			log.Printf("[WARN] WebSocket: skipping allowed origin %q with invalid scheme %q (must be http or https)", origin, parsed.Scheme)
			continue
		}
		// Require host to be present
		if parsed.Host == "" {
			log.Printf("[WARN] WebSocket: skipping allowed origin %q with empty host", origin)
			continue
		}
		parsedAllowedOrigins = append(parsedAllowedOrigins, parsed)
	}

	if len(parsedAllowedOrigins) == 0 && len(origins) > 0 {
		log.Printf("[WARN] WebSocket: no valid allowed origins after filtering; WebSocket connections will be rejected")
	}
}

// isValidOriginURL checks if the given origin string is a valid http/https origin
// and matches one of the configured allowed origins.
func isValidOriginURL(origin string) bool {
	if origin == "" {
		return false
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		log.Printf("[WARN] WebSocket: rejecting origin with parse error: %q: %v", origin, err)
		return false
	}

	// Reject non-http schemes (data:, javascript:, file:, etc.)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		log.Printf("[WARN] WebSocket: rejecting origin with non-http scheme: %q (scheme=%q)", origin, parsed.Scheme)
		return false
	}

	// Require host to be present
	if parsed.Host == "" {
		log.Printf("[WARN] WebSocket: rejecting origin with empty host: %q", origin)
		return false
	}

	// Check if origin matches any allowed origin
	for _, allowed := range parsedAllowedOrigins {
		if matchOrigin(parsed, allowed) {
			return true
		}
	}

	log.Printf("[WARN] WebSocket: rejecting origin not in allowlist: %q", origin)
	return false
}

// matchOrigin checks if the request origin matches the allowed origin,
// considering scheme, host, and port.
func matchOrigin(reqOrigin, allowed *url.URL) bool {
	// Scheme must match exactly (http or https)
	if reqOrigin.Scheme != allowed.Scheme {
		return false
	}

	// Host must match
	if reqOrigin.Host != allowed.Host {
		return false
	}

	return true
}

// isOriginAllowed checks if the given origin is in the allowed list using proper URL parsing.
func isOriginAllowed(origin string) bool {
	return isValidOriginURL(origin)
}

// newUpgrader creates a websocket.Upgrader with proper origin validation.
func newUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return isOriginAllowed(origin)
		},
	}
}

// WebSocketServer broadcasts data updates to connected WebSocket clients.
type WebSocketServer struct {
	broadcaster *data.Broadcaster
	clients     map[*websocket.Conn]struct{}
	mu          sync.RWMutex
}

// NewWebSocketServer creates a new WebSocketServer that broadcasts
// data updates from the given broadcaster to WebSocket clients.
func NewWebSocketServer(broadcaster *data.Broadcaster) *WebSocketServer {
	return &WebSocketServer{
		broadcaster: broadcaster,
		clients:     make(map[*websocket.Conn]struct{}),
	}
}

// ServeHTTP handles WebSocket upgrade requests at /monitor/stream.
func (s *WebSocketServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upgrader := newUpgrader()
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.addClient(conn)
	defer s.removeClient(conn)

	sub := s.broadcaster.Subscribe()
	defer s.broadcaster.Unsubscribe(sub)

	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return nil
	})

	go s.pingLoop(conn)

	for update := range sub {
		data, err := json.Marshal(update)
		if err != nil {
			continue
		}

		s.mu.RLock()
		hasClients := len(s.clients) > 0
		s.mu.RUnlock()

		if !hasClients {
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return
		}
	}
}

func (s *WebSocketServer) addClient(conn *websocket.Conn) {
	s.mu.Lock()
	s.clients[conn] = struct{}{}
	s.mu.Unlock()
}

func (s *WebSocketServer) removeClient(conn *websocket.Conn) {
	s.mu.Lock()
	delete(s.clients, conn)
	s.mu.Unlock()
	conn.Close()
}

func (s *WebSocketServer) pingLoop(conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			return
		}
	}
}
