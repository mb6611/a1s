package ui

import (
	"github.com/derailed/tview"
)

// PromptModel represents the prompt data model
type PromptModel interface {
	SetText(string, string)
	GetText() string
	ClearText(bool)
	IsActive() bool
	SetActive(bool)
	GetSuggestion() string
	Suggest(string)
}

// simplePromptModel is a basic PromptModel implementation.
type simplePromptModel struct {
	text   string
	active bool
}

func (m *simplePromptModel) SetText(_, text string) { m.text = text }
func (m *simplePromptModel) GetText() string        { return m.text }
func (m *simplePromptModel) ClearText(bool)         { m.text = "" }
func (m *simplePromptModel) IsActive() bool         { return m.active }
func (m *simplePromptModel) SetActive(b bool)       { m.active = b }
func (m *simplePromptModel) GetSuggestion() string  { return "" }
func (m *simplePromptModel) Suggest(string)         {}

// Prompt represents a command input field
type Prompt struct {
	*tview.InputField
	model        PromptModel
	icon         rune
	activeFn     func(bool)
	changeFn     func(string)
	bufferFn     func(string, bool)
	suggestionFn func(string)
}

// NewPrompt returns a new prompt
func NewPrompt(icon rune, model PromptModel) *Prompt {
	if model == nil {
		model = &simplePromptModel{}
	}
	p := &Prompt{
		InputField: tview.NewInputField(),
		model:      model,
		icon:       icon,
	}
	p.SetLabel(string(icon) + " ")

	return p
}

// SetModel sets the prompt model
func (p *Prompt) SetModel(m PromptModel) {
	p.model = m
}

// SendKey handles key events (placeholder - not functional without event type compatibility)
func (p *Prompt) SendKey(evt interface{}) {
	// Note: event handling requires type compatibility between tcell versions
	// This is a placeholder for future implementation
}

// InCmdMode checks if prompt is in command mode
func (p *Prompt) InCmdMode() bool {
	return p.model != nil && p.model.IsActive()
}

// Activate activates the prompt
func (p *Prompt) Activate() {
	if p.model == nil {
		return
	}
	p.model.SetActive(true)
	p.SetText("")
	if p.activeFn != nil {
		p.activeFn(true)
	}
}

// Deactivate deactivates the prompt
func (p *Prompt) Deactivate() {
	if p.model == nil {
		return
	}
	p.model.SetActive(false)
	p.SetText("")
	if p.activeFn != nil {
		p.activeFn(false)
	}
}

// SetBufferChangedFn sets buffer change callback
func (p *Prompt) SetBufferChangedFn(fn func(string, bool)) {
	p.bufferFn = fn
}

// SetSuggestionFn sets suggestion callback
func (p *Prompt) SetSuggestionFn(fn func(string)) {
	p.suggestionFn = fn
}

// SetChangeFn sets change callback
func (p *Prompt) SetChangeFn(fn func(string)) {
	p.changeFn = fn
	p.SetChangedFunc(fn)
}

// SetActiveFn sets activation callback
func (p *Prompt) SetActiveFn(fn func(bool)) {
	p.activeFn = fn
}
