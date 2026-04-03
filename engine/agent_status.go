package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/constants"
)

// ReadAgentStatus reads the agent config file from a running container
// and extracts per-project status data. Returns a map keyed by the
// working directory path inside the container.
//
// This is the Go-side equivalent of the jq extraction in warden-event.sh,
// but more reliable: it uses proper JSON parsing, doesn't depend on
// WARDEN_EVENT_DIR being set, and runs from the host via docker exec.
func (ec *EngineClient) ReadAgentStatus(ctx context.Context, containerID string) (map[string]*agent.Status, error) {
	provider := ec.resolveProvider(ctx, containerID)
	if provider == nil {
		return nil, fmt.Errorf("no agent provider configured")
	}

	raw, err := ec.readAgentConfigRaw(ctx, containerID, provider)
	if err != nil {
		return nil, err
	}
	return provider.ExtractStatus(raw), nil
}

// resolveProvider returns the StatusProvider for a container by reading
// its WARDEN_AGENT_TYPE env var and looking up the registry. The agent
// type is immutable per container (set at creation), so the result is
// cached to avoid repeated ContainerInspect calls on hot paths.
func (ec *EngineClient) resolveProvider(ctx context.Context, containerID string) agent.StatusProvider {
	if ec.agentRegistry == nil {
		return nil
	}

	agentType := ec.cachedAgentType(ctx, containerID)
	return ec.agentRegistry.Resolve(agentType)
}

// cachedAgentType returns the WARDEN_AGENT_TYPE for a container, reading
// from the container env on first call and caching for subsequent calls.
func (ec *EngineClient) cachedAgentType(ctx context.Context, containerID string) constants.AgentType {
	if cached, ok := ec.agentTypeCache.Load(containerID); ok {
		return cached.(constants.AgentType)
	}

	info, err := ec.api.ContainerInspect(ctx, containerID)
	if err != nil {
		return ""
	}
	agentType := constants.AgentType(envValue(info.Config.Env, "WARDEN_AGENT_TYPE"))
	ec.agentTypeCache.Store(containerID, agentType)
	return agentType
}

// readAgentConfigRaw reads the raw agent config file bytes from a container.
// Shared by ReadAgentStatus and IsEstimatedCost to avoid duplicate docker exec calls.
func (ec *EngineClient) readAgentConfigRaw(ctx context.Context, containerID string, provider agent.StatusProvider) ([]byte, error) {
	if provider == nil {
		return nil, fmt.Errorf("no agent provider configured")
	}

	configPath := provider.ConfigFilePath()
	output, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"cat", configPath},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("reading agent config from container: %w", err)
	}

	return []byte(output), nil
}

// defaultWorkspacePrefix is the legacy path prefix. Used as fallback
// when no workspace dir is specified.
const defaultWorkspacePrefix = "/project"

// billingTypeSubscription identifies Pro/Max subscription users in Claude Code's config.
const billingTypeSubscription = "stripe_subscription"

// SessionCost holds cost data for a single agent session.
type SessionCost struct {
	SessionID string
	Cost      float64
}

// AgentCostResult holds cost and billing type from a single config read.
type AgentCostResult struct {
	TotalCost   float64
	IsEstimated bool
	// Sessions holds per-session cost breakdown, keyed by session ID.
	// Used for session-keyed DB persistence.
	Sessions []SessionCost
}

// ReadAgentCostAndBillingType reads the agent config file once and
// extracts both cost (filtered by workspace prefix) and billing type.
// Returns per-session cost breakdown for session-keyed DB persistence.
func (ec *EngineClient) ReadAgentCostAndBillingType(ctx context.Context, containerID, workspacePrefix string) (*AgentCostResult, error) {
	provider := ec.resolveProvider(ctx, containerID)
	if provider == nil {
		return &AgentCostResult{}, nil
	}

	raw, err := ec.readAgentConfigRaw(ctx, containerID, provider)
	if err != nil {
		return nil, err
	}

	statuses := provider.ExtractStatus(raw)
	sessions := sessionCostsFromStatuses(statuses, workspacePrefix)
	if len(sessions) == 0 {
		return &AgentCostResult{}, nil
	}

	var total float64
	for _, s := range sessions {
		total += s.Cost
	}

	return &AgentCostResult{
		TotalCost:   total,
		IsEstimated: isEstimatedCostFromConfig(raw),
		Sessions:    sessions,
	}, nil
}

// ProjectCostFromContainerStatuses sums cost only for entries whose
// path starts with the given workspace prefix. This filters out host
// project entries that appear in the bind-mounted .claude.json but
// don't belong to this container.
func ProjectCostFromContainerStatuses(statuses map[string]*agent.Status, workspacePrefix string) float64 {
	if workspacePrefix == "" {
		workspacePrefix = defaultWorkspacePrefix
	}
	var total float64
	for path, s := range statuses {
		if s != nil && strings.HasPrefix(path, workspacePrefix) {
			total += s.CostUSD
		}
	}
	return total
}

// sessionCostsFromStatuses extracts per-session cost entries for paths
// matching the workspace prefix. Only includes entries with a known
// session ID and positive cost, since these are persisted to the DB
// keyed by session ID.
func sessionCostsFromStatuses(statuses map[string]*agent.Status, workspacePrefix string) []SessionCost {
	if workspacePrefix == "" {
		workspacePrefix = defaultWorkspacePrefix
	}
	var sessions []SessionCost
	for path, s := range statuses {
		if s == nil || !strings.HasPrefix(path, workspacePrefix) {
			continue
		}
		if s.AgentSessionID == "" || s.CostUSD <= 0 {
			continue
		}
		sessions = append(sessions, SessionCost{
			SessionID: s.AgentSessionID,
			Cost:      s.CostUSD,
		})
	}
	return sessions
}

// IsEstimatedCost checks whether a container is using estimated cost
// (subscription user) vs actual API cost. Reads oauthAccount.billingType
// from .claude.json — "stripe_subscription" means estimated cost.
// Falls back to true (estimated) if the billing type can't be determined.
func (ec *EngineClient) IsEstimatedCost(ctx context.Context, containerID string) bool {
	provider := ec.resolveProvider(ctx, containerID)
	if provider == nil {
		return true
	}
	raw, err := ec.readAgentConfigRaw(ctx, containerID, provider)
	if err != nil {
		return true
	}
	return isEstimatedCostFromConfig(raw)
}

// isEstimatedCostFromConfig extracts billing type from raw config bytes.
// Used to avoid a second docker exec when ReadAgentStatus already fetched the file.
func isEstimatedCostFromConfig(raw []byte) bool {
	var config struct {
		OAuthAccount *struct {
			BillingType string `json:"billingType"`
		} `json:"oauthAccount"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return true
	}
	return config.OAuthAccount != nil && config.OAuthAccount.BillingType == billingTypeSubscription
}
