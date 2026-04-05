package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/constants"
)

func (v *ContainerFormView) submit() tea.Cmd {
	name := v.inputs[inputName].Value()
	if name == "" {
		v.err = fmt.Errorf("container name is required")
		return nil
	}
	path := v.inputs[inputPath].Value()
	if path == "" {
		v.err = fmt.Errorf("project path is required")
		return nil
	}

	var domains []string
	raw := v.domains.Value()
	if raw != "" {
		for _, d := range strings.Split(raw, "\n") {
			d = strings.TrimSpace(d)
			if d != "" {
				domains = append(domains, d)
			}
		}
	}

	envMap := make(map[string]string)
	for _, e := range v.envVars {
		k := strings.TrimSpace(e.key)
		if k != "" && e.value != "" {
			envMap[k] = e.value
		}
	}

	// Collect valid user mounts.
	var validMounts []api.Mount
	for _, m := range v.mounts {
		if strings.TrimSpace(m.HostPath) == "" || strings.TrimSpace(m.ContainerPath) == "" {
			continue
		}
		validMounts = append(validMounts, m)
	}

	req := api.CreateContainerRequest{
		Name:            name,
		ProjectPath:     path,
		Image:           v.inputs[inputImage].Value(),
		AgentType:       agentTypes[v.agentType],
		NetworkMode:     api.NetworkMode(networkModes[v.network]),
		AllowedDomains:  domains,
		SkipPermissions: v.skipPerm,
	}
	if budget, err := strconv.ParseFloat(v.budgetInput.Value(), 64); err == nil && budget > 0 {
		req.CostBudget = budget
	}
	if len(envMap) > 0 {
		req.EnvVars = envMap
	}
	if len(validMounts) > 0 {
		req.Mounts = validMounts
	}

	// Collect enabled access item IDs.
	for _, p := range v.accessItems {
		if v.accessToggles[p.ID] {
			req.EnabledAccessItems = append(req.EnabledAccessItems, p.ID)
		}
	}

	// Collect enabled runtime IDs.
	for _, r := range v.runtimeDefaults {
		if v.runtimeToggles[r.ID] {
			req.EnabledRuntimes = append(req.EnabledRuntimes, r.ID)
		}
	}

	if v.editID != "" {
		return func() tea.Msg {
			_, err := v.client.UpdateContainer(context.Background(), v.editID, v.editAgentType, req)
			return OperationResultMsg{Operation: "update", Err: err}
		}
	}
	return func() tea.Msg {
		_, err := v.client.CreateContainer(context.Background(), "", string(req.AgentType), req)
		return OperationResultMsg{Operation: "create", Err: err}
	}
}

// toggleRuntime flips the toggle for a runtime if it is not always-enabled.
func (v *ContainerFormView) toggleRuntime(id string) {
	for _, r := range v.runtimeDefaults {
		if r.ID == id && !r.AlwaysEnabled {
			v.runtimeToggles[id] = !v.runtimeToggles[id]
			return
		}
	}
}

// toggleAccessItem flips the toggle for an access item if it is detected.
func (v *ContainerFormView) toggleAccessItem(id string) {
	if v.isAccessItemAvailable(id) {
		v.accessToggles[id] = !v.accessToggles[id]
	}
}

// isAccessItemAvailable returns whether an access item is detected on the host.
func (v *ContainerFormView) isAccessItemAvailable(id string) bool {
	for _, item := range v.accessItems {
		if item.ID == id {
			return item.Detection.Available
		}
	}
	return false
}

// refilterDefaultMounts replaces the mounts list with agent-type-filtered
// defaults. Only applies in create mode when defaults are loaded.
func (v *ContainerFormView) refilterDefaultMounts() {
	if v.editID != "" || v.defaults == nil {
		return
	}
	selected := agentTypes[v.agentType]
	v.mounts = nil
	for _, dm := range v.defaults.Mounts {
		if !isMountForAgent(dm, selected) {
			continue
		}
		v.mounts = append(v.mounts, api.Mount{
			HostPath:      dm.HostPath,
			ContainerPath: dm.ContainerPath,
			ReadOnly:      dm.ReadOnly,
		})
	}
}

// isMountForAgent returns true if a default mount belongs to the given agent type.
func isMountForAgent(dm api.DefaultMount, agentType constants.AgentType) bool {
	if dm.AgentType != "" {
		return dm.AgentType == string(agentType)
	}
	return true // non-agent mount, always include
}

// requiredContainerPath returns the container path of the required mount for
// the current agent type, or empty string if none.
func (v *ContainerFormView) requiredContainerPath() string {
	if v.defaults == nil {
		return ""
	}
	selected := agentTypes[v.agentType]
	for _, dm := range v.defaults.Mounts {
		if dm.Required && isMountForAgent(dm, selected) {
			return dm.ContainerPath
		}
	}
	return ""
}

// isRequiredMount returns true if the mount at the given index is the
// required agent config mount that cannot be removed.
func (v *ContainerFormView) isRequiredMount(index int) bool {
	if index < 0 || index >= len(v.mounts) {
		return false
	}
	rcp := v.requiredContainerPath()
	return rcp != "" && v.mounts[index].ContainerPath == rcp
}

// ensureRequiredMount checks that the required agent config mount is present
// in v.mounts, prepending it from defaults if missing.
func (v *ContainerFormView) ensureRequiredMount() {
	rcp := v.requiredContainerPath()
	if rcp == "" {
		return
	}
	for _, m := range v.mounts {
		if m.ContainerPath == rcp {
			return
		}
	}
	// Find the full default mount to copy host path and read-only flag.
	selected := agentTypes[v.agentType]
	for _, dm := range v.defaults.Mounts {
		if dm.Required && isMountForAgent(dm, selected) {
			v.mounts = append([]api.Mount{{
				HostPath:      dm.HostPath,
				ContainerPath: dm.ContainerPath,
				ReadOnly:      dm.ReadOnly,
			}}, v.mounts...)
			return
		}
	}
}

// applyTemplate applies a .warden.json template to the form state.
// Only used in create mode to pre-populate fields from the template.
func (v *ContainerFormView) applyTemplate(tmpl *api.ProjectTemplate, currentAgent constants.AgentType) {
	if tmpl.Image != "" {
		v.inputs[inputImage].SetValue(tmpl.Image)
	}
	if tmpl.SkipPermissions != nil {
		v.skipPerm = *tmpl.SkipPermissions
	}
	if tmpl.NetworkMode != "" {
		for i, m := range networkModes {
			if m == string(tmpl.NetworkMode) {
				v.network = i
				break
			}
		}
	}
	if tmpl.CostBudget != nil && *tmpl.CostBudget > 0 {
		v.budgetInput.SetValue(strconv.FormatFloat(*tmpl.CostBudget, 'f', -1, 64))
	}

	// Apply runtime toggles.
	if tmpl.Runtimes != nil {
		templateSet := make(map[string]bool, len(tmpl.Runtimes))
		for _, id := range tmpl.Runtimes {
			templateSet[id] = true
		}
		for _, r := range v.runtimeDefaults {
			v.runtimeToggles[r.ID] = templateSet[r.ID] || r.AlwaysEnabled
		}
	}

	// Apply agent-specific domains with runtime domains merged in.
	// If the template has agent-specific domain overrides, use those as
	// the base. Otherwise fall back to server defaults so that runtime
	// domains (e.g. Go's proxy.golang.org) still get merged in.
	if tmpl.NetworkMode == api.NetworkModeRestricted {
		if override, ok := tmpl.Agents[string(currentAgent)]; ok && len(override.AllowedDomains) > 0 {
			merged := mergeRuntimeDomainsForToggles(override.AllowedDomains, v.runtimeDefaults, v.runtimeToggles)
			v.domains.SetValue(strings.Join(merged, "\n"))
		} else if tmpl.Runtimes != nil && v.restrictedDomains != nil {
			baseDomains := v.restrictedDomains[string(currentAgent)]
			merged := mergeRuntimeDomainsForToggles(baseDomains, v.runtimeDefaults, v.runtimeToggles)
			v.domains.SetValue(strings.Join(merged, "\n"))
		}
	}
}

// mergeRuntimeDomainsForToggles appends runtime-contributed domains to a
// base domain list, deduplicating entries. Mirrors the frontend's
// mergeRuntimeDomains helper so both UIs show the same domain set.
func mergeRuntimeDomainsForToggles(baseDomains []string, runtimeDefaults []api.RuntimeDefault, toggles map[string]bool) []string {
	existing := make(map[string]bool, len(baseDomains))
	for _, d := range baseDomains {
		existing[d] = true
	}
	merged := append([]string{}, baseDomains...)
	for _, r := range runtimeDefaults {
		if !toggles[r.ID] {
			continue
		}
		for _, d := range r.Domains {
			if !existing[d] {
				existing[d] = true
				merged = append(merged, d)
			}
		}
	}
	return merged
}
