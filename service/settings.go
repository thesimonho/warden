package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/version"
)

// parseFloat parses a string to float64, returning 0 on failure.
func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// formatFloat formats a float64 to a string for database storage.
func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// Settings keys stored in the database.
const (
	settingAuditLogMode         = "auditLogMode"
	settingDisconnectKey        = "disconnectKey"
	settingDefaultProjectBudget = "defaultProjectBudget"

	settingNotificationsEnabled = "notificationsEnabled"

	settingBudgetActionWarn          = "budgetActionWarn"
	settingBudgetActionStopWorktrees = "budgetActionStopWorktrees"
	settingBudgetActionStopContainer = "budgetActionStopContainer"
	settingBudgetActionPreventStart  = "budgetActionPreventStart"
)

// GetSettings returns the current server-side settings.
func (s *Service) GetSettings() SettingsResponse {
	return SettingsResponse{
		Runtime:              "docker",
		AuditLogMode:         api.AuditLogMode(s.db.GetSetting(settingAuditLogMode, string(api.AuditLogOff))),
		DisconnectKey:        s.db.GetSetting(settingDisconnectKey, engine.DefaultDisconnectKey),
		DefaultProjectBudget: parseFloat(s.db.GetSetting(settingDefaultProjectBudget, "0")),

		NotificationsEnabled: s.db.GetSetting(settingNotificationsEnabled, "true") == "true",

		BudgetActionWarn:          s.db.GetSetting(settingBudgetActionWarn, "true") == "true",
		BudgetActionStopWorktrees: s.db.GetSetting(settingBudgetActionStopWorktrees, "false") == "true",
		BudgetActionStopContainer: s.db.GetSetting(settingBudgetActionStopContainer, "false") == "true",
		BudgetActionPreventStart:  s.db.GetSetting(settingBudgetActionPreventStart, "false") == "true",

		WorkingDirectory:  s.workingDir,
		Version:           version.Version,
		ClaudeCodeVersion: agent.ClaudeCodeVersion,
		CodexVersion:      agent.CodexVersion,
	}
}

// GetAuditLogMode returns the current audit log mode.
func (s *Service) GetAuditLogMode() api.AuditLogMode {
	return api.AuditLogMode(s.db.GetSetting(settingAuditLogMode, string(api.AuditLogOff)))
}

// GetDefaultProjectBudget returns the global default per-project budget.
// Returns 0 (unlimited) if not configured.
func (s *Service) GetDefaultProjectBudget() float64 {
	return parseFloat(s.db.GetSetting(settingDefaultProjectBudget, "0"))
}

// UpdateSettings applies setting changes and returns whether a
// restart is required.
func (s *Service) UpdateSettings(ctx context.Context, req UpdateSettingsRequest) (*UpdateSettingsResult, error) {
	restartRequired := false

	if req.AuditLogMode != nil {
		mode := *req.AuditLogMode
		if mode != api.AuditLogOff && mode != api.AuditLogStandard && mode != api.AuditLogDetailed {
			return nil, fmt.Errorf("invalid auditLogMode: %q (must be \"off\", \"standard\", or \"detailed\")", mode)
		}
		if err := s.db.SetSetting(settingAuditLogMode, string(mode)); err != nil {
			return nil, err
		}
		if s.audit != nil {
			s.audit.SetMode(db.AuditMode(mode))
		}
	}

	if req.DisconnectKey != nil {
		key := *req.DisconnectKey
		if engine.DisconnectKeyToByte(key) == 0 {
			return nil, fmt.Errorf("invalid detach key: %q (must be ctrl+<char>)", key)
		}
		if err := s.db.SetSetting(settingDisconnectKey, key); err != nil {
			return nil, err
		}
	}

	if req.DefaultProjectBudget != nil {
		budget := *req.DefaultProjectBudget
		if budget < 0 {
			return nil, fmt.Errorf("defaultProjectBudget must be >= 0")
		}
		if err := s.db.SetSetting(settingDefaultProjectBudget, formatFloat(budget)); err != nil {
			return nil, err
		}
	}

	for _, ba := range []struct {
		field *bool
		key   string
	}{
		{req.NotificationsEnabled, settingNotificationsEnabled},
		{req.BudgetActionWarn, settingBudgetActionWarn},
		{req.BudgetActionStopWorktrees, settingBudgetActionStopWorktrees},
		{req.BudgetActionStopContainer, settingBudgetActionStopContainer},
		{req.BudgetActionPreventStart, settingBudgetActionPreventStart},
	} {
		if ba.field != nil {
			val := "false"
			if *ba.field {
				val = "true"
			}
			if err := s.db.SetSetting(ba.key, val); err != nil {
				return nil, err
			}
		}
	}

	return &UpdateSettingsResult{RestartRequired: restartRequired}, nil
}

// budgetActions holds the resolved budget enforcement settings.
type budgetActions struct {
	warn          bool
	stopWorktrees bool
	stopContainer bool
	preventStart  bool
}

// getBudgetActions reads the four budget enforcement settings from the DB.
func (s *Service) getBudgetActions() budgetActions {
	return budgetActions{
		warn:          s.db.GetSetting(settingBudgetActionWarn, "true") == "true",
		stopWorktrees: s.db.GetSetting(settingBudgetActionStopWorktrees, "false") == "true",
		stopContainer: s.db.GetSetting(settingBudgetActionStopContainer, "false") == "true",
		preventStart:  s.db.GetSetting(settingBudgetActionPreventStart, "false") == "true",
	}
}
