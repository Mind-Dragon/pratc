package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jeffersonnunn/pratc/internal/monitor/data"
)

// allowedOrigins holds the list of permitted origins for WebSocket connections.
// It is configured at server startup via NewWebSocketServer.
var allowedOrigins []string

// SetAllowedOrigins configures the global allowed origins list for WebSocket connections.
func SetAllowedOrigins(origins []string) {
	allowedOrigins = origins
}

// isOriginAllowed checks if the given origin is in the allowed list.
func isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	for _, a := range allowedOrigins {
		if a == origin {
			return true
		}
	}
	return false
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
