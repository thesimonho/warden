package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/thesimonho/warden/access"
	"github.com/thesimonho/warden/api"
)

// accessFormField identifies a field in the access item form.
type accessFormField int

const (
	accessFieldLabel accessFormField = iota
	accessFieldDescription
	accessFieldCredentials // section header
	accessFieldSubmit
)

// AccessFormView handles creating or editing an access item.
// It provides inline editing for the label, description, and
// a simplified credential editor.
type AccessFormView struct {
	client Client
	editID string // non-empty when editing

	err error

	// Text fields.
	labelInput textinput.Model
	descInput  textinput.Model

	// Credentials — simplified TUI representation.
	credentials []formCredential

	// Navigation state.
	cursor  accessFormField
	editing bool // true when a text field is focused

	// Credential sub-navigation.
	credCursor      int  // index into credentials (-1 = section header/add)
	editingCred     bool // true when editing a credential inline
	credFieldIdx    int  // index into credField* constants
	credIsNew       bool // true when editing a newly added credential
	credLabelInput  textinput.Model
	credSourceInput textinput.Model
	credInjInput    textinput.Model

	keys  FormKeyMap
	width int
}

// formCredential is a simplified credential for TUI editing.
// The TUI supports one source and one injection per credential
// (matching the most common use case). Users needing complex
// multi-source/multi-injection setups should use the web UI.
type formCredential struct {
	label       string
	sourceType  access.SourceType
	sourceValue string
	injType     access.InjectionType
	injKey      string
}

// sourceTypes lists the available source types for cycling.
var sourceTypes = []access.SourceType{
	access.SourceEnvVar,
	access.SourceFilePath,
	access.SourceSocketPath,
	access.SourceCommand,
}

// sourceTypeLabels provides display names for source types.
var sourceTypeLabels = map[access.SourceType]string{
	access.SourceEnvVar:     "env",
	access.SourceFilePath:   "file",
	access.SourceSocketPath: "socket",
	access.SourceCommand:    "command",
}

// injectionTypes lists the available injection types for cycling.
var injectionTypes = []access.InjectionType{
	access.InjectionEnvVar,
	access.InjectionMountFile,
	access.InjectionMountSocket,
}

// injectionTypeLabels provides display names for injection types.
var injectionTypeLabels = map[access.InjectionType]string{
	access.InjectionEnvVar:      "env",
	access.InjectionMountFile:   "mount_file",
	access.InjectionMountSocket: "mount_socket",
}

// NewAccessFormView creates a form for creating or editing an access item.
func NewAccessFormView(client Client, editItem *api.AccessItemResponse) *AccessFormView {
	labelInput := textinput.New()
	labelInput.Prompt = ""
	labelInput.Placeholder = "My Credentials"

	descInput := textinput.New()
	descInput.Prompt = ""
	descInput.Placeholder = "What this access item provides..."

	credLabel := textinput.New()
	credLabel.Prompt = ""
	credLabel.Placeholder = "Credential label"

	credSource := textinput.New()
	credSource.Prompt = ""
	credSource.Placeholder = "VAR_NAME or /path"

	credInj := textinput.New()
	credInj.Prompt = ""
	credInj.Placeholder = "VAR_NAME or /container/path"

	v := &AccessFormView{
		client:          client,
		keys:            DefaultFormKeyMap(),
		labelInput:      labelInput,
		descInput:       descInput,
		credLabelInput:  credLabel,
		credSourceInput: credSource,
		credInjInput:    credInj,
		credCursor:      -1,
	}

	if editItem != nil {
		v.editID = editItem.ID
		v.labelInput.SetValue(editItem.Label)
		v.descInput.SetValue(editItem.Description)
		v.credentials = itemToFormCredentials(editItem.Credentials)
	}

	return v
}

// itemToFormCredentials converts API credentials to the simplified form model.
func itemToFormCredentials(creds []access.Credential) []formCredential {
	var result []formCredential
	for _, c := range creds {
		fc := formCredential{label: c.Label}
		if len(c.Sources) > 0 {
			fc.sourceType = c.Sources[0].Type
			fc.sourceValue = c.Sources[0].Value
		}
		if len(c.Injections) > 0 {
			fc.injType = c.Injections[0].Type
			fc.injKey = c.Injections[0].Key
		}
		result = append(result, fc)
	}
	return result
}

// formCredentialToAPI converts the simplified form credentials back to API format.
func formCredentialToAPI(creds []formCredential) []access.Credential {
	var result []access.Credential
	for _, fc := range creds {
		c := access.Credential{
			Label: fc.label,
			Sources: []access.Source{
				{Type: fc.sourceType, Value: fc.sourceValue},
			},
			Injections: []access.Injection{
				{Type: fc.injType, Key: fc.injKey},
			},
		}
		result = append(result, c)
	}
	return result
}

// Init is a no-op; the form is ready immediately.
func (v *AccessFormView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the access form.
func (v *AccessFormView) Update(msg tea.Msg) (View, tea.Cmd) {
	switch msg := msg.(type) {
	case OperationResultMsg:
		if msg.Err != nil {
			v.err = msg.Err
			return v, nil
		}
		return v, func() tea.Msg { return NavigateBackMsg{} }

	case tea.WindowSizeMsg:
		v.width = msg.Width
		return v, nil

	case tea.KeyPressMsg:
		if v.err != nil {
			v.err = nil
			return v, nil
		}
		return v.handleKey(msg)
	}

	// Forward non-key messages to focused inputs.
	if v.editing || v.editingCred {
		return v.updateActiveInput(msg)
	}
	return v, nil
}

// handleKey processes key presses in the form.
func (v *AccessFormView) handleKey(msg tea.KeyPressMsg) (View, tea.Cmd) {
	if v.editingCred {
		return v.handleCredEditKey(msg)
	}
	if v.editing {
		switch msg.String() {
		case "esc":
			v.blurField()
			v.editing = false
			return v, nil
		}
		return v.updateActiveInput(msg)
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
	case msg.String() == "x":
		return v.removeCredential()
	case msg.String() == "tab":
		return v.cycleCredType()
	}
	return v, nil
}

// moveCursor moves the cursor, handling credential sub-navigation.
func (v *AccessFormView) moveCursor(delta int) {
	if v.cursor == accessFieldCredentials {
		next := v.credCursor + delta
		if next < -1 {
			v.credCursor = -1
			v.cursor = accessFieldDescription
			return
		}
		if next >= len(v.credentials) {
			v.cursor = accessFieldSubmit
			return
		}
		v.credCursor = next
		return
	}

	next := v.cursor + accessFormField(delta)
	if next < 0 {
		return
	}
	if next > accessFieldSubmit {
		return
	}
	v.cursor = next
	if v.cursor == accessFieldCredentials {
		if delta > 0 {
			v.credCursor = -1
		} else {
			v.credCursor = max(len(v.credentials)-1, -1)
		}
	}
}

// activateField starts editing the current field.
func (v *AccessFormView) activateField() (View, tea.Cmd) {
	switch v.cursor {
	case accessFieldLabel:
		v.editing = true
		return v, v.labelInput.Focus()
	case accessFieldDescription:
		v.editing = true
		return v, v.descInput.Focus()
	case accessFieldCredentials:
		return v.activateCredField()
	case accessFieldSubmit:
		return v, v.submit()
	}
	return v, nil
}

// activateCredField handles enter on the credentials section.
func (v *AccessFormView) activateCredField() (View, tea.Cmd) {
	if v.credCursor == -1 {
		// Add new credential.
		v.credentials = append(v.credentials, formCredential{
			sourceType: access.SourceEnvVar,
			injType:    access.InjectionEnvVar,
		})
		v.credCursor = len(v.credentials) - 1
		v.credIsNew = true
		return v.startCredEdit()
	}
	if v.credCursor >= 0 && v.credCursor < len(v.credentials) {
		v.credIsNew = false
		return v.startCredEdit()
	}
	return v, nil
}

// startCredEdit opens inline editing for the selected credential.
func (v *AccessFormView) startCredEdit() (View, tea.Cmd) {
	c := v.credentials[v.credCursor]
	v.credLabelInput.SetValue(c.label)
	v.credSourceInput.SetValue(c.sourceValue)
	v.credInjInput.SetValue(c.injKey)
	v.editingCred = true
	v.credFieldIdx = 0
	return v, v.credLabelInput.Focus()
}

// handleCredEditKey processes keys during credential inline editing.
func (v *AccessFormView) handleCredEditKey(msg tea.KeyPressMsg) (View, tea.Cmd) {
	isOnTypeField := v.credFieldIdx == credFieldSourceType || v.credFieldIdx == credFieldInjType
	switch msg.String() {
	case "enter":
		if isOnTypeField {
			return v.cycleCredType()
		}
		v.saveCredEdit()
		return v, nil
	case " ":
		if isOnTypeField {
			return v.cycleCredType()
		}
	case "esc":
		v.cancelCredEdit()
		return v, nil
	case "tab":
		return v.nextCredField()
	}
	if isOnTypeField {
		return v, nil // ignore text input on type selector fields
	}
	return v.updateActiveInput(msg)
}

// Credential sub-field indices within inline editing.
const (
	credFieldLabel       = 0
	credFieldSourceType  = 1 // cycle-only, skipped by nextCredField
	credFieldSourceValue = 2
	credFieldInjType     = 3 // cycle-only, skipped by nextCredField
	credFieldInjKey      = 4
	credFieldCount       = 5
)

// nextCredField cycles through all credential editing fields including
// type selectors.
func (v *AccessFormView) nextCredField() (View, tea.Cmd) {
	v.blurCredField()
	v.credFieldIdx = (v.credFieldIdx + 1) % credFieldCount
	return v, v.focusCredField()
}

// cycleCredType cycles source/injection type when on a credential.
func (v *AccessFormView) cycleCredType() (View, tea.Cmd) {
	if v.cursor != accessFieldCredentials || v.credCursor < 0 || v.credCursor >= len(v.credentials) {
		return v, nil
	}
	// When not in edit mode, don't cycle.
	if !v.editingCred {
		return v, nil
	}
	switch v.credFieldIdx {
	case credFieldSourceType:
		c := &v.credentials[v.credCursor]
		for i, t := range sourceTypes {
			if t == c.sourceType {
				c.sourceType = sourceTypes[(i+1)%len(sourceTypes)]
				break
			}
		}
	case credFieldInjType:
		c := &v.credentials[v.credCursor]
		for i, t := range injectionTypes {
			if t == c.injType {
				c.injType = injectionTypes[(i+1)%len(injectionTypes)]
				break
			}
		}
	}
	return v, nil
}

// focusCredField focuses the appropriate input for the current field.
func (v *AccessFormView) focusCredField() tea.Cmd {
	switch v.credFieldIdx {
	case credFieldLabel:
		return v.credLabelInput.Focus()
	case credFieldSourceValue:
		return v.credSourceInput.Focus()
	case credFieldInjKey:
		return v.credInjInput.Focus()
	}
	return nil
}

// blurCredField blurs all credential editing inputs.
func (v *AccessFormView) blurCredField() {
	v.credLabelInput.Blur()
	v.credSourceInput.Blur()
	v.credInjInput.Blur()
}

// saveCredEdit saves the credential inline edit.
func (v *AccessFormView) saveCredEdit() {
	if v.credCursor >= 0 && v.credCursor < len(v.credentials) {
		v.credentials[v.credCursor].label = v.credLabelInput.Value()
		v.credentials[v.credCursor].sourceValue = v.credSourceInput.Value()
		v.credentials[v.credCursor].injKey = v.credInjInput.Value()
	}
	v.editingCred = false
	v.credIsNew = false
	v.blurCredField()
}

// cancelCredEdit cancels credential editing and removes new credentials.
func (v *AccessFormView) cancelCredEdit() {
	if v.credIsNew {
		v.credentials = append(v.credentials[:v.credCursor], v.credentials[v.credCursor+1:]...)
		if v.credCursor >= len(v.credentials) {
			v.credCursor = max(len(v.credentials)-1, -1)
		}
	}
	v.editingCred = false
	v.credIsNew = false
	v.blurCredField()
}

// removeCredential removes the selected credential.
func (v *AccessFormView) removeCredential() (View, tea.Cmd) {
	if v.cursor != accessFieldCredentials || v.credCursor < 0 || v.credCursor >= len(v.credentials) {
		return v, nil
	}
	v.credentials = append(v.credentials[:v.credCursor], v.credentials[v.credCursor+1:]...)
	if v.credCursor >= len(v.credentials) {
		v.credCursor = max(len(v.credentials)-1, -1)
	}
	return v, nil
}

// blurField blurs the currently active top-level text input.
func (v *AccessFormView) blurField() {
	switch v.cursor {
	case accessFieldLabel:
		v.labelInput.Blur()
	case accessFieldDescription:
		v.descInput.Blur()
	}
}

// updateActiveInput forwards messages to the active text input.
func (v *AccessFormView) updateActiveInput(msg tea.Msg) (View, tea.Cmd) {
	var cmd tea.Cmd
	if v.editingCred {
		switch v.credFieldIdx {
		case credFieldLabel:
			v.credLabelInput, cmd = v.credLabelInput.Update(msg)
		case credFieldSourceValue:
			v.credSourceInput, cmd = v.credSourceInput.Update(msg)
		case credFieldInjKey:
			v.credInjInput, cmd = v.credInjInput.Update(msg)
		}
		return v, cmd
	}
	switch v.cursor {
	case accessFieldLabel:
		v.labelInput, cmd = v.labelInput.Update(msg)
	case accessFieldDescription:
		v.descInput, cmd = v.descInput.Update(msg)
	}
	return v, cmd
}

// submit validates and saves the access item.
func (v *AccessFormView) submit() tea.Cmd {
	label := strings.TrimSpace(v.labelInput.Value())
	if label == "" {
		v.err = fmt.Errorf("label is required")
		return nil
	}
	description := strings.TrimSpace(v.descInput.Value())
	creds := formCredentialToAPI(v.credentials)

	if v.editID != "" {
		return func() tea.Msg {
			_, err := v.client.UpdateAccessItem(context.Background(), v.editID, api.UpdateAccessItemRequest{
				Label:       &label,
				Description: &description,
				Credentials: &creds,
			})
			return OperationResultMsg{Operation: "update_access", Err: err}
		}
	}
	return func() tea.Msg {
		_, err := v.client.CreateAccessItem(context.Background(), api.CreateAccessItemRequest{
			Label:       label,
			Description: description,
			Credentials: creds,
		})
		return OperationResultMsg{Operation: "create_access", Err: err}
	}
}

// Render renders the access form.
func (v *AccessFormView) Render(width, height int) string {
	title := "Create Access Item"
	if v.editID != "" {
		title = "Edit Access Item"
	}

	var s strings.Builder
	s.WriteString(Styles.Muted.Render("← ") + Styles.Bold.Render(title) + "\n\n")

	if v.err != nil {
		s.WriteString(Styles.Error.Render("Error: "+v.err.Error()) + "\n\n")
	}

	// Label field.
	s.WriteString(cursorPrefix(v.cursor == accessFieldLabel) + formLabel.Render("Label:") + "\n")
	s.WriteString(formValue.Render(textInputView(v.labelInput, v.editing && v.cursor == accessFieldLabel)) + "\n\n")

	// Description field.
	s.WriteString(cursorPrefix(v.cursor == accessFieldDescription) + formLabel.Render("Description:") + "\n")
	s.WriteString(formValue.Render(textInputView(v.descInput, v.editing && v.cursor == accessFieldDescription)) + "\n\n")

	// Credentials section.
	isOnCredHeader := v.cursor == accessFieldCredentials && v.credCursor == -1
	headerLine := cursorPrefix(isOnCredHeader) + formLabel.Render("Credentials:")
	if isOnCredHeader {
		headerLine += formCursor.Render(" [Add Credential]")
	} else {
		headerLine += Styles.Muted.Render(" (enter: add)")
	}
	s.WriteString(headerLine + "\n")

	if len(v.credentials) == 0 {
		s.WriteString(formValue.Render(Styles.Muted.Render("None configured.")) + "\n")
	} else {
		for i, c := range v.credentials {
			isSelected := v.cursor == accessFieldCredentials && v.credCursor == i
			s.WriteString(v.renderCredential(c, isSelected, i) + "\n")
		}
	}
	s.WriteString("\n")

	// Submit button.
	submitLabel := "Save"
	if v.editID == "" {
		submitLabel = "Create"
	}
	if v.cursor == accessFieldSubmit {
		s.WriteString(cursorPrefix(true) + formCursor.Render("["+submitLabel+"]"))
	} else {
		s.WriteString(cursorPrefix(false) + Styles.Muted.Render("["+submitLabel+"]"))
	}
	s.WriteString("\n")

	return s.String()
}

// renderCredential renders a single credential entry.
func (v *AccessFormView) renderCredential(c formCredential, isSelected bool, idx int) string {
	prefix := subItemPrefix(isSelected)

	if v.editingCred && isSelected {
		return v.renderCredentialEditing(prefix)
	}

	label := orEmpty(c.label)
	srcType := sourceTypeLabels[c.sourceType]
	srcVal := orEmpty(c.sourceValue)
	injType := injectionTypeLabels[c.injType]
	injKey := orEmpty(c.injKey)

	return fmt.Sprintf(
		"%s%s  %s:%s → %s:%s",
		prefix, label,
		Styles.Muted.Render(srcType), srcVal,
		Styles.Muted.Render(injType), injKey,
	)
}

// renderCredentialEditing renders the inline editing view for a credential.
func (v *AccessFormView) renderCredentialEditing(prefix string) string {
	c := v.credentials[v.credCursor]

	labelView := v.credLabelInput.View()
	if v.credFieldIdx != credFieldLabel {
		labelView = orEmpty(v.credLabelInput.Value())
	}

	srcType := sourceTypeLabels[c.sourceType]
	srcTypeView := Styles.Muted.Render(srcType)
	if v.credFieldIdx == credFieldSourceType {
		srcTypeView = formCursor.Render("[" + srcType + "]")
	}
	srcView := v.credSourceInput.View()
	if v.credFieldIdx != credFieldSourceValue {
		srcView = orEmpty(v.credSourceInput.Value())
	}

	injType := injectionTypeLabels[c.injType]
	injTypeView := Styles.Muted.Render(injType)
	if v.credFieldIdx == credFieldInjType {
		injTypeView = formCursor.Render("[" + injType + "]")
	}
	injView := v.credInjInput.View()
	if v.credFieldIdx != credFieldInjKey {
		injView = orEmpty(v.credInjInput.Value())
	}

	return fmt.Sprintf(
		"%s%s  %s:%s → %s:%s",
		prefix, labelView,
		srcTypeView, srcView,
		injTypeView, injView,
	)
}

// HelpKeyMap returns the form's key bindings for the help bar.
func (v *AccessFormView) HelpKeyMap() help.KeyMap {
	if v.editingCred {
		return credEditHelpKeyMap
	}
	if v.editing {
		return editingHelpKeyMap
	}
	if v.cursor == accessFieldCredentials && v.credCursor >= 0 {
		return formWithRemoveKeyMap{keys: v.keys}
	}
	return v.keys
}
