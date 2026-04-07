package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/constants"
	"github.com/thesimonho/warden/internal/tui/components"
)

// defaultDomainsForAgent returns the default allowed domains for a given
// agent type from the server-provided defaults map.
func defaultDomainsForAgent(restrictedDomains map[string][]string, agentType constants.AgentType) string {
	domains := restrictedDomains[string(agentType)]
	return strings.Join(domains, "\n")
}

// formStep identifies which step of the form is active.
type formStep int

const (
	stepGeneral formStep = iota
	stepEnvironment
	stepNetwork
	stepAdvanced
	stepCount
)

// stepLabels are human-readable names for each step.
var stepLabels = [stepCount]string{"General", "Environment", "Network", "Advanced"}

// General step field indices.
const (
	genAgentType = iota
	genName
	genPath
	genSkipPerms
	genBudget
	genSubmit
	genFieldCount
)

// Environment step field indices.
const (
	envRuntimes = iota
	envAccessItems
	envSubmit
	envFieldCount
)

// Network step field indices.
const (
	netNetwork = iota
	netDomains // only visible when network == "restricted"
	netPorts
	netSubmit
	netFieldCount
)

// Advanced step field indices.
const (
	advImage = iota
	advMounts
	advEnvVars
	advSubmit
	advFieldCount
)

// Agent type options sourced from the agent registry.
var agentTypes = agent.AllTypes

// agentTypeLabels maps agent type IDs to display labels.
var agentTypeLabels = agent.DisplayLabels

// Network mode options.
var networkModes = []string{"full", "restricted", "none"}

// defaultContainerImage is the default warden container image.
const defaultContainerImage = "ghcr.io/thesimonho/warden:latest"

// Named indices into the inputs[3] array for readability.
const (
	inputName  = 0
	inputPath  = 1
	inputImage = 2
)

// networkDescriptions provides help text for each network mode.
var networkDescriptions = map[string]string{
	"full":       "Unrestricted internet access",
	"restricted": "Only specified domains allowed",
	"none":       "All outbound traffic blocked",
}

// ContainerFormView handles creating or editing a container using
// native bubbles components instead of huh.
type ContainerFormView struct {
	client        Client
	editID        string
	editAgentType string
	defaults      *api.DefaultsResponse
	loading       bool
	err           error

	// Field state.
	step        formStep // current step
	fieldCursor int      // field index within the current step
	editing     bool     // true when a text field is actively receiving input
	browsing    bool     // true when the directory browser is open

	// Text input fields.
	inputs      [3]textinput.Model // name, path, image
	budgetInput textinput.Model
	domains     textarea.Model

	// Selection fields.
	agentType int // index into agentTypes
	network   int // index into networkModes
	skipPerm  bool

	// Runtimes (Node, Python, Go, etc.).
	runtimeDefaults []api.RuntimeDefault
	runtimeToggles  map[string]bool
	runtimeCursor   int // sub-cursor within runtimes (-1 = header)

	// Access items (Git, SSH, user-defined toggles).
	accessItems   []api.AccessItemResponse
	accessToggles map[string]bool
	accessCursor  int // sub-cursor within access items (-1 = header)

	// Bind mounts.
	mounts       []api.Mount
	mountCursor  int // sub-cursor within the mounts list (-1 = header/add)
	editingMount bool
	mountIsNew   bool               // true when editing a newly added mount
	mountInputs  [2]textinput.Model // host path, container path

	// Environment variables.
	envVars       []envVarEntry
	envCursor     int // sub-cursor within the env vars list (-1 = header/add)
	editingEnv    bool
	envIsNew      bool               // true when editing a newly added env var
	envInputs     [2]textinput.Model // key, value
	envInputFocus int                // 0=key, 1=value

	// Forwarded ports.
	forwardedPorts []int
	portCursor     int // sub-cursor within the ports list (-1 = header/add)
	editingPort    bool
	portIsNew      bool
	portInput      textinput.Model

	dirBrowser *components.DirectoryBrowser

	// Server-provided restricted domains per agent type.
	restrictedDomains map[string][]string

	keys   FormKeyMap
	width  int
	height int
}

// IsCapturingInput reports whether the form is actively capturing text
// input (editing a field, inline mount/env/port editing, or browsing).
func (v *ContainerFormView) IsCapturingInput() bool {
	return v.editing || v.editingMount || v.editingEnv || v.editingPort || v.browsing
}

// envVarEntry is a single key-value environment variable.
type envVarEntry struct {
	key   string
	value string
}

// containerConfigLoadedMsg carries the container config for editing.
type containerConfigLoadedMsg struct {
	Config *api.ContainerConfig
	Err    error
}

// fieldCountForStep returns the number of fields in a given step.
func fieldCountForStep(s formStep) int {
	switch s {
	case stepGeneral:
		return genFieldCount
	case stepEnvironment:
		return envFieldCount
	case stepNetwork:
		return netFieldCount
	case stepAdvanced:
		return advFieldCount
	}
	return 0
}

// NewContainerFormView creates a container creation form.
func NewContainerFormView(client Client) *ContainerFormView {
	v := &ContainerFormView{
		client:         client,
		loading:        true,
		step:           stepGeneral,
		keys:           DefaultFormKeyMap(),
		accessToggles:  make(map[string]bool),
		runtimeToggles: make(map[string]bool),
	}
	v.initFields("", "", defaultContainerImage, "full", "", false)
	return v
}

// NewContainerEditView creates a container editing form.
func NewContainerEditView(client Client, editID, editAgentType string) *ContainerFormView {
	v := &ContainerFormView{
		client:         client,
		editID:         editID,
		editAgentType:  editAgentType,
		loading:        true,
		step:           stepGeneral,
		keys:           DefaultFormKeyMap(),
		accessToggles:  make(map[string]bool),
		runtimeToggles: make(map[string]bool),
	}
	v.initFields("", "", "", "", "", false)
	return v
}

func (v *ContainerFormView) initFields(name, path, image, network, domains string, skipPerm bool) {
	for i := range v.inputs {
		v.inputs[i] = textinput.New()
		v.inputs[i].Prompt = ""
	}
	v.inputs[inputName].Placeholder = "my-project"
	v.inputs[inputName].SetValue(name)
	v.inputs[inputPath].Placeholder = "/home/user/project"
	v.inputs[inputPath].SetValue(path)
	v.inputs[inputImage].Placeholder = defaultContainerImage
	v.inputs[inputImage].SetValue(image)

	v.budgetInput = textinput.New()
	v.budgetInput.Prompt = ""
	v.budgetInput.Placeholder = "unlimited"

	v.domains = textarea.New()
	v.domains.Placeholder = "one domain per line"
	v.domains.SetValue(domains)
	v.domains.SetHeight(10)
	v.domains.ShowLineNumbers = false

	for i, m := range networkModes {
		if m == network {
			v.network = i
			break
		}
	}
	v.skipPerm = skipPerm

	for i := range v.mountInputs {
		ti := textinput.New()
		ti.Prompt = ""
		ti.SetWidth(30)
		v.mountInputs[i] = ti
	}
	v.mountInputs[0].Placeholder = "/host/path"
	v.mountInputs[1].Placeholder = "/container/path"

	for i := range v.envInputs {
		ti := textinput.New()
		ti.Prompt = ""
		ti.SetWidth(30)
		v.envInputs[i] = ti
	}
	v.envInputs[0].Placeholder = "VARIABLE_NAME"
	v.envInputs[1].Placeholder = "variable value"

	v.portInput = textinput.New()
	v.portInput.Prompt = ""
	v.portInput.Placeholder = "8080"
	v.portInput.SetWidth(10)
	v.portCursor = -1
}

// Init fetches defaults, settings, and container config (if editing).
func (v *ContainerFormView) Init() tea.Cmd {
	v.loading = true
	cmds := []tea.Cmd{
		func() tea.Msg {
			defaults, err := v.client.GetDefaults(context.Background(), v.inputs[inputPath].Value())
			return DefaultsLoadedMsg{Defaults: defaults, Err: err}
		},
		func() tea.Msg {
			settings, err := v.client.GetSettings(context.Background())
			return SettingsLoadedMsg{Settings: settings, Err: err}
		},
		func() tea.Msg {
			resp, err := v.client.ListAccessItems(context.Background())
			if err != nil {
				return AccessItemsLoadedMsg{Err: err}
			}
			return AccessItemsLoadedMsg{Items: resp.Items}
		},
	}
	if v.editID != "" {
		cmds = append(cmds, func() tea.Msg {
			cfg, err := v.client.InspectContainer(context.Background(), v.editID, v.editAgentType)
			return containerConfigLoadedMsg{Config: cfg, Err: err}
		})
	}
	return tea.Batch(cmds...)
}

// Update handles messages for the container form.
func (v *ContainerFormView) Update(msg tea.Msg) (View, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		return v, nil

	case DefaultsLoadedMsg:
		if msg.Err != nil {
			v.err = msg.Err
			v.loading = false
			return v, nil
		}
		v.defaults = msg.Defaults
		if v.inputs[inputPath].Value() == "" && v.defaults.HomeDir != "" {
			v.inputs[inputPath].SetValue(v.defaults.HomeDir)
		}

		if v.editID == "" && len(v.mounts) == 0 && len(v.defaults.Mounts) > 0 {
			selected := agentTypes[v.agentType]
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
		if v.editID == "" && len(v.envVars) == 0 && len(v.defaults.EnvVars) > 0 {
			for _, ev := range v.defaults.EnvVars {
				v.envVars = append(v.envVars, envVarEntry{key: ev.Key, value: ev.Value})
			}
		}
		if v.defaults.RestrictedDomains != nil {
			v.restrictedDomains = v.defaults.RestrictedDomains
		}
		if len(v.defaults.Runtimes) > 0 {
			v.runtimeDefaults = v.defaults.Runtimes
			if v.editID == "" {
				for _, r := range v.runtimeDefaults {
					v.runtimeToggles[r.ID] = r.AlwaysEnabled || r.Detected
				}
			}
		}
		if v.editID == "" {
			// Set initial allowed domains from server defaults for the selected agent type.
			if v.restrictedDomains != nil {
				selected := agentTypes[v.agentType]
				v.domains.SetValue(defaultDomainsForAgent(v.restrictedDomains, selected))
			}

			// Apply project template overrides if .warden.json was found.
			if v.defaults.Template != nil {
				v.applyTemplate(v.defaults.Template, agentTypes[v.agentType])
			}

			v.loading = false
		}
		return v, nil

	case AccessItemsLoadedMsg:
		if msg.Err == nil && len(msg.Items) > 0 {
			v.accessItems = msg.Items
			if v.editID == "" {
				// Create mode: enable all detected access items.
				for _, item := range v.accessItems {
					v.accessToggles[item.ID] = item.Detection.Available
				}
			}
		}
		return v, nil

	case SettingsLoadedMsg:
		if msg.Err == nil && msg.Settings != nil && v.editID == "" {
			if msg.Settings.DefaultProjectBudget > 0 {
				v.budgetInput.SetValue(fmt.Sprintf("%.2f", msg.Settings.DefaultProjectBudget))
			}
		}
		return v, nil

	case containerConfigLoadedMsg:
		if msg.Err != nil {
			v.err = msg.Err
			v.loading = false
			return v, nil
		}
		v.inputs[inputName].SetValue(msg.Config.Name)
		v.inputs[inputPath].SetValue(msg.Config.ProjectPath)
		v.inputs[inputImage].SetValue(msg.Config.Image)
		for i, at := range agentTypes {
			if at == msg.Config.AgentType {
				v.agentType = i
				break
			}
		}
		for i, m := range networkModes {
			if m == string(msg.Config.NetworkMode) {
				v.network = i
				break
			}
		}
		v.domains.SetValue(strings.Join(msg.Config.AllowedDomains, "\n"))
		v.skipPerm = msg.Config.SkipPermissions
		if msg.Config.CostBudget > 0 {
			v.budgetInput.SetValue(fmt.Sprintf("%.2f", msg.Config.CostBudget))
		}
		if len(msg.Config.Mounts) > 0 {
			v.mounts = msg.Config.Mounts
		}
		// Ensure the required agent config mount is present (covers
		// projects created before this mount became mandatory).
		v.ensureRequiredMount()
		if len(msg.Config.EnvVars) > 0 {
			for k, val := range msg.Config.EnvVars {
				v.envVars = append(v.envVars, envVarEntry{key: k, value: val})
			}
		}

		// Set toggle state from stored access items.
		for _, id := range msg.Config.EnabledAccessItems {
			v.accessToggles[id] = true
		}

		// Set toggle state from stored runtimes.
		if len(msg.Config.EnabledRuntimes) > 0 {
			enabled := make(map[string]bool, len(msg.Config.EnabledRuntimes))
			for _, id := range msg.Config.EnabledRuntimes {
				enabled[id] = true
			}
			for _, r := range v.runtimeDefaults {
				v.runtimeToggles[r.ID] = r.AlwaysEnabled || enabled[r.ID]
			}
		}

		if len(msg.Config.ForwardedPorts) > 0 {
			v.forwardedPorts = msg.Config.ForwardedPorts
		}

		v.loading = false
		return v, nil

	case OperationResultMsg:
		if msg.Err != nil {
			v.err = msg.Err
		} else {
			return v, func() tea.Msg { return NavigateBackMsg{} }
		}
		return v, nil

	case components.DirectoryBrowserMsg:
		if v.browsing && v.dirBrowser != nil {
			var cmd tea.Cmd
			v.dirBrowser, cmd = v.dirBrowser.Update(msg)
			return v, cmd
		}

	case tea.KeyPressMsg:
		if v.err != nil {
			v.err = nil
			return v, nil
		}
		return v.handleKey(msg)
	}

	if v.editing || v.editingMount || v.editingEnv || v.editingPort {
		return v.updateActiveField(msg)
	}

	return v, nil
}
