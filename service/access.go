package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/thesimonho/warden/access"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/db"
)

// ListAccessItems returns all access items (built-in + user-created)
// enriched with host detection status. If a built-in item has been
// customized (saved to DB), the customized version is returned instead.
func (s *Service) ListAccessItems() ([]api.AccessItemResponse, error) {
	userRows, err := s.db.ListAccessItems()
	if err != nil {
		return nil, fmt.Errorf("listing user access items: %w", err)
	}

	// Index DB rows by ID so we can check for built-in overrides.
	dbByID := make(map[string]db.AccessItemRow, len(userRows))
	for _, row := range userRows {
		dbByID[row.ID] = row
	}

	var items []api.AccessItemResponse

	// Built-in items: use DB override if present, otherwise default.
	for _, builtIn := range access.BuiltInItems() {
		item := builtIn
		if override, ok := dbByID[builtIn.ID]; ok {
			converted, convErr := accessItemFromRow(override)
			if convErr == nil {
				converted.BuiltIn = true
				item = converted
			}
			delete(dbByID, builtIn.ID)
		}
		items = append(items, api.AccessItemResponse{
			Item:      item,
			Detection: access.Detect(item, s.envResolver),
		})
	}

	// Remaining DB rows are user-created items.
	for _, row := range userRows {
		if _, isOverride := dbByID[row.ID]; !isOverride {
			continue // Already handled as a built-in override.
		}
		item, convErr := accessItemFromRow(row)
		if convErr != nil {
			continue
		}
		items = append(items, api.AccessItemResponse{
			Item:      item,
			Detection: access.Detect(item, s.envResolver),
		})
	}

	return items, nil
}

// GetAccessItem returns a single access item by ID. For built-in items,
// returns the DB override if one exists, otherwise the default.
func (s *Service) GetAccessItem(id string) (*api.AccessItemResponse, error) {
	// Check DB first — handles both user items and built-in overrides.
	row, err := s.db.GetAccessItem(id)
	if err != nil {
		return nil, fmt.Errorf("getting access item: %w", err)
	}
	if row != nil {
		item, convErr := accessItemFromRow(*row)
		if convErr != nil {
			return nil, convErr
		}
		if access.IsBuiltInID(id) {
			item.BuiltIn = true
		}
		return &api.AccessItemResponse{
			Item:      item,
			Detection: access.Detect(item, s.envResolver),
		}, nil
	}

	// Fall back to built-in default.
	if builtIn := access.BuiltInItemByID(id); builtIn != nil {
		return &api.AccessItemResponse{
			Item:      *builtIn,
			Detection: access.Detect(*builtIn, s.envResolver),
		}, nil
	}

	return nil, ErrNotFound
}

// CreateAccessItem creates a user-defined access item.
func (s *Service) CreateAccessItem(req api.CreateAccessItemRequest) (*access.Item, error) {
	if req.Label == "" {
		return nil, fmt.Errorf("%w: label is required", ErrInvalidInput)
	}
	if len(req.Credentials) == 0 {
		return nil, fmt.Errorf("%w: at least one credential is required", ErrInvalidInput)
	}

	id := generateID()

	credsJSON, err := json.Marshal(req.Credentials)
	if err != nil {
		return nil, fmt.Errorf("marshaling credentials: %w", err)
	}

	row := db.AccessItemRow{
		ID:          id,
		Label:       req.Label,
		Description: req.Description,
		Method:      string(access.MethodTransport),
		Credentials: credsJSON,
	}

	if err := s.db.InsertAccessItem(row); err != nil {
		return nil, err
	}

	item, err := accessItemFromRow(row)
	if err != nil {
		return nil, err
	}

	s.audit.Write(db.Entry{
		Source:  db.SourceBackend,
		Level:   db.LevelInfo,
		Event:   "access_item_created",
		Message: fmt.Sprintf("access item %q created", req.Label),
		Attrs:   map[string]any{"accessItemId": id},
	})

	return &item, nil
}

// UpdateAccessItem updates an access item. For built-in items, this saves
// a customized copy to the DB (overriding the default). For user items,
// this updates the existing DB row.
func (s *Service) UpdateAccessItem(id string, req api.UpdateAccessItemRequest) (*access.Item, error) {
	row, err := s.db.GetAccessItem(id)
	if err != nil {
		return nil, err
	}

	if row == nil {
		// Built-in item with no DB override yet — seed from the default.
		builtIn := access.BuiltInItemByID(id)
		if builtIn == nil {
			return nil, ErrNotFound
		}
		credsJSON, marshalErr := json.Marshal(builtIn.Credentials)
		if marshalErr != nil {
			return nil, fmt.Errorf("marshaling credentials: %w", marshalErr)
		}
		row = &db.AccessItemRow{
			ID:          builtIn.ID,
			Label:       builtIn.Label,
			Description: builtIn.Description,
			Method:      string(builtIn.Method),
			Credentials: credsJSON,
		}
	}

	// Capture pre-update state for audit diff.
	oldLabel := row.Label
	oldDescription := row.Description
	var oldCreds []access.Credential
	if unmarshalErr := json.Unmarshal(row.Credentials, &oldCreds); unmarshalErr != nil {
		slog.Warn("failed to unmarshal old credentials for audit diff", "accessItemId", id, "err", unmarshalErr)
	}

	if req.Label != nil {
		row.Label = *req.Label
	}
	if req.Description != nil {
		row.Description = *req.Description
	}
	if req.Credentials != nil {
		credsJSON, marshalErr := json.Marshal(*req.Credentials)
		if marshalErr != nil {
			return nil, fmt.Errorf("marshaling credentials: %w", marshalErr)
		}
		row.Credentials = credsJSON
	}

	// Upsert: insert if new override, update if existing.
	if existingRow, getErr := s.db.GetAccessItem(id); getErr != nil {
		slog.Warn("failed to check existing access item for upsert", "accessItemId", id, "err", getErr)
	} else if existingRow != nil {
		if err := s.db.UpdateAccessItem(*row); err != nil {
			return nil, err
		}
	} else {
		if err := s.db.InsertAccessItem(*row); err != nil {
			return nil, err
		}
	}

	item, err := accessItemFromRow(*row)
	if err != nil {
		return nil, err
	}
	if access.IsBuiltInID(id) {
		item.BuiltIn = true
	}

	changes := make(map[string]any)
	if req.Label != nil {
		changes["label"] = map[string]string{"old": oldLabel, "new": *req.Label}
	}
	if req.Description != nil {
		changes["description"] = map[string]string{"old": oldDescription, "new": *req.Description}
	}
	if req.Credentials != nil {
		changes["credentials"] = map[string]any{"old": oldCreds, "new": *req.Credentials}
	}
	s.audit.Write(db.Entry{
		Source:  db.SourceBackend,
		Level:   db.LevelInfo,
		Event:   "access_item_updated",
		Message: fmt.Sprintf("access item %q updated", row.Label),
		Attrs:   map[string]any{"accessItemId": id, "builtIn": access.IsBuiltInID(id), "changes": changes},
	})

	return &item, nil
}

// DeleteAccessItem removes a user-defined access item. Built-in items
// cannot be deleted (use ResetAccessItem instead).
func (s *Service) DeleteAccessItem(id string) error {
	if access.IsBuiltInID(id) {
		return fmt.Errorf("%w: cannot delete built-in access item (use reset instead)", ErrInvalidInput)
	}

	// Fetch label before deleting so the audit message is descriptive.
	label := id
	if row, getErr := s.db.GetAccessItem(id); getErr != nil {
		slog.Warn("failed to fetch access item label for audit", "accessItemId", id, "err", getErr)
	} else if row != nil {
		label = row.Label
	}

	if err := s.db.DeleteAccessItem(id); err != nil {
		return err
	}

	s.audit.Write(db.Entry{
		Source:  db.SourceBackend,
		Level:   db.LevelInfo,
		Event:   "access_item_deleted",
		Message: fmt.Sprintf("access item %q deleted", label),
		Attrs:   map[string]any{"accessItemId": id, "label": label},
	})

	return nil
}

// ResetAccessItem restores a built-in access item to its default by
// removing any DB override. Returns ErrInvalidInput for non-built-in items.
func (s *Service) ResetAccessItem(id string) (*access.Item, error) {
	if !access.IsBuiltInID(id) {
		return nil, fmt.Errorf("%w: only built-in access items can be reset", ErrInvalidInput)
	}

	// Remove the DB override if one exists.
	if err := s.db.DeleteAccessItem(id); err != nil {
		return nil, err
	}

	builtIn := access.BuiltInItemByID(id)

	s.audit.Write(db.Entry{
		Source:  db.SourceBackend,
		Level:   db.LevelInfo,
		Event:   "access_item_reset",
		Message: fmt.Sprintf("access item %q reset to default", builtIn.Label),
		Attrs:   map[string]any{"accessItemId": id},
	})

	return builtIn, nil
}

// ResolveAccessItems resolves the given access items and returns their
// injections. Used by the "Test" button in the UI. Accepts items
// directly — no DB lookup is performed.
//
// Refreshes the shell environment cache before resolving so newly
// exported env vars are picked up without restarting Warden.
func (s *Service) ResolveAccessItems(items []access.Item) (*api.ResolveAccessItemsResponse, error) {
	s.refreshEnvResolver()
	return s.resolveAccessItems(items)
}

// resolveAccessItems resolves access items without refreshing the env cache.
// Used internally when the caller has already refreshed.
func (s *Service) resolveAccessItems(items []access.Item) (*api.ResolveAccessItemsResponse, error) {
	resp := &api.ResolveAccessItemsResponse{}
	for _, item := range items {
		resolved, err := access.Resolve(item, s.envResolver)
		if err != nil {
			return nil, fmt.Errorf("resolving access item %q: %w", item.ID, err)
		}
		resp.Items = append(resp.Items, *resolved)
	}

	return resp, nil
}

// ResolveAccessItemsForContainer resolves the given access item IDs and
// merges the resulting env vars and mounts into the container request.
// Looks up items from the DB/built-ins by ID before resolving.
//
// Refreshes the shell environment cache to ensure the container gets
// the latest env vars from the user's shell configuration.
func (s *Service) ResolveAccessItemsForContainer(req *api.CreateContainerRequest) error {
	if len(req.EnabledAccessItems) == 0 {
		return nil
	}

	s.refreshEnvResolver()

	items, err := s.getAccessItemsByIDs(req.EnabledAccessItems)
	if err != nil {
		return err
	}

	resp, err := s.resolveAccessItems(items)
	if err != nil {
		return err
	}

	if req.EnvVars == nil {
		req.EnvVars = make(map[string]string)
	}

	for _, resolved := range resp.Items {
		for _, cred := range resolved.Credentials {
			for _, inj := range cred.Injections {
				switch inj.Type {
				case access.InjectionEnvVar:
					req.EnvVars[inj.Key] = inj.Value
				case access.InjectionMountFile:
					req.Mounts = append(req.Mounts, api.Mount{
						HostPath:      inj.Value,
						ContainerPath: inj.Key,
						ReadOnly:      inj.ReadOnly,
					})
				case access.InjectionMountSocket:
					req.SocketBridges = append(req.SocketBridges, api.Mount{
						HostPath:      inj.Value,
						ContainerPath: inj.Key,
					})
				}
			}
		}
	}

	return nil
}

// startSocketBridgesForSpecs starts TCP bridge proxies for each socket
// bridge spec. Each bridge listens on the Docker bridge gateway IP and
// proxies connections to the host socket. On native Docker, a per-port
// iptables rule is added so containers can reach the listener through
// firewalls with restrictive INPUT policies.
//
// The container-side socat is started separately via docker exec after
// the container is running (see execSocatBridges).
func (s *Service) startSocketBridgesForSpecs(ctx context.Context, specs []api.Mount) []*socketBridge {
	if len(specs) == 0 || s.bridgeIP == "" {
		return nil
	}

	var bridges []*socketBridge
	for _, spec := range specs {
		if bridge := s.startBridgeWithFirewall(ctx, spec.HostPath, spec.ContainerPath); bridge != nil {
			bridges = append(bridges, bridge)
		}
	}
	return bridges
}

// getAccessItemsByIDs returns access items for the given IDs, looking up
// built-ins first and batching DB queries for user items. For built-in
// IDs, returns the DB override if one exists.
func (s *Service) getAccessItemsByIDs(ids []string) ([]access.Item, error) {
	var items []access.Item
	var dbIDs []string

	// Always check DB — it may have overrides for built-in items.
	dbIDs = append(dbIDs, ids...)

	dbRows, err := s.db.GetAccessItemsByIDs(dbIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching access items: %w", err)
	}

	dbByID := make(map[string]db.AccessItemRow, len(dbRows))
	for _, row := range dbRows {
		dbByID[row.ID] = row
	}

	for _, id := range ids {
		if row, ok := dbByID[id]; ok {
			item, convErr := accessItemFromRow(row)
			if convErr != nil {
				return nil, convErr
			}
			if access.IsBuiltInID(id) {
				item.BuiltIn = true
			}
			items = append(items, item)
		} else if builtIn := access.BuiltInItemByID(id); builtIn != nil {
			items = append(items, *builtIn)
		}
		// Silently skip IDs that don't exist anywhere.
	}

	return items, nil
}

// accessItemFromRow converts a database row to an access.Item.
func accessItemFromRow(row db.AccessItemRow) (access.Item, error) {
	var creds []access.Credential
	if err := json.Unmarshal(row.Credentials, &creds); err != nil {
		return access.Item{}, fmt.Errorf("unmarshaling credentials for %q: %w", row.ID, err)
	}

	return access.Item{
		ID:          row.ID,
		Label:       row.Label,
		Description: row.Description,
		Method:      access.Method(row.Method),
		Credentials: creds,
	}, nil
}

// refreshEnvResolver triggers a refresh of the shell environment
// cache if the resolver supports it. This picks up env vars that
// the user may have exported since Warden started, without requiring
// a restart. The refresh has a cooldown (typically 30s) to avoid
// redundant shell spawns on rapid calls.
func (s *Service) refreshEnvResolver() {
	if r, ok := s.envResolver.(access.Refresher); ok {
		if err := r.Refresh(); err != nil {
			slog.Warn("shell env refresh failed, using cached/process env", "err", err)
		}
	}
}

// generateID returns a random 16-character hex string for use as
// a user-created access item ID.
func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
