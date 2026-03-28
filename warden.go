// Package warden provides a high-level entry point for the Warden
// container orchestration engine. It wires together the database, container
// engine, event bus, and service layer into a ready-to-use App.
//
// For most consumers, [New] is the only function needed:
//
//	app, err := warden.New(warden.Options{})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer app.Close()
//
//	projects, _ := app.Service.ListProjects(ctx)
//
// App also provides higher-level convenience methods that combine
// multiple service operations into common workflows:
//
//	// Create a project with defaults and add to config in one step.
//	result, _ := app.CreateProject(ctx, "my-project", "/home/user/code", nil)
//
//	// Delete a project completely (stop + remove container + remove from config).
//	result, _ := app.DeleteProject(ctx, result.ContainerID)
//
//	// Get a project's full status (container state + worktrees) in one call.
//	status, _ := app.GetProjectStatus(ctx, "my-project")
package warden

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/thesimonho/warden/agent/claudecode"
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

// App is the top-level handle for the Warden engine. It owns the
// lifecycle of all subsystems (event bus, watcher, liveness checker)
// and exposes the service layer for operations.
type App struct {
	// Service provides all Warden operations (projects, containers,
	// worktrees, settings, audit log).
	Service *service.Service

	// Broker is the SSE event broker. Subscribe to receive real-time
	// worktree state, cost updates, and heartbeat events.
	Broker *eventbus.Broker

	// DB is the central SQLite database (projects, settings, events).
	DB *db.Store

	// Engine is the container engine client. Most consumers should
	// use Service instead; Engine is exposed for advanced use cases
	// that need direct Docker/Podman API access.
	Engine *engine.DockerClient

	// Watcher monitors bind-mounted event directories for container
	// events. Exposed so callers can register/unregister container
	// directories and trigger cleanup.
	Watcher *eventbus.Watcher

	livenessCancel context.CancelFunc
	closeOnce      sync.Once
}

// New creates and starts a Warden application. It detects the container
// runtime, wires the event bus pipeline, and returns a ready-to-use App.
// Call [App.Close] when done to release resources.
func New(opts Options) (*App, error) {
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

	socketPath := runtime.SocketForRuntime(context.Background(), runtimeName)
	dockerClient, err := engine.NewClient(socketPath, string(runtimeName), claudecode.NewProvider())
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	dockerClient.SetSeccompProfile(seccompPath, seccomp.ProfileJSON())

	auditModeStr := database.GetSetting("auditLogMode", "")
	auditMode := db.AuditMode(auditModeStr)
	auditWriter := db.NewAuditWriter(database, auditMode, service.StandardAuditEvents())

	// Tee slog output to the audit log so backend warnings/errors
	// appear as debug-category events (detailed mode only).
	stderrHandler := slog.NewTextHandler(os.Stderr, nil)
	compositeHandler := db.NewSlogHandler(stderrHandler, auditWriter)
	slog.SetDefault(slog.New(compositeHandler))

	// Event bus pipeline: broker → store → file watcher.
	// Container events are delivered via bind-mounted directories instead
	// of TCP, so no network listener or auth token is needed.
	eventBaseDir := filepath.Join(os.TempDir(), "warden")
	broker := eventbus.NewBroker()
	store := eventbus.NewStore(broker, auditWriter)
	watcher := eventbus.NewWatcher(eventBaseDir, store.HandleEvent, 2*time.Second)

	if err := watcher.Start(context.Background()); err != nil {
		broker.Shutdown()
		_ = database.Close()
		return nil, fmt.Errorf("starting event watcher: %w", err)
	}
	dockerClient.SetEventBaseDir(eventBaseDir)

	livenessCtx, livenessCancel := context.WithCancel(context.Background())
	go eventbus.StartLivenessChecker(livenessCtx, store)

	svc := service.New(dockerClient, database, store, auditWriter)

	// Wire cost persistence and budget enforcement: on every stop event,
	// funnel through the single gateway that persists cost and enforces
	// budget limits. See [service.Service.PersistSessionCost].
	store.SetStopCallback(svc.PersistSessionCost)
	store.SetStaleCallback(svc.HandleContainerStale)

	return &App{
		Service:        svc,
		Broker:         broker,
		DB:             database,
		Engine:         dockerClient,
		Watcher:        watcher,
		livenessCancel: livenessCancel,
	}, nil
}

// --- Convenience methods ---

// CreateProjectOptions overrides defaults when creating a project container.
// All fields are optional — zero values fall back to sensible defaults
// (the standard Warden image, auto-detected mounts, full network access).
type CreateProjectOptions struct {
	// Image overrides the container image (default: ghcr.io/thesimonho/warden:latest).
	Image string
	// EnvVars sets additional environment variables inside the container.
	EnvVars map[string]string
	// Mounts adds extra bind mounts from host into the container.
	Mounts []engine.Mount
	// SkipPermissions makes terminals skip Claude Code permission prompts.
	SkipPermissions bool
	// NetworkMode controls network isolation (default: "full").
	NetworkMode engine.NetworkMode
	// AllowedDomains lists domains accessible when NetworkMode is "restricted".
	AllowedDomains []string
	// CostBudget is the per-project cost limit in USD (0 = use global default).
	CostBudget float64
}

// CreateProject creates a new project container and adds it to the
// database in one step. Pass nil for opts to use all defaults.
//
// This combines [service.Service.CreateContainer] (which itself creates
// the Docker container and adds it to the database) into a simpler signature
// where only the project name and path are required.
func (a *App) CreateProject(
	ctx context.Context,
	name, projectPath string,
	opts *CreateProjectOptions,
) (*service.ContainerResult, error) {
	req := engine.CreateContainerRequest{
		Name:        name,
		ProjectPath: projectPath,
	}
	if opts != nil {
		req.Image = opts.Image
		req.EnvVars = opts.EnvVars
		req.Mounts = opts.Mounts
		req.SkipPermissions = opts.SkipPermissions
		req.NetworkMode = opts.NetworkMode
		req.AllowedDomains = opts.AllowedDomains
		req.CostBudget = opts.CostBudget
	}
	result, err := a.Service.CreateContainer(ctx, req)
	if err != nil {
		return nil, err
	}
	// Register the container's event directory for fsnotify fast-path detection.
	a.Watcher.WatchContainerDir(name)
	return result, nil
}

// resolveProject looks up a project row by ID. Returns an error if not found.
func (a *App) resolveProject(id string) (*db.ProjectRow, error) {
	row, err := a.Service.GetProject(id)
	if err != nil {
		return nil, fmt.Errorf("looking up project %q: %w", id, err)
	}
	if row == nil {
		return nil, fmt.Errorf("project %q not found", id)
	}
	return row, nil
}

// DeleteProject stops and removes a container, then removes it from
// the project database. This is the inverse of [App.CreateProject].
//
// The returned [service.ContainerResult] contains the ID and name of
// the deleted container. The id parameter is a project ID (sha256 hash).
func (a *App) DeleteProject(ctx context.Context, id string) (*service.ContainerResult, error) {
	row, err := a.resolveProject(id)
	if err != nil {
		return nil, err
	}

	result, err := a.Service.DeleteContainer(ctx, row)
	if err != nil {
		return nil, err
	}
	// Best-effort DB removal — the container is already gone.
	if result.ProjectID != "" {
		if _, removeErr := a.Service.RemoveProject(result.ProjectID); removeErr != nil {
			slog.Warn(
				"container deleted but failed to remove from db",
				"name",
				result.Name,
				"err",
				removeErr,
			)
		}
	}
	return result, nil
}

// StopProject stops a running container. The id parameter is a project ID.
func (a *App) StopProject(ctx context.Context, id string) (*service.ProjectResult, error) {
	row, err := a.resolveProject(id)
	if err != nil {
		return nil, err
	}
	return a.Service.StopProject(ctx, row)
}

// RestartProject restarts a container. The id parameter is a project ID.
func (a *App) RestartProject(ctx context.Context, id string) (*service.ProjectResult, error) {
	row, err := a.resolveProject(id)
	if err != nil {
		return nil, err
	}
	return a.Service.RestartProject(ctx, row)
}

// StopAll stops all running project containers. Returns a result for
// each container that was stopped. Containers that are already stopped
// or not found are silently skipped.
func (a *App) StopAll(ctx context.Context) ([]service.ProjectResult, error) {
	projects, err := a.Service.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	// Pre-load all project rows to avoid N+1 DB lookups.
	allRows, _ := a.DB.ListAllProjects()

	var results []service.ProjectResult
	for _, p := range projects {
		if p.State != "running" || p.ProjectID == "" {
			continue
		}
		row := allRows[p.ProjectID]
		if row == nil {
			slog.Warn("failed to resolve project for stop", "name", p.Name)
			continue
		}
		result, stopErr := a.Service.StopProject(ctx, row)
		if stopErr != nil {
			slog.Warn("failed to stop project", "name", p.Name, "err", stopErr)
			continue
		}
		results = append(results, *result)
	}
	return results, nil
}

// RestartWorktree kills the terminal process for a worktree and
// reconnects it. This is useful when Claude Code gets into a bad state
// and the user wants a fresh terminal without removing the worktree.
// The projectID parameter is a project ID (sha256 hash).
func (a *App) RestartWorktree(
	ctx context.Context,
	projectID, worktreeID string,
) (*service.WorktreeResult, error) {
	row, err := a.resolveProject(projectID)
	if err != nil {
		return nil, err
	}
	if _, err := a.Service.KillWorktreeProcess(ctx, row, worktreeID); err != nil {
		return nil, err
	}
	return a.Service.ConnectTerminal(ctx, row, worktreeID)
}

// ProjectStatus holds a project's container state and its worktrees.
type ProjectStatus struct {
	// Project is the container state, cost, and attention data.
	Project engine.Project `json:"project"`
	// Worktrees lists all worktrees with their terminal state.
	Worktrees []engine.Worktree `json:"worktrees"`
}

// GetProjectStatus returns a single project's full state: container
// info and all worktrees. This combines [service.Service.ListProjects]
// and [service.Service.ListWorktrees] into one call.
func (a *App) GetProjectStatus(ctx context.Context, name string) (*ProjectStatus, error) {
	projects, err := a.Service.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	var project *engine.Project
	for i := range projects {
		if projects[i].Name == name {
			project = &projects[i]
			break
		}
	}
	if project == nil {
		return nil, fmt.Errorf("project %q not found", name)
	}

	row, err := a.Service.GetProject(project.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("looking up project %q: %w", name, err)
	}
	if row == nil {
		return nil, fmt.Errorf("project %q has no DB row", name)
	}

	worktrees, err := a.Service.ListWorktrees(ctx, row)
	if err != nil {
		return nil, err
	}

	return &ProjectStatus{
		Project:   *project,
		Worktrees: worktrees,
	}, nil
}

// Close shuts down all subsystems gracefully. Safe to call multiple times.
func (a *App) Close() {
	a.closeOnce.Do(func() {
		a.livenessCancel()
		a.Broker.Shutdown()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.Watcher.Shutdown(shutdownCtx); err != nil {
			slog.Warn("event watcher shutdown error", "err", err)
		}

		if a.DB != nil {
			if err := a.DB.Close(); err != nil {
				slog.Warn("database close error", "err", err)
			}
		}
	})
}
