// Package warden provides the entry point for the Warden container engine.
//
//	w, err := warden.New(warden.Options{})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer w.Close()
//
//	projects, _ := w.Service.ListProjects(ctx)
//	result, _ := w.Service.CreateContainer(ctx, api.CreateContainerRequest{...})
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

	"github.com/thesimonho/warden/access"
	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/agent/claudecode"
	"github.com/thesimonho/warden/agent/codex"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/docker"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/engine/seccomp"
	"github.com/thesimonho/warden/eventbus"
	"github.com/thesimonho/warden/service"
	"github.com/thesimonho/warden/watcher/hook"
)

// Options configures the Warden application. All fields are optional
// and have sensible defaults.
type Options struct {
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
	Watcher *hook.Watcher

	// DockerAvailable indicates whether the Docker daemon was reachable
	// at startup. When false, container operations are disabled but the
	// API server and database are still functional.
	DockerAvailable bool

	// DockerDesktop indicates whether the Docker runtime is Docker Desktop.
	// Exposed so frontends can display runtime-specific guidance.
	DockerDesktop bool

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

	agentRegistry := agent.NewRegistry()
	agentRegistry.Register(agent.ClaudeCode, claudecode.NewProvider())
	agentRegistry.Register(agent.Codex, codex.NewProvider())

	// Detect Docker once — determines socket path, availability, and
	// whether the runtime is Docker Desktop (VM-based).
	dockerInfo := docker.Detect(context.Background())

	engineClient, err := engine.NewClient(dockerInfo.SocketPath, agentRegistry)
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	engineClient.SetSeccompProfile(seccomp.ProfileJSON())

	// Verify the engine client can reach the daemon. docker.Detect()
	// used a separate client — this confirms the engine's own connection.
	dockerAvailable := dockerInfo.Available
	if dockerAvailable {
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pingCancel()
		if pingErr := engineClient.Ping(pingCtx); pingErr != nil {
			dockerAvailable = false
			slog.Warn("Docker is not available — container operations are disabled",
				"err", pingErr,
				"install", "https://docs.docker.com/get-docker/",
			)
		}
	} else {
		slog.Warn("Docker is not available — container operations are disabled",
			"install", "https://docs.docker.com/get-docker/",
		)
	}
	if dockerInfo.IsDesktop {
		slog.Info("Docker Desktop detected — socket mounts will use VM proxies")
	}

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
	// user always owns them.
	eventBaseDir := filepath.Join(dbDir, "events")
	broker := eventbus.NewBroker()
	store := eventbus.NewStore(broker, auditWriter)
	watcher := hook.NewWatcher(eventBaseDir, store.HandleEvent, 2*time.Second)

	if err := watcher.Start(context.Background()); err != nil {
		broker.Shutdown()
		_ = database.Close()
		return nil, fmt.Errorf("starting event watcher: %w", err)
	}
	engineClient.SetEventBaseDir(eventBaseDir)

	livenessCtx, livenessCancel := context.WithCancel(context.Background())
	go eventbus.StartLivenessChecker(livenessCtx, store)

	// Shell env resolver: spawns the user's login shell to capture env
	// vars that aren't inherited when launched from a desktop entry
	// (AppImage, .deb, .dmg, Windows installer). The background load
	// warms the cache before the user interacts with access items.
	shellEnv := access.NewShellEnvResolver()
	go func() {
		if err := shellEnv.Load(); err != nil {
			slog.Warn("shell env load failed, access items will use process env only", "err", err)
		}
	}()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("failed to get home dir for session watchers", "err", err)
	}
	svc := service.New(service.ServiceDeps{
		Engine:          engineClient,
		DB:              database,
		Store:           store,
		Audit:           auditWriter,
		Registry:        agentRegistry,
		EventWatcher:    watcher,
		EventHandler:    store.HandleEvent,
		HomeDir:         homeDir,
		DockerAvailable: dockerAvailable,
		DockerInfo:      dockerInfo,
		BridgeIP:        dockerInfo.BridgeIP,
		EnvResolver:     shellEnv,
	})

	// Wire cost persistence and budget enforcement: on every cost update,
	// funnel through the single gateway that persists cost and enforces
	// budget limits. See [service.Service.PersistSessionCost].
	store.SetCostUpdateCallback(svc.PersistSessionCost)
	store.SetStaleCallback(svc.HandleContainerStale)
	store.SetAliveCallback(svc.HandleContainerAlive)

	w := &Warden{
		Service:         svc,
		Broker:          broker,
		DB:              database,
		Engine:          engineClient,
		Watcher:         watcher,
		DockerAvailable: dockerAvailable,
		DockerDesktop:   dockerInfo.IsDesktop,
		livenessCancel:  livenessCancel,
	}

	if dockerAvailable {
		// Start session watchers for already-running containers so JSONL
		// event parsing resumes after a server restart.
		svc.ResumeSessionWatchers(context.Background())

		// Pre-warm the CLI cache in the background so the first container
		// create for each agent type is a cache hit (near-instant).
		// Uses the liveness context so it cancels cleanly on shutdown.
		go func() {
			if err := engineClient.PreWarmCLICache(livenessCtx); err != nil {
				slog.Warn("CLI cache pre-warm failed (first container create will download)", "err", err)
			}
		}()

		// Watch Docker container start events to re-apply network isolation
		// after auto-restarts (restart policy: unless-stopped). The watcher
		// reconnects automatically on errors.
		go engineClient.WatchContainerEvents(livenessCtx, svc.HandleContainerStart)
	}

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
