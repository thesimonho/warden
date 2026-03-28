// Package service provides the business logic for Warden operations.
//
// It orchestrates the container engine, event bus store, and database
// without any HTTP concerns. Both the HTTP server and direct Go
// library consumers call the same service methods.
package service

import (
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/eventbus"
)

// Service provides business logic for all Warden operations. It is
// the single orchestration layer between external consumers (HTTP
// handlers, CLI, Go library callers) and the lower-level engine,
// database, and event subsystems.
type Service struct {
	docker engine.Client
	db     *db.Store
	store  *eventbus.Store
	audit  *db.AuditWriter
}

// New creates a Service with the given dependencies. The store, db,
// and audit writer may be nil — methods degrade gracefully when absent.
func New(docker engine.Client, database *db.Store, store *eventbus.Store, audit *db.AuditWriter) *Service {
	return &Service{
		docker: docker,
		db:     database,
		store:  store,
		audit:  audit,
	}
}
