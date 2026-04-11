// Package service provides the business logic for Warden operations.
//
// Service is the single orchestration layer — all lifecycle management
// (session watchers, event directory watchers), business logic, and
// state lives here. HTTP handlers, TUI adapters, and Go library
// consumers all call the same Service methods.
package service

import (
	"errors"
	"os"
	"sync"
	"time"

	"github.com/thesimonho/warden/access"
	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/docker"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/event"
	"github.com/thesimonho/warden/eventbus"
	"github.com/thesimonho/warden/watcher/hook"
)

// ErrDockerUnavailable is returned by container-mutating operations
// when Docker was not reachable at startup.
var ErrDockerUnavailable = errors.New( //nolint:staticcheck // user-facing message, capitalization intentional
	"Docker is required but not available. " +
		"Install Docker (https://docs.docker.com/get-docker/) " +
		"and make sure the daemon is running",
)

// ServiceDeps holds all dependencies for constructing a Service.
// Using a struct because the constructor has many parameters.
type ServiceDeps struct {
	Engine          engine.Client
	DB              *db.Store
	Store           *eventbus.Store
	Audit           *db.AuditWriter
	Registry        *agent.Registry
	EventWatcher    *hook.Watcher
	EventHandler    func(event.ContainerEvent)
	HomeDir         string
	DockerAvailable bool

	// EnvResolver provides environment variable lookup for access item
	// detection and resolution. When nil, a default ProcessEnvResolver
	// is used (direct os.LookupEnv delegation).
	EnvResolver access.EnvResolver

	// DockerInfo caches the Docker runtime info detected at startup.
	// Used by ListRuntimes to avoid re-detecting on every API call.
	DockerInfo docker.Info

	// BridgeIP is the host IP reachable from containers via
	// host.docker.internal. Used as the listen address for socket
	// bridge TCP proxies (SSH/GPG agent forwarding).
	BridgeIP string

	// Broker is the SSE event broker for real-time updates.
	// Used by the focus tracker to broadcast viewer focus changes.
	Broker *eventbus.Broker
}

// Service provides business logic for all Warden operations. It is
// the single orchestration layer between external consumers (HTTP
// handlers, TUI, Go library callers) and the lower-level engine,
// database, and event subsystems.
//
// Service manages all container lifecycle including session watcher
// start/stop and event directory registration. Callers never need
// to manage these directly.
type Service struct {
	docker engine.Client
	db     *db.Store
	store  *eventbus.Store
	audit  *db.AuditWriter

	// Lifecycle deps for session watchers and event directory management.
	agentRegistry *agent.Registry
	eventWatcher  *hook.Watcher
	eventHandler  func(event.ContainerEvent)
	homeDir       string
	workingDir    string

	// envResolver provides combined process + shell environment lookup
	// for access item detection and resolution.
	envResolver access.EnvResolver

	// dockerAvailable indicates whether Docker was reachable at startup.
	// When false, container-mutating operations return ErrDockerUnavailable.
	dockerAvailable bool

	// dockerInfo caches the Docker runtime info from startup.
	dockerInfo docker.Info

	// bridgeIP is the host IP reachable from containers. Bridge TCP
	// listeners bind to this address so containers can connect via
	// host.docker.internal.
	bridgeIP string

	// socketBridges tracks active TCP→Unix socket bridges keyed by
	// container name. Each bridge proxies TCP connections to a host
	// Unix socket so containers can reach host agents (SSH, GPG)
	// via host.docker.internal.
	socketBridges   map[string][]*socketBridge
	socketBridgesMu sync.Mutex

	// Session watcher state — one watcher per project+agent, keyed by compound key.
	sessionWatchers         map[db.ProjectAgentKey]*agent.SessionWatcher
	sessionWatchersMu       sync.Mutex
	sessionWatcherCooldowns map[db.ProjectAgentKey]time.Time

	// costFallbackNegCache tracks projects where the docker exec cost fallback
	// returned no data. Entries are TTL'd at 60s to avoid repeated exec calls
	// for freshly started containers that haven't written cost data yet.
	// Cleared when a JSONL cost event arrives for the project.
	costFallbackNegCache   map[db.ProjectAgentKey]time.Time
	costFallbackNegCacheMu sync.RWMutex

	// recentlyCreated tracks container IDs that were just created by
	// CreateContainer. HandleContainerStart skips these to avoid
	// double-applying network isolation (CreateContainer already
	// waits for installs and applies it).
	recentlyCreated sync.Map

	// focus tracks which clients are viewing which projects/worktrees.
	focus *focusTracker
}

// New creates a Service with the given dependencies. The lifecycle
// deps (Registry, EventWatcher, EventHandler, HomeDir) may be nil —
// session watcher operations degrade gracefully when absent.
func New(deps ServiceDeps) *Service {
	wd, _ := os.Getwd()

	envResolver := deps.EnvResolver
	if envResolver == nil {
		envResolver = access.ProcessEnvResolver{}
	}

	return &Service{
		docker:                  deps.Engine,
		db:                      deps.DB,
		store:                   deps.Store,
		audit:                   deps.Audit,
		agentRegistry:           deps.Registry,
		eventWatcher:            deps.EventWatcher,
		eventHandler:            deps.EventHandler,
		homeDir:                 deps.HomeDir,
		workingDir:              wd,
		envResolver:             envResolver,
		dockerAvailable:         deps.DockerAvailable,
		dockerInfo:              deps.DockerInfo,
		bridgeIP:                deps.BridgeIP,
		socketBridges:           make(map[string][]*socketBridge),
		sessionWatchers:         make(map[db.ProjectAgentKey]*agent.SessionWatcher),
		sessionWatcherCooldowns: make(map[db.ProjectAgentKey]time.Time),
		costFallbackNegCache:    make(map[db.ProjectAgentKey]time.Time),
		focus:                   newFocusTracker(deps.Broker),
	}
}

// requireDocker returns ErrDockerUnavailable when Docker was not
// reachable at startup. Call at the top of container-mutating methods.
func (s *Service) requireDocker() error {
	if !s.dockerAvailable {
		return ErrDockerUnavailable
	}
	return nil
}
