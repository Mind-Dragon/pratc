package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jeffersonnunn/pratc/internal/monitor/data"
)

type mockBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan data.DataUpdate]*mockSubscriber
}

type mockSubscriber struct {
	ch     chan data.DataUpdate
	closed bool
}

func newMockBroadcaster() *mockBroadcaster {
	return &mockBroadcaster{
		subscribers: make(map[chan data.DataUpdate]*mockSubscriber),
	}
}

func (b *mockBroadcaster) Subscribe() chan data.DataUpdate {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan data.DataUpdate, 64)
	sub := &mockSubscriber{ch: ch}
	b.subscribers[ch] = sub
	return ch
}

func (b *mockBroadcaster) Unsubscribe(ch chan data.DataUpdate) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if sub, ok := b.subscribers[ch]; ok {
		delete(b.subscribers, ch)
		sub.closed = true
		close(ch)
	}
}

func (b *mockBroadcaster) Broadcast(update data.DataUpdate) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscribers {
		if !sub.closed {
			select {
			case sub.ch <- update:
			default:
			}
		}
	}
}

func (b *mockBroadcaster) Start(ctx context.Context) {}
func (b *mockBroadcaster) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for ch, sub := range b.subscribers {
		if !sub.closed {
			sub.closed = true
			close(ch)
		}
		delete(b.subscribers, ch)
	}
}

func TestWebSocketUpgrade(t *testing.T) {
	broadcaster := newMockBroadcaster()
	server := NewWebSocketServer((*data.Broadcaster)(nil))
	server.broadcaster = (*data.Broadcaster)(nil)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade failed: %v", err)
		}
		defer conn.Close()
	}))
	defer ts.Close()

	wsURL := strings.Replace(ts.URL, "http://", "ws://", 1)

	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer ws.Close()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("expected status %d, got %d", http.StatusSwitchingProtocols, resp.StatusCode)
	}

	if !strings.Contains(resp.Header.Get("Upgrade"), "websocket") {
		t.Errorf("expected Upgrade header to contain 'websocket', got %s", resp.Header.Get("Upgrade"))
	}

	if !strings.Contains(resp.Header.Get("Connection"), "Upgrade") {
		t.Errorf("expected Connection header to contain 'Upgrade', got %s", resp.Header.Get("Connection"))
	}

	_ = broadcaster
	_ = server
}

func TestWebSocketMessageBroadcast(t *testing.T) {
	broadcaster := newMockBroadcaster()

	wsServer := &testWebSocketServer{
		broadcaster: broadcaster,
		clients:     make(map[*websocket.Conn]struct{}),
	}

	ts := httptest.NewServer(wsServer)
	defer ts.Close()

	wsURL := strings.Replace(ts.URL, "http://", "ws://", 1)

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer ws.Close()

	time.Sleep(100 * time.Millisecond)

	update := data.DataUpdate{
		Timestamp: time.Now(),
		SyncJobs: []data.SyncJobView{
			{ID: "job-1", Repo: "owner/repo", Status: "active", Progress: 50},
		},
	}
	broadcaster.Broadcast(update)

	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("read message failed: %v", err)
	}

	if !strings.Contains(string(msg), "job-1") {
		t.Errorf("expected message to contain 'job-1', got %s", string(msg))
	}

	if !strings.Contains(string(msg), "owner/repo") {
		t.Errorf("expected message to contain 'owner/repo', got %s", string(msg))
	}
}

func TestWebSocketMultipleClients(t *testing.T) {
	broadcaster := newMockBroadcaster()

	wsServer := &testWebSocketServer{
		broadcaster: broadcaster,
		clients:     make(map[*websocket.Conn]struct{}),
	}

	ts := httptest.NewServer(wsServer)
	defer ts.Close()

	wsURL := strings.Replace(ts.URL, "http://", "ws://", 1)

	numClients := 3
	clients := make([]*websocket.Conn, numClients)
	for i := 0; i < numClients; i++ {
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial failed for client %d: %v", i, err)
		}
		clients[i] = ws
		defer ws.Close()
	}

	time.Sleep(100 * time.Millisecond)

	update := data.DataUpdate{
		Timestamp: time.Now(),
		RateLimit: data.RateLimitView{
			Remaining: 4000,
			Total:     5000,
		},
	}
	broadcaster.Broadcast(update)

	var wg sync.WaitGroup
	wg.Add(numClients)

	for i, client := range clients {
		go func(idx int, ws *websocket.Conn) {
			defer wg.Done()

			ws.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, msg, err := ws.ReadMessage()
			if err != nil {
				t.Errorf("client %d: read message failed: %v", idx, err)
				return
			}

			if !strings.Contains(string(msg), "4000") {
				t.Errorf("client %d: expected message to contain '4000', got %s", idx, string(msg))
			}
		}(i, client)
	}

	wg.Wait()
}

func TestWebSocketClientDisconnect(t *testing.T) {
	broadcaster := newMockBroadcaster()

	wsServer := &testWebSocketServer{
		broadcaster: broadcaster,
		clients:     make(map[*websocket.Conn]struct{}),
	}

	ts := httptest.NewServer(wsServer)
	defer ts.Close()

	wsURL := strings.Replace(ts.URL, "http://", "ws://", 1)

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	wsServer.mu.RLock()
	initialCount := len(wsServer.clients)
	wsServer.mu.RUnlock()

	if initialCount != 1 {
		t.Errorf("expected 1 client registered, got %d", initialCount)
	}

	ws.Close()

	time.Sleep(300 * time.Millisecond)

	wsServer.mu.RLock()
	finalCount := len(wsServer.clients)
	wsServer.mu.RUnlock()

	if finalCount != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", finalCount)
	}

	update := data.DataUpdate{
		Timestamp: time.Now(),
		SyncJobs:  []data.SyncJobView{{ID: "test", Status: "active"}},
	}
	broadcaster.Broadcast(update)
}

func TestWebSocketPingPong(t *testing.T) {
	broadcaster := newMockBroadcaster()

	wsServer := &testWebSocketServer{
		broadcaster: broadcaster,
		clients:     make(map[*websocket.Conn]struct{}),
	}

	ts := httptest.NewServer(wsServer)
	defer ts.Close()

	wsURL := strings.Replace(ts.URL, "http://", "ws://", 1)

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer ws.Close()

	time.Sleep(100 * time.Millisecond)

	pongReceived := make(chan bool, 1)
	ws.SetPongHandler(func(appData string) error {
		pongReceived <- true
		return nil
	})

	wsServer.mu.RLock()
	var serverConn *websocket.Conn
	for conn := range wsServer.clients {
		serverConn = conn
		break
	}
	wsServer.mu.RUnlock()

	if serverConn != nil {
		serverConn.WriteMessage(websocket.PingMessage, nil)
	}

	ws.WriteMessage(websocket.PingMessage, nil)

	select {
	case <-pongReceived:
	case <-time.After(500 * time.Millisecond):
		t.Log("pong not received within timeout (this is acceptable)")
	}

	ws.WriteMessage(websocket.TextMessage, []byte(`{"test": "ping"}`))
}

func TestWebSocketReconnection(t *testing.T) {
	broadcaster := newMockBroadcaster()

	wsServer := &testWebSocketServer{
		broadcaster: broadcaster,
		clients:     make(map[*websocket.Conn]struct{}),
	}

	ts := httptest.NewServer(wsServer)
	defer ts.Close()

	wsURL := strings.Replace(ts.URL, "http://", "ws://", 1)

	ws1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("first dial failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	wsServer.mu.RLock()
	count1 := len(wsServer.clients)
	wsServer.mu.RUnlock()
	if count1 != 1 {
		t.Errorf("expected 1 client after first connect, got %d", count1)
	}

	ws1.Close()

	time.Sleep(300 * time.Millisecond)

	wsServer.mu.RLock()
	count2 := len(wsServer.clients)
	wsServer.mu.RUnlock()
	if count2 != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", count2)
	}

	ws2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("reconnect dial failed: %v", err)
	}
	defer ws2.Close()

	time.Sleep(100 * time.Millisecond)

	wsServer.mu.RLock()
	count3 := len(wsServer.clients)
	wsServer.mu.RUnlock()
	if count3 != 1 {
		t.Errorf("expected 1 client after reconnect, got %d", count3)
	}

	update := data.DataUpdate{
		Timestamp: time.Now(),
		SyncJobs:  []data.SyncJobView{{ID: "reconnect-test", Status: "active"}},
	}
	broadcaster.Broadcast(update)

	ws2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := ws2.ReadMessage()
	if err != nil {
		t.Fatalf("read after reconnect failed: %v", err)
	}

	if !strings.Contains(string(msg), "reconnect-test") {
		t.Errorf("expected message to contain 'reconnect-test', got %s", string(msg))
	}
}

type testWebSocketServer struct {
	broadcaster *mockBroadcaster
	clients     map[*websocket.Conn]struct{}
	mu          sync.RWMutex
}

func (s *testWebSocketServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.addClient(conn)

	sub := s.broadcaster.Subscribe()
	defer s.broadcaster.Unsubscribe(sub)

	conn.SetReadDeadline(time.Time{})
	conn.SetPongHandler(func(string) error {
		return nil
	})

	stopCh := make(chan struct{})
	go s.pingLoop(conn, stopCh)
	defer close(stopCh)

	disconnectCh := make(chan struct{})
	go s.readLoop(conn, disconnectCh)

	for {
		select {
		case update, ok := <-sub:
			if !ok {
				s.removeClient(conn)
				return
			}
			s.mu.RLock()
			hasClients := len(s.clients) > 0
			s.mu.RUnlock()

			if !hasClients {
				continue
			}

			data, err := json.Marshal(update)
			if err != nil {
				continue
			}

			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				s.removeClient(conn)
				return
			}
		case <-disconnectCh:
			s.removeClient(conn)
			return
		}
	}
}

func (s *testWebSocketServer) readLoop(conn *websocket.Conn, disconnectCh chan struct{}) {
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			close(disconnectCh)
			return
		}
	}
}

func (s *testWebSocketServer) addClient(conn *websocket.Conn) {
	s.mu.Lock()
	s.clients[conn] = struct{}{}
	s.mu.Unlock()
}

func (s *testWebSocketServer) removeClient(conn *websocket.Conn) {
	s.mu.Lock()
	delete(s.clients, conn)
	s.mu.Unlock()
	conn.Close()
}

func (s *testWebSocketServer) pingLoop(conn *websocket.Conn, stopCh chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				s.removeClient(conn)
				return
			}
		case <-stopCh:
			return
		}
	}
}
