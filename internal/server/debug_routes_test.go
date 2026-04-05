package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/service"
)

func TestHandleGetSettings_IncludesAuditLogMode(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: testDB(t)}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	mode, ok := body["auditLogMode"]
	if !ok {
		t.Fatal("expected auditLogMode field in settings response")
	}

	if mode != "off" {
		t.Errorf("expected auditLogMode=off by default, got %v", mode)
	}
}

func TestHandleUpdateSettings_AuditLogMode(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	database := testDB(t)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	body := strings.NewReader(`{"auditLogMode":"detailed"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings", body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if database.GetSetting("auditLogMode", "off") != "detailed" {
		t.Error("expected auditLogMode to be detailed after update")
	}

	var resp map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["restartRequired"] {
		t.Error("audit log mode change should not require restart")
	}
}

func TestHandlePostAuditEvent(t *testing.T) {
	t.Parallel()

	logger, err := db.New(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	writer := db.NewAuditWriter(logger, db.AuditDetailed, nil)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: &mockEngineClient{}, DB: logger, Audit: writer}), nil, nil)

	body := strings.NewReader(`{"event":"terminal_opened","message":"terminal opened"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit", body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	entries, err := logger.Read()
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Source != db.SourceFrontend {
		t.Errorf("expected source %q, got %q", db.SourceFrontend, entries[0].Source)
	}
}

func TestHandleDeleteAuditEvents(t *testing.T) {
	t.Parallel()

	logger, err := db.New(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	_ = logger.Write(db.Entry{Source: db.SourceBackend, Message: "test"})

	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: &mockEngineClient{}, DB: logger}), nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/audit", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	entries, _ := logger.Read()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after delete, got %d", len(entries))
	}
}
