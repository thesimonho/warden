// Package warden provides the entry point for the Warden container engine.
//
//	w, err := warden.New(warden.Options{})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer w.Close()
//
//	projects, _ := w.Service.ListProjects(ctx)
//	result, _ := w.Service.CreateContainer(ctx, engine.CreateContainerRequest{...})
//	_, _ = w.Service.StopProject(ctx, projectID)
package warden

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/agent/claudecode"
	"github.com/thesimonho/warden/agent/codex"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/engine/seccomp"
	"github.com/thesimonho/warden/eventbus"
	"github.com/thesimonho/warden/runtime"
	"github.com/thesimonho/warden/service"
)

// Options configures the Warden application. All fields are optional
// and have sensible defaults.
type Options struct {
	// Runtime overrides the configured container runtime. Takes precedence
	// over the WARDEN_RUNTIME env var and the database setting. When empty,
	// WARDEN_RUNTIME is checked, then the database setting (default: "docker").
	Runtime runtime.Runtime

	// DBDir overrides the directory containing the SQLite database.
	// Takes precedence over the WARDEN_DB_DIR env var. When both are
	// empty, the platform-default config directory is used
	// (e.g. ~/.config/warden/).
	DBDir string
}

// Warden holds the initialized engine subsystems and exposes the
// Service for all operations. This is the library entry point.
type Warden struct {
	// Service provides all Warden operations. This is the primary
	// interface for library consumers.
	Service *service.Service

	// Broker is the SSE event broker for real-time updates.
	Broker *eventbus.Broker

	// Engine is the container engine client (advanced use only).
	Engine *engine.EngineClient

	// DB is the SQLite database (advanced use only).
	DB *db.Store

	// Watcher is the file-based event watcher (advanced use only).
	Watcher *eventbus.Watcher

	livenessCancel context.CancelFunc
	closeOnce      sync.Once
}

// New creates and starts a Warden instance. It detects the container
// runtime, wires the event bus pipeline, and returns a ready-to-use
// [Warden]. Call [Warden.Close] when done to release resources.
func New(opts Options) (*Warden, error) {
	// Initialize DB first — settings (runtime, auditLog) are read from it.
	dbDir := opts.DBDir
	if envDB := os.Getenv("WARDEN_DB_DIR"); envDB != "" && dbDir == "" {
		dbDir = envDB
	}
	if dbDir == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("resolving config dir: %w", err)
		}
		dbDir = filepath.Join(configDir, "warden")
	}

	database, err := db.New(dbDir)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Determine runtime: env var > explicit option > DB setting.
	runtimeName := runtime.Runtime(database.GetSetting("runtime", "docker"))
	if envRT := os.Getenv("WARDEN_RUNTIME"); envRT != "" {
		runtimeName = runtime.Runtime(envRT)
	}
	if opts.Runtime != "" {
		runtimeName = opts.Runtime
	}

	seccompPath, err := seccomp.WriteProfileFile(dbDir)
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("writing seccomp profile: %w", err)
	}

	agentRegistry := agent.NewRegistry()
	agentRegistry.Register(agent.ClaudeCode, claudecode.NewProvider())
	agentRegistry.Register(agent.Codex, codex.NewProvider())

	socketPath := runtime.SocketForRuntime(context.Background(), runtimeName)
	engineClient, err := engine.NewClient(socketPath, string(runtimeName), agentRegistry)
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	engineClient.SetSeccompProfile(seccompPath, seccomp.ProfileJSON())

	auditModeStr := database.GetSetting("auditLogMode", "")
	auditMode := db.AuditMode(auditModeStr)
	auditWriter := db.NewAuditWriter(database, auditMode, service.StandardAuditEvents())

	// Tee slog output to the audit log so backend warnings/errors
	// appear as debug-category events (detailed mode only).
	stderrHandler := slog.NewTextHandler(os.Stderr, nil)
	compositeHandler := db.NewSlogHandler(stderrHandler, auditWriter)
	slog.SetDefault(slog.New(compositeHandler))

	// Event bus pipeline: broker -> store -> file watcher.
	// Container events are delivered via bind-mounted directories instead
	// of TCP, so no network listener or auth token is needed.
	// Event dirs live under the config directory (not /tmp) so the current
	// user always owns them — avoids permission failures when switching
	// between Docker and rootless Podman.
	eventBaseDir := filepath.Join(dbDir, "events")
	broker := eventbus.NewBroker()
	store := eventbus.NewStore(broker, auditWriter)
	watcher := eventbus.NewWatcher(eventBaseDir, store.HandleEvent, 2*time.Second)

	if err := watcher.Start(context.Background()); err != nil {
		broker.Shutdown()
		_ = database.Close()
		return nil, fmt.Errorf("starting event watcher: %w", err)
	}
	engineClient.SetEventBaseDir(eventBaseDir)

	livenessCtx, livenessCancel := context.WithCancel(context.Background())
	go eventbus.StartLivenessChecker(livenessCtx, store)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("failed to get home dir for session watchers", "err", err)
	}
	svc := service.New(service.ServiceDeps{
		Engine:       engineClient,
		DB:           database,
		Store:        store,
		Audit:        auditWriter,
		Registry:     agentRegistry,
		EventWatcher: watcher,
		EventHandler: store.HandleEvent,
		HomeDir:      homeDir,
	})

	// Wire cost persistence and budget enforcement: on every cost update,
	// funnel through the single gateway that persists cost and enforces
	// budget limits. See [service.Service.PersistSessionCost].
	store.SetCostUpdateCallback(svc.PersistSessionCost)
	store.SetStaleCallback(svc.HandleContainerStale)
	store.SetAliveCallback(svc.HandleContainerAlive)

	w := &Warden{
		Service:        svc,
		Broker:         broker,
		DB:             database,
		Engine:         engineClient,
		Watcher:        watcher,
		livenessCancel: livenessCancel,
	}

	// Start session watchers for already-running containers so JSONL
	// event parsing resumes after a server restart.
	svc.ResumeSessionWatchers(context.Background())

	return w, nil
}

// Close shuts down all subsystems gracefully. Safe to call multiple times.
func (w *Warden) Close() {
	w.closeOnce.Do(func() {
		w.livenessCancel()
		w.Broker.Shutdown()

		// Stop all session watchers managed by the service layer.
		w.Service.Close()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := w.Watcher.Shutdown(shutdownCtx); err != nil {
			slog.Warn("event watcher shutdown error", "err", err)
		}

		if w.DB != nil {
			if err := w.DB.Close(); err != nil {
				slog.Warn("database close error", "err", err)
			}
		}
	})
}
