// Package service provides the business logic for Warden operations.
//
// Service is the single orchestration layer — all lifecycle management
// (session watchers, event directory watchers), business logic, and
// state lives here. HTTP handlers, TUI adapters, and Go library
// consumers all call the same Service methods.
package service

import (
	"os"
	"sync"
	"time"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/eventbus"
)

// ServiceDeps holds all dependencies for constructing a Service.
// Using a struct because the constructor has many parameters.
type ServiceDeps struct {
	Engine       engine.Client
	DB           *db.Store
	Store        *eventbus.Store
	Audit        *db.AuditWriter
	Registry     *agent.Registry
	EventWatcher *eventbus.Watcher
	EventHandler func(eventbus.ContainerEvent)
	HomeDir      string
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
	eventWatcher  *eventbus.Watcher
	eventHandler  func(eventbus.ContainerEvent)
	homeDir    string
	workingDir string

	// Session watcher state — one watcher per project+agent, keyed by compound key.
	sessionWatchers         map[db.ProjectAgentKey]*agent.SessionWatcher
	sessionWatchersMu       sync.Mutex
	sessionWatcherCooldowns map[db.ProjectAgentKey]time.Time
}

// New creates a Service with the given dependencies. The lifecycle
// deps (Registry, EventWatcher, EventHandler, HomeDir) may be nil —
// session watcher operations degrade gracefully when absent.
func New(deps ServiceDeps) *Service {
	wd, _ := os.Getwd()
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
		sessionWatchers:         make(map[db.ProjectAgentKey]*agent.SessionWatcher),
		sessionWatcherCooldowns: make(map[db.ProjectAgentKey]time.Time),
	}
}
