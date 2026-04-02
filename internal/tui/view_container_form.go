package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/constants"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/internal/tui/components"
)

// defaultAllowedDomains is the minimum useful set for AI coding agents in
// restricted network mode. Matches web/src/lib/domain-groups.ts.
const defaultAllowedDomains = `*.anthropic.com
*.openai.com
*.chatgpt.com
*.github.com
*.githubusercontent.com
pypi.org
files.pythonhosted.org
registry.npmjs.org
registry.yarnpkg.com
go.dev
proxy.golang.org
sum.golang.org`

// Form field indices.
const (
	fieldAgentType = iota
	fieldName
	fieldPath
	fieldSkipPerms
	fieldBudget
	fieldNetwork
	fieldDomains // only visible when network == "restricted"
	fieldAdvanced
	// --- Advanced fields (visible when advancedOpen) ---
	fieldImage
	fieldAccessItems // dynamic access item toggles (Git, SSH, user-defined)
	fieldMounts      // section header with "add" action
	fieldEnvVars     // section header with "add" action
	fieldSubmit
	fieldCount
)

// Agent type options sourced from the agent registry.
var agentTypes = agent.AllTypes

// agentTypeLabels maps agent type IDs to display labels.
var agentTypeLabels = agent.DisplayLabels

// Network mode options.
var networkModes = []string{"full", "restricted", "none"}

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
	defaults *api.DefaultsResponse
	loading  bool
	err      error

	// Field state.
	cursor   int
	editing  bool // true when a text field is actively receiving input
	browsing bool // true when the directory browser is open

	// Text input fields.
	inputs      [3]textinput.Model // name, path, image
	budgetInput textinput.Model
	domains     textarea.Model

	// Selection fields.
	agentType int // index into agentTypes
	network   int // index into networkModes
	skipPerm  bool

	// Advanced section.
	advancedOpen bool

	// Access items (Git, SSH, user-defined toggles).
	accessItems   []api.AccessItemResponse
	accessToggles map[string]bool
	accessCursor  int // sub-cursor within access items (-1 = header)

	// Bind mounts.
	mounts       []engine.Mount
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

	dirBrowser *components.DirectoryBrowser

	keys   FormKeyMap
	width  int
	height int
}

// envVarEntry is a single key-value environment variable.
type envVarEntry struct {
	key   string
	value string
}

// containerConfigLoadedMsg carries the container config for editing.
type containerConfigLoadedMsg struct {
	Config *engine.ContainerConfig
	Err    error
}

// NewContainerFormView creates a container creation form.
func NewContainerFormView(client Client) *ContainerFormView {
	v := &ContainerFormView{
		client:        client,
		loading:       true,
		keys:          DefaultFormKeyMap(),
		accessToggles: make(map[string]bool),
	}
	v.initFields("", "", "ghcr.io/thesimonho/warden:latest", "full", defaultAllowedDomains, false)
	return v
}

// NewContainerEditView creates a container editing form.
func NewContainerEditView(client Client, editID, editAgentType string) *ContainerFormView {
	v := &ContainerFormView{
		client:        client,
		editID:        editID,
		editAgentType: editAgentType,
		loading:       true,
		keys:          DefaultFormKeyMap(),
		accessToggles: make(map[string]bool),
	}
	v.initFields("", "", "", "", "", false)
	return v
}

func (v *ContainerFormView) initFields(name, path, image, network, domains string, skipPerm bool) {
	for i := range v.inputs {
		v.inputs[i] = textinput.New()
		v.inputs[i].Prompt = ""
	}
	v.inputs[0].Placeholder = "my-project"
	v.inputs[0].SetValue(name)
	v.inputs[1].Placeholder = "/home/user/project"
	v.inputs[1].SetValue(path)
	v.inputs[2].Placeholder = "ghcr.io/thesimonho/warden:latest"
	v.inputs[2].SetValue(image)

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
}

// Init fetches defaults, settings, and container config (if editing).
func (v *ContainerFormView) Init() tea.Cmd {
	v.loading = true
	cmds := []tea.Cmd{
		func() tea.Msg {
			defaults, err := v.client.GetDefaults(context.Background())
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
		if v.inputs[1].Value() == "" && v.defaults.HomeDir != "" {
			v.inputs[1].SetValue(v.defaults.HomeDir)
		}

		if v.editID == "" && len(v.mounts) == 0 && len(v.defaults.Mounts) > 0 {
			selected := agentTypes[v.agentType]
			for _, dm := range v.defaults.Mounts {
				if !isMountForAgent(dm, selected) {
					continue
				}
				v.mounts = append(v.mounts, engine.Mount{
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
		if v.editID == "" {
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
		v.inputs[0].SetValue(msg.Config.Name)
		v.inputs[1].SetValue(msg.Config.ProjectPath)
		v.inputs[2].SetValue(msg.Config.Image)
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
		// Access items are resolved server-side; no client-side stripping needed.

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

	if v.editing || v.editingMount || v.editingEnv {
		return v.updateActiveField(msg)
	}

	return v, nil
}

func (v *ContainerFormView) handleKey(msg tea.KeyPressMsg) (View, tea.Cmd) {
	if v.browsing && v.dirBrowser != nil {
		return v.handleBrowsingKey(msg)
	}
	if v.editingMount {
		// Required mounts only allow editing the host path — skip tab to container path.
		if msg.String() == "tab" && v.isRequiredMount(v.mountCursor) {
			return v, nil
		}
		return v.handleInlineEditKey(msg, &v.mountInputs, v.cancelMountEdit, v.saveMountInputs)
	}
	if v.editingEnv {
		return v.handleInlineEditKey(msg, &v.envInputs, v.cancelEnvEdit, v.saveEnvInputs)
	}
	if v.editing {
		if msg.String() == "esc" {
			v.blurActiveField()
			v.editing = false
			return v, nil
		}
		return v.updateActiveField(msg)
	}

	// Navigation mode.
	switch {
	case msg.String() == "esc":
		return v, func() tea.Msg { return NavigateBackMsg{} }
	case msg.String() == "up" || msg.String() == "k":
		v.moveCursor(-1)
	case msg.String() == "down" || msg.String() == "j":
		v.moveCursor(1)
	case msg.String() == "enter" || msg.String() == " ":
		return v.activateField()
	case msg.String() == "tab":
		return v.cycleSelection()
	case msg.String() == "x":
		return v.removeCurrentItem()
	case msg.String() == "r":
		if v.cursor == fieldMounts && v.mountCursor >= 0 && v.mountCursor < len(v.mounts) {
			v.mounts[v.mountCursor].ReadOnly = !v.mounts[v.mountCursor].ReadOnly
		}
	}

	return v, nil
}

// handleInlineEditKey handles keys for mount/env inline editing.
// The two-input tab/enter/esc pattern is identical for both.
func (v *ContainerFormView) handleInlineEditKey(
	msg tea.KeyPressMsg,
	inputs *[2]textinput.Model,
	cancelFn func(),
	saveFn func(),
) (View, tea.Cmd) {
	switch msg.String() {
	case "enter":
		saveFn()
		return v, nil
	case "esc":
		cancelFn()
		return v, nil
	case "tab":
		if inputs[0].Focused() {
			inputs[0].Blur()
			return v, inputs[1].Focus()
		}
		inputs[1].Blur()
		return v, inputs[0].Focus()
	}
	return v.updateActiveField(msg)
}

func (v *ContainerFormView) saveMountInputs() {
	if v.mountCursor >= 0 && v.mountCursor < len(v.mounts) {
		v.mounts[v.mountCursor].HostPath = v.mountInputs[0].Value()
		v.mounts[v.mountCursor].ContainerPath = v.mountInputs[1].Value()
	}
	v.editingMount = false
	v.mountIsNew = false
	v.mountInputs[0].Blur()
	v.mountInputs[1].Blur()
}

func (v *ContainerFormView) cancelMountEdit() {
	if v.mountIsNew {
		v.mounts = append(v.mounts[:v.mountCursor], v.mounts[v.mountCursor+1:]...)
		if v.mountCursor >= len(v.mounts) {
			v.mountCursor = max(len(v.mounts)-1, -1)
		}
	}
	v.editingMount = false
	v.mountIsNew = false
	v.mountInputs[0].Blur()
	v.mountInputs[1].Blur()
}

func (v *ContainerFormView) saveEnvInputs() {
	if v.envCursor >= 0 && v.envCursor < len(v.envVars) {
		v.envVars[v.envCursor].key = v.envInputs[0].Value()
		v.envVars[v.envCursor].value = v.envInputs[1].Value()
	}
	v.editingEnv = false
	v.envIsNew = false
	v.envInputs[0].Blur()
	v.envInputs[1].Blur()
}

func (v *ContainerFormView) cancelEnvEdit() {
	if v.envIsNew {
		v.envVars = append(v.envVars[:v.envCursor], v.envVars[v.envCursor+1:]...)
		if v.envCursor >= len(v.envVars) {
			v.envCursor = max(len(v.envVars)-1, -1)
		}
	}
	v.editingEnv = false
	v.envIsNew = false
	v.envInputs[0].Blur()
	v.envInputs[1].Blur()
}

func (v *ContainerFormView) handleBrowsingKey(msg tea.KeyPressMsg) (View, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.browsing = false
		v.dirBrowser = nil
		return v, nil
	case "space", " ":
		v.inputs[1].SetValue(v.dirBrowser.Path())
		v.browsing = false
		v.dirBrowser = nil
		return v, nil
	}
	var cmd tea.Cmd
	v.dirBrowser, cmd = v.dirBrowser.Update(msg)
	return v, cmd
}

// cycleSelection cycles the value of selection fields (tab key).
func (v *ContainerFormView) cycleSelection() (View, tea.Cmd) {
	switch v.cursor {
	case fieldAgentType:
		if v.editID == "" { // read-only in edit mode
			v.agentType = (v.agentType + 1) % len(agentTypes)
			v.refilterDefaultMounts()
		}
	case fieldNetwork:
		v.network = (v.network + 1) % len(networkModes)
	case fieldSkipPerms:
		v.skipPerm = !v.skipPerm
	case fieldAccessItems:
		if v.accessCursor >= 0 && v.accessCursor < len(v.accessItems) {
			v.toggleAccessItem(v.accessItems[v.accessCursor].ID)
		}
	}
	return v, nil
}

// isFieldVisible returns whether a field should be shown.
func (v *ContainerFormView) isFieldVisible(field int) bool {
	switch field {
	case fieldDomains:
		return networkModes[v.network] == "restricted"
	case fieldImage, fieldAccessItems, fieldMounts, fieldEnvVars:
		return v.advancedOpen
	}
	return true
}

// moveCursor moves the cursor by delta, skipping hidden fields.
// For access/mount/env sections, navigates sub-items.
func (v *ContainerFormView) moveCursor(delta int) {
	if v.cursor == fieldAccessItems {
		next := v.accessCursor + delta
		if next < 0 {
			v.accessCursor = 0
			v.moveCursorField(delta)
			return
		}
		if next >= len(v.accessItems) {
			v.moveCursorField(delta)
			return
		}
		v.accessCursor = next
		return
	}

	if v.cursor == fieldMounts {
		next := v.mountCursor + delta
		if next < -1 {
			v.mountCursor = -1
			v.moveCursorField(delta)
			return
		}
		if next >= len(v.mounts) {
			v.moveCursorField(delta)
			return
		}
		v.mountCursor = next
		return
	}

	if v.cursor == fieldEnvVars {
		next := v.envCursor + delta
		if next < -1 {
			v.envCursor = -1
			v.moveCursorField(delta)
			return
		}
		if next >= len(v.envVars) {
			v.moveCursorField(delta)
			return
		}
		v.envCursor = next
		return
	}

	v.moveCursorField(delta)
}

// moveCursorField moves the main field cursor, skipping hidden fields.
func (v *ContainerFormView) moveCursorField(delta int) {
	next := v.cursor + delta
	for next >= 0 && next < fieldCount {
		if v.isFieldVisible(next) {
			v.cursor = next
			if next == fieldAccessItems {
				if delta > 0 {
					v.accessCursor = 0
				} else {
					v.accessCursor = max(len(v.accessItems)-1, 0)
				}
			}
			if next == fieldMounts {
				if delta > 0 {
					v.mountCursor = -1
				} else {
					v.mountCursor = max(len(v.mounts)-1, -1)
				}
			}
			if next == fieldEnvVars {
				if delta > 0 {
					v.envCursor = -1
				} else {
					v.envCursor = max(len(v.envVars)-1, -1)
				}
			}
			return
		}
		next += delta
	}
}

func (v *ContainerFormView) activateField() (View, tea.Cmd) {
	switch v.cursor {
	case fieldName, fieldImage:
		idx := 0
		if v.cursor == fieldImage {
			idx = 2
		}
		v.editing = true
		return v, v.inputs[idx].Focus()

	case fieldBudget:
		v.editing = true
		return v, v.budgetInput.Focus()

	case fieldPath:
		return v.openDirectoryBrowser()

	case fieldDomains:
		v.editing = true
		return v, v.domains.Focus()

	case fieldAgentType:
		if v.editID == "" {
			v.agentType = (v.agentType + 1) % len(agentTypes)
			v.refilterDefaultMounts()
		}
	case fieldNetwork:
		v.network = (v.network + 1) % len(networkModes)
	case fieldSkipPerms:
		v.skipPerm = !v.skipPerm
	case fieldAdvanced:
		v.advancedOpen = !v.advancedOpen
	case fieldAccessItems:
		if v.accessCursor >= 0 && v.accessCursor < len(v.accessItems) {
			v.toggleAccessItem(v.accessItems[v.accessCursor].ID)
		}

	case fieldMounts:
		return v.activateMountField()
	case fieldEnvVars:
		return v.activateEnvField()
	case fieldSubmit:
		return v, v.submit()
	}
	return v, nil
}

func (v *ContainerFormView) activateMountField() (View, tea.Cmd) {
	if v.mountCursor == -1 {
		v.mounts = append(v.mounts, engine.Mount{ReadOnly: true})
		v.mountCursor = len(v.mounts) - 1
		v.mountIsNew = true
		return v.startMountEdit()
	}
	if v.mountCursor >= 0 && v.mountCursor < len(v.mounts) {
		v.mountIsNew = false
		return v.startMountEdit()
	}
	return v, nil
}

func (v *ContainerFormView) startMountEdit() (View, tea.Cmd) {
	m := v.mounts[v.mountCursor]
	v.mountInputs[0].SetValue(m.HostPath)
	v.mountInputs[1].SetValue(m.ContainerPath)
	v.editingMount = true
	// Container path must stay at the agent's expected location;
	// only let the user remap which host directory backs it.
	if v.isRequiredMount(v.mountCursor) {
		v.mountInputs[1].Blur()
	}
	return v, v.mountInputs[0].Focus()
}

func (v *ContainerFormView) activateEnvField() (View, tea.Cmd) {
	if v.envCursor == -1 {
		v.envVars = append(v.envVars, envVarEntry{})
		v.envCursor = len(v.envVars) - 1
		v.envIsNew = true
		return v.startEnvEdit()
	}
	if v.envCursor >= 0 && v.envCursor < len(v.envVars) {
		v.envIsNew = false
		return v.startEnvEdit()
	}
	return v, nil
}

func (v *ContainerFormView) startEnvEdit() (View, tea.Cmd) {
	e := v.envVars[v.envCursor]
	v.envInputs[0].SetValue(e.key)
	v.envInputs[1].SetValue(e.value)
	v.editingEnv = true
	v.envInputFocus = 0
	return v, v.envInputs[0].Focus()
}

// removeCurrentItem removes the selected mount or env var.
func (v *ContainerFormView) removeCurrentItem() (View, tea.Cmd) {
	if v.cursor == fieldMounts && v.mountCursor >= 0 && v.mountCursor < len(v.mounts) {
		if v.isRequiredMount(v.mountCursor) {
			return v, nil // agent won't function without its config directory
		}
		v.mounts = append(v.mounts[:v.mountCursor], v.mounts[v.mountCursor+1:]...)
		if v.mountCursor >= len(v.mounts) {
			v.mountCursor = len(v.mounts) - 1
		}
		if len(v.mounts) == 0 {
			v.mountCursor = -1
		}
		return v, nil
	}
	if v.cursor == fieldEnvVars && v.envCursor >= 0 && v.envCursor < len(v.envVars) {
		v.envVars = append(v.envVars[:v.envCursor], v.envVars[v.envCursor+1:]...)
		if v.envCursor >= len(v.envVars) {
			v.envCursor = len(v.envVars) - 1
		}
		if len(v.envVars) == 0 {
			v.envCursor = -1
		}
		return v, nil
	}
	return v, nil
}

func (v *ContainerFormView) openDirectoryBrowser() (View, tea.Cmd) {
	startPath := v.inputs[1].Value()
	if startPath == "" && v.defaults != nil && v.defaults.HomeDir != "" {
		startPath = v.defaults.HomeDir
	}
	if startPath == "" {
		startPath = "/"
	}
	v.dirBrowser = components.NewDirectoryBrowser(startPath, func(path string) tea.Cmd {
		return func() tea.Msg {
			entries, err := v.client.ListDirectories(context.Background(), path, false)
			return components.DirectoryBrowserMsg{Path: path, Entries: entries, Err: err}
		}
	})
	v.browsing = true
	return v, v.dirBrowser.Init()
}

func (v *ContainerFormView) blurActiveField() {
	switch v.cursor {
	case fieldName:
		v.inputs[0].Blur()
	case fieldPath:
		v.inputs[1].Blur()
	case fieldImage:
		v.inputs[2].Blur()
	case fieldBudget:
		v.budgetInput.Blur()
	case fieldDomains:
		v.domains.Blur()
	}
}

func (v *ContainerFormView) updateActiveField(msg tea.Msg) (View, tea.Cmd) {
	var cmd tea.Cmd
	switch {
	case v.editingMount:
		if v.mountInputs[0].Focused() {
			v.mountInputs[0], cmd = v.mountInputs[0].Update(msg)
		} else {
			v.mountInputs[1], cmd = v.mountInputs[1].Update(msg)
		}
	case v.editingEnv:
		if v.envInputs[0].Focused() {
			v.envInputs[0], cmd = v.envInputs[0].Update(msg)
		} else {
			v.envInputs[1], cmd = v.envInputs[1].Update(msg)
		}
	case v.cursor == fieldName:
		v.inputs[0], cmd = v.inputs[0].Update(msg)
	case v.cursor == fieldImage:
		v.inputs[2], cmd = v.inputs[2].Update(msg)
	case v.cursor == fieldBudget:
		v.budgetInput, cmd = v.budgetInput.Update(msg)
	case v.cursor == fieldDomains:
		v.domains, cmd = v.domains.Update(msg)
	}
	return v, cmd
}

func (v *ContainerFormView) submit() tea.Cmd {
	name := v.inputs[0].Value()
	if name == "" {
		v.err = fmt.Errorf("container name is required")
		return nil
	}
	path := v.inputs[1].Value()
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
	var validMounts []engine.Mount
	for _, m := range v.mounts {
		if strings.TrimSpace(m.HostPath) == "" || strings.TrimSpace(m.ContainerPath) == "" {
			continue
		}
		validMounts = append(validMounts, m)
	}

	req := engine.CreateContainerRequest{
		Name:            name,
		ProjectPath:     path,
		Image:           v.inputs[2].Value(),
		AgentType:       agentTypes[v.agentType],
		NetworkMode:     engine.NetworkMode(networkModes[v.network]),
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
		v.mounts = append(v.mounts, engine.Mount{
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
			v.mounts = append([]engine.Mount{{
				HostPath:      dm.HostPath,
				ContainerPath: dm.ContainerPath,
				ReadOnly:      dm.ReadOnly,
			}}, v.mounts...)
			return
		}
	}
}

