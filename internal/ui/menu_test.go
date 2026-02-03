package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMenuModel(t *testing.T) {
	m := NewMenuModel()

	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	model, _ := m.Update(msg)
	m = model.(MenuModel)
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after 'j', got %d", m.cursor)
	}

	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}
	model, _ = m.Update(msg)
	m = model.(MenuModel)
	if m.cursor != 0 {
		t.Errorf("expected cursor 0 after 'k', got %d", m.cursor)
	}

	msg = tea.KeyMsg{Type: tea.KeyEnter}
	model, cmd := m.Update(msg)
	m = model.(MenuModel)
	if m.Selected() != "init" {
		t.Errorf("expected selection 'init', got %s", m.Selected())
	}
	if cmd == nil {
		t.Error("expected quit command after enter")
	}

	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	model, cmd = m.Update(msg)
	m = model.(MenuModel)
	if !m.quitting {
		t.Error("expected quitting true after 'q'")
	}
}

func TestMenuChoices(t *testing.T) {
	m := NewMenuModel()
	found := false
	for _, choice := range m.choices {
		if choice == "work" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'work' to be in menu choices")
	}
}
