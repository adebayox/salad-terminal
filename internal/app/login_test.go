package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestLoginCRemainsEmailInput(t *testing.T) {
	m := model{loginFocus: 0}
	nextModel, _ := m.updateLogin(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	next := nextModel.(model)
	if next.loginEmail != "c" {
		t.Fatalf("c must remain usable in the email field, got %q", next.loginEmail)
	}
	if next.loginFocus != 0 {
		t.Fatalf("typing c must not change focus, got %d", next.loginFocus)
	}
}

func TestLoginAccountCreationIsVisibleAndFocusable(t *testing.T) {
	m := model{loginFocus: 0}
	for i := 0; i < 3; i++ {
		nextModel, _ := m.updateLogin(tea.KeyMsg{Type: tea.KeyTab})
		m = nextModel.(model)
	}
	if m.loginFocus != 3 {
		t.Fatalf("account creation must be the fourth focusable option, got %d", m.loginFocus)
	}
	view := m.viewLogin()
	if !strings.Contains(view, "Create a Salad account in your browser") {
		t.Fatal("account creation action is missing from the sign-in screen")
	}
	if strings.Contains(view, "Press c") {
		t.Fatal("sign-in screen must not advertise a conflicting c shortcut")
	}
}
