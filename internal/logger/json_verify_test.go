package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestVerifyJsonOutput(t *testing.T) {
	var buf bytes.Buffer
	l := New("test")
	l.Logger = slog.New(slog.NewJSONHandler(&buf, nil))
	
	l.Info("test message", "key", "value")
	
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	
	if entry["msg"] != "test message" {
		t.Errorf("expected msg 'test message', got '%v'", entry["msg"])
	}
	if entry["level"] != "INFO" {
		t.Errorf("expected level 'INFO', got '%v'", entry["level"])
	}
	if entry["key"] != "value" {
		t.Errorf("expected key 'value', got '%v'", entry["key"])
	}
	
	t.Logf("VERIFIED: JSON output: %s", buf.String())
}

func TestVerifyErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	l := New("test")
	l.Logger = slog.New(slog.NewJSONHandler(&buf, nil))
	
	l.Error("error occurred", "component", "service")
	
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	
	if entry["msg"] != "error occurred" {
		t.Errorf("expected msg 'error occurred', got '%v'", entry["msg"])
	}
	if entry["level"] != "ERROR" {
		t.Errorf("expected level 'ERROR', got '%v'", entry["level"])
	}
	if entry["component"] != "service" {
		t.Errorf("expected component 'service', got '%v'", entry["component"])
	}
	
	t.Logf("VERIFIED: ERROR level output: %s", buf.String())
}
